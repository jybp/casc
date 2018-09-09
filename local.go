package casc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/jybp/casc/blte"
	"github.com/jybp/casc/common"
	"github.com/pkg/errors"
)

var productToApps = map[string]string{
	"Diablo3": common.Diablo3,
	// "Hero":common.HeroesOfTheStorm,
	// "Prometheus":common.Overwatch,
	"StarCraft1": common.Starcraft1,
	// "SC2": common.Starcraft2,
	"War3": common.Warcraft3,
	// "WoW":common.WorldOfWarcraft,
}

type local struct {
	app             string
	versionName     string
	rootEncodedHash []byte
	installDir      string
	encoding        map[string][][]byte
	idxs            map[uint8][]common.IdxEntry
}

func newLocalStorage(installDir string) (l *local, err error) {

	//
	// app & versionName
	//

	buildInfoB, err := ioutil.ReadFile(filepath.Join(installDir, ".build.info"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	versions, err := common.ParseBuildInfo(bytes.NewReader(buildInfoB))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(versions) != 1 {
		return nil, errors.WithStack(errors.New("several regions within .build.info"))
	}
	var region string
	for key := range versions {
		region = key
	}
	version := versions[region]

	//
	// rootEncodedHash & app
	//

	buildConfigHash := hex.EncodeToString(version.BuildConfigHash)
	buildConfigB, err := ioutil.ReadFile(filepath.Join(installDir,
		"Data",
		common.PathTypeConfig,
		buildConfigHash[0:2],
		buildConfigHash[2:4],
		buildConfigHash))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	buildCfg, err := common.ParseBuildConfig(bytes.NewReader(buildConfigB))
	if err != nil {
		return nil, err
	}
	rootHash := buildCfg.RootHash
	app, ok := productToApps[buildCfg.BuildProduct]
	if !ok {
		return nil, errors.WithStack(errors.Errorf("unknown build-product: %s", buildCfg.BuildProduct))
	}

	//
	// encoding & dataFromEncodedHashFn
	//

	// Load all indices
	files, err := ioutil.ReadDir(filepath.Join(installDir, "Data", "data"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// There is multiple files for the same bucket with duplicate entries.
	// It looks like the last file contains the most up to date indices.
	// Sort the files accordingly so that the first index findIdx finds is the correct.
	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
	idxEntries := map[uint8][]common.IdxEntry{}
	for _, file := range files {
		name := file.Name()
		if len(name) < 4 {
			continue
		}
		if name[len(name)-4:] != ".idx" {
			continue
		}
		f, err := os.Open(filepath.Join(installDir, "Data", "data", name))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				err = cerr
			}
		}()
		bucketID, err := strconv.ParseUint(string(name[1]), 16, 8)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		fmt.Fprintf(common.Wlog, "bucket %x: %s\n", uint8(bucketID), name)
		indices, err := common.ParseIdx(f)
		if err != nil {
			return nil, err
		}
		idxEntries[uint8(bucketID)] = append(idxEntries[uint8(bucketID)], indices...)
	}

	if len(buildCfg.EncodingHashes) < 2 {
		return nil, errors.WithStack(errors.New("expected at least two encoding hash"))
	}
	encodingR, err := dataFromEncodedHash(buildCfg.EncodingHashes[1], installDir, idxEntries)
	if err != nil {
		return nil, err
	}
	encoding, err := common.ParseEncoding(bytes.NewReader(encodingR))
	if err != nil {
		return nil, err
	}

	return &local{
		app:             app,
		versionName:     version.Name,
		rootEncodedHash: rootHash,
		encoding:        encoding,
		installDir:      installDir,
		idxs:            idxEntries,
	}, nil
}

func (s *local) App() string {
	return s.app
}

func (s *local) Version() string {
	return s.versionName
}

func (s *local) RootHash() []byte {
	return s.rootEncodedHash
}

func (s *local) FromContentHash(hash []byte) ([]byte, error) {
	encodedHashes, ok := s.encoding[hex.EncodeToString(hash)]
	if !ok || len(encodedHashes) == 0 {
		return nil, ErrNotFound
	}
	return dataFromEncodedHash(encodedHashes[0], s.installDir, s.idxs)
}

func bucketID(hash []byte) (uint8, error) {
	if len(hash) < 9 {
		return 0, errors.WithStack(errors.New("invalid hash len"))
	}
	i := hash[0] ^ hash[1] ^ hash[2] ^ hash[3] ^ hash[4] ^ hash[5] ^ hash[6] ^ hash[7] ^ hash[8]
	return (i & 0xf) ^ (i >> 4), nil
}

func findIdx(hash []byte, idxs []common.IdxEntry) (common.IdxEntry, error) {
	foundIdx := common.IdxEntry{}
	for _, idx := range idxs {
		keyLen := len(idx.Key)
		hashLen := len(hash)
		shift := hashLen - keyLen
		if shift < 0 {
			return common.IdxEntry{}, errors.WithStack(errors.New("invalid key/hash len"))
		}
		h := hash[:len(hash)-shift]
		if bytes.Compare(h, idx.Key) == 0 {
			foundIdx = idx
			break
		}
	}
	if foundIdx.Key == nil {
		return common.IdxEntry{}, ErrNotFound
	}
	return foundIdx, nil
}

func dataFromEncodedHash(hash []byte, installDir string, idxs map[uint8][]common.IdxEntry) (b []byte, err error) {
	bucketID, err := bucketID(hash)
	if err != nil {
		return nil, err
	}
	indices, ok := idxs[bucketID]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("bucket %x not found", bucketID))
	}
	idx, err := findIdx(hash, indices)
	if err != nil {
		return nil, err
	}
	dataFilename := filepath.Join(installDir, "Data", "data", "data."+fmt.Sprintf("%03d", idx.Index))
	f, err := os.Open(dataFilename)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			err = cerr
		}
	}()
	if _, err := f.Seek(int64(idx.Offset), io.SeekStart); err != nil {
		return nil, errors.WithStack(err)
	}
	// first 9 bytes of reversed blteHash must match hash
	blteHash := make([]byte, 16)
	if err := binary.Read(f, binary.LittleEndian, &blteHash); err != nil {
		return nil, errors.WithStack(err)
	}
	for i := len(blteHash)/2 - 1; i >= 0; i-- { //reverse
		opp := len(blteHash) - 1 - i
		blteHash[i], blteHash[opp] = blteHash[opp], blteHash[i]
	}
	if len(hash) < 9 || bytes.Compare(blteHash[:9], hash[:9]) != 0 {
		return nil, errors.WithStack(errors.New("corrupted local file"))
	}
	var size uint32
	if err := binary.Read(f, binary.LittleEndian, &size); err != nil {
		return nil, errors.WithStack(err)
	}
	if size != idx.Size {
		return nil, errors.WithStack(errors.New("inconsistent size"))
	}
	if _, err := f.Seek(10, io.SeekCurrent); err != nil { //unk, ChecksumA, ChecksumB
		return nil, errors.WithStack(err)
	}
	blteReader, err := blte.NewReader(io.LimitReader(f, int64(idx.Size-30)))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	b, err = ioutil.ReadAll(blteReader)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return b, nil
}

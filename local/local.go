package local

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

type local struct {
	app             string
	versionName     string
	rootEncodedHash []byte
	installDir      string
	encoding        map[string][][]byte
	idxs            map[uint8][]common.IdxEntry
}

func NewStorage(installDir string) (*local, error) {

	//
	// Set app
	//

	findAppFn := func() (string, error) {
		binaryToApp := map[string]string{
			"Diablo III":   common.Diablo3,
			"Warcraft III": common.Warcraft3,
		}
		for binary, app := range binaryToApp {
			if _, err := os.Stat(filepath.Join(installDir, binary+".exe")); err == nil {
				return app, nil
			}
			if _, err := os.Stat(filepath.Join(installDir, binary+".app")); err == nil {
				return app, nil
			}
		}
		return "", errors.WithStack(errors.New("unsupported app"))
	}
	app, err := findAppFn()
	if err != nil {
		return nil, err
	}

	//
	// Set versionName
	//

	buildInfoB, err := ioutil.ReadFile(filepath.Join(installDir, ".build.info"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	versions, err := common.ParseVersions(bytes.NewReader(buildInfoB))
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
	// Set RootEncodedHash
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

	//
	// Set encoding and dataFromEncodedHashFn
	//

	// Load all indices
	files, err := ioutil.ReadDir(filepath.Join(installDir, "Data", "data"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// There is multiple files for the same bucket with duplicate entries.
	// It looks like the last file contains the most up to date indices.
	// Sort the files accordingly so that the first index findIdxFn finds is the correct.
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
		bucketID, err := strconv.ParseUint(string(name[1]), 16, 8)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		fmt.Fprintf(common.Wlog, "bucket %x: %s\n", uint8(bucketID), name)
		indices, err := common.ParseIdx(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		idxEntries[uint8(bucketID)] = append(idxEntries[uint8(bucketID)], indices...)
	}

	if len(buildCfg.EncodingHash) < 2 { // TODO handle cases where only 1 encoding hash is provided
		return nil, errors.WithStack(errors.New("expected at least two encoding hash"))
	}
	encodingR, err := dataFromEncodedHash(buildCfg.EncodingHash[1], installDir, idxEntries)
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
		rootEncodedHash: buildCfg.RootHash,
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

func (s *local) DataFromContentHash(hash []byte) ([]byte, error) {
	encodedHashes, ok := s.encoding[hex.EncodeToString(hash)]
	if !ok || len(encodedHashes) == 0 {
		return nil, errors.WithStack(errors.Errorf("encoded hash not found for decoded hash %x", hash))
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
		return common.IdxEntry{}, errors.WithStack(errors.New("key not found in idx"))
	}
	return foundIdx, nil
}

func dataFromEncodedHash(hash []byte, installDir string, idxs map[uint8][]common.IdxEntry) ([]byte, error) {
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
	defer f.Close()
	if _, err := f.Seek(int64(idx.Offset), io.SeekStart); err != nil {
		return nil, errors.WithStack(err)
	}
	blteHash := make([]byte, 16)
	if err := binary.Read(f, binary.LittleEndian, &blteHash); err != nil {
		return nil, errors.WithStack(err)
	}
	for i := len(blteHash)/2 - 1; i >= 0; i-- { //reverse blteHash
		opp := len(blteHash) - 1 - i
		blteHash[i], blteHash[opp] = blteHash[opp], blteHash[i]
	} //TODO check blteHash against hash
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
	blteEncoded := make([]byte, idx.Size-30)
	if _, err := io.ReadFull(f, blteEncoded); err != nil {
		return nil, errors.WithStack(err)
	}
	blteReader, err := blte.NewReader(bytes.NewReader(blteEncoded))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return ioutil.ReadAll(blteReader)
}

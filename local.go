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

type local struct {
	app             string
	versionName     string
	rootEncodedHash []byte
	buildCfg        common.BuildConfig
	dataDir         string
	encoding        map[string][][]byte
	idxs            map[string]common.IdxEntry // key: first 9 bytes of the encoded hash
}

func newLocalStorage(installDir string) (l *local, err error) {

	//
	// app & versionName
	//

	var dirToApp = map[string]string{
		"Diablo III":            Diablo3,
		"Diablo II Resurrected": Diablo2R,
		"StarCraft":             Starcraft1,
		"Warcraft III":          Warcraft3,
	}
	app, ok := dirToApp[filepath.Base(installDir)]
	if !ok {
		return nil, errors.WithStack(errors.New("unsupported app"))
	}
	var cascDir string
	switch app {
	case Diablo3, Diablo2R, Starcraft1, Warcraft3:
		cascDir = filepath.Join(installDir, "Data")
	default:
		return nil, errors.WithStack(errors.New("unsupported app"))
	}

	buildInfoB, err := ioutil.ReadFile(filepath.Join(installDir, ".build.info"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	versions, err := common.ParseLocalBuildInfo(bytes.NewReader(buildInfoB))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(versions) == 0 {
		return nil, errors.WithStack(errors.New("no entries within .build.info"))
	}

	var version common.Version = versions[0]
	for _, v := range versions {
		if len(v.ProductCode) > 0 && v.ProductCode == app {
			version = v
			break
		}
	}

	fmt.Fprintf(common.Wlog, "app %s, version %s and region %s\n", app, version.Name, version.Region)

	//
	// rootEncodedHash & app
	//

	configDir := filepath.Join(cascDir, common.PathTypeConfig)
	dataDir := filepath.Join(cascDir, common.PathTypeData)

	buildConfigHash := hex.EncodeToString(version.BuildConfigHash)
	buildConfigB, err := ioutil.ReadFile(filepath.Join(
		configDir,
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
	var productToApps = map[string]string{
		"Diablo3":    Diablo3,
		"Fenway":     Diablo2R,
		"StarCraft1": Starcraft1,
		"War3":       Warcraft3,
	}
	buildApp, ok := productToApps[buildCfg.BuildProduct]
	if !ok {
		return nil, errors.WithStack(errors.Errorf("unknown build-product: %s", buildCfg.BuildProduct))
	}
	if app != buildApp {
		return nil, errors.WithStack(errors.Errorf("inconsistent app %s != %s", app, buildApp))
	}

	//
	// encoding & dataFromEncodedHashFn
	//

	// Load all indices
	files, err := ioutil.ReadDir(dataDir)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// There is multiple files for the same bucket with duplicate entries.
	// It looks like the last file contains the most up to date indices.
	// Sort the files accordingly so that the first index findIdx finds is the correct.
	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
	idxEntries := map[string]common.IdxEntry{}
	for _, file := range files {
		name := file.Name()
		if len(name) < 4 {
			continue
		}
		if name[len(name)-4:] != ".idx" {
			continue
		}
		idxB, err := ioutil.ReadFile(filepath.Join(dataDir, name))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		bucketID, err := strconv.ParseUint(string(name[1]), 16, 8)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		fmt.Fprintf(common.Wlog, "bucket %x: %s\n", uint8(bucketID), name)
		indices, err := common.ParseIdx(bytes.NewReader(idxB))
		if err != nil {
			return nil, err
		}
		for _, idx := range indices {
			k := string(idx.Key)
			if _, ok := idxEntries[k]; !ok { // first found is the most up to date
				idxEntries[k] = idx
			}
		}
	}

	if len(buildCfg.EncodingHashes) < 2 {
		return nil, errors.WithStack(errors.New("expected at least two encoding hash"))
	}
	encodingR, err := dataFromEncodedHash(buildCfg.EncodingHashes[1], dataDir, idxEntries)
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
		buildCfg:        buildCfg,
		encoding:        encoding,
		dataDir:         dataDir,
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

func (s *local) BuildConfig() common.BuildConfig {
	return s.buildCfg
}

func (s *local) FromContentHash(hash []byte) ([]byte, error) {
	encodedHashes, ok := s.encoding[hex.EncodeToString(hash)]
	if !ok || len(encodedHashes) == 0 {
		return nil, ErrNotFound
	}
	return dataFromEncodedHash(encodedHashes[0], s.dataDir, s.idxs)
}

// FromEncodedHash accepts full or truncated (at least 9 bytes) encoded hashes.
func (s *local) FromEncodedHash(hash []byte) ([]byte, error) {
	return dataFromEncodedHash(hash, s.dataDir, s.idxs)
}

func findIdx(hash []byte, idxs map[string]common.IdxEntry) (common.IdxEntry, error) {
	if len(hash) < 9 {
		return common.IdxEntry{}, errors.WithStack(errors.New("invalid hash len"))
	}
	idx, ok := idxs[string(hash[:9])]
	if !ok {
		return common.IdxEntry{}, ErrNotFound
	}
	return idx, nil
}

func dataFromEncodedHash(hash []byte, dataDir string, idxs map[string]common.IdxEntry) (b []byte, err error) {
	idx, err := findIdx(hash, idxs)
	if err != nil {
		return nil, err
	}
	dataFilename := filepath.Join(dataDir, "data."+fmt.Sprintf("%03d", idx.Index))
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
	encoded := make([]byte, idx.Size-30)
	if _, err := io.ReadFull(f, encoded); err != nil {
		return nil, errors.WithStack(err)
	}
	blteReader, err := blte.NewReader(bytes.NewReader(encoded))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	b, err = ioutil.ReadAll(blteReader)
	if err != nil {
		return nil, err
	}
	return b, nil
}

package casc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/jybp/casc/common"
	"github.com/pkg/errors"
)

type LocalStorage struct {
	app        string
	region     string
	installDir string
}

func binaryExists(filename string) bool {
	if _, err := os.Stat(filename + ".exe"); err == nil {
		return true
	}
	if _, err := os.Stat(filename + ".app"); err == nil {
		return true
	}
	return false
}

func detectApp(installDir string) (string, error) {
	if binaryExists(path.Join(installDir, "Diablo III")) {
		return Diablo3, nil
	}
	return "", errors.WithStack(errors.New("unsupported app"))
}

func newLocalStorage(installDir string) (*LocalStorage, error) {
	app, err := detectApp(installDir)
	if err != nil {
		return nil, err
	}

	//TODO fetching region is not efficient
	b, err := ioutil.ReadFile(path.Join(installDir, ".build.info"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	versions, err := common.ParseVersions(bytes.NewReader(b))
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
	return &LocalStorage{app, region, installDir}, nil
}

func (l LocalStorage) App() string {
	return l.app
}

func (l LocalStorage) Region() string {
	return l.region
}

func (l LocalStorage) OpenVersions() (io.ReadSeeker, error) {
	b, err := ioutil.ReadFile(path.Join(l.installDir, ".build.info"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return bytes.NewReader(b), nil
}

func (l LocalStorage) OpenConfig(hash []byte) (io.ReadSeeker, error) {
	hashStr := hex.EncodeToString(hash)
	b, err := ioutil.ReadFile(path.Join(l.installDir,
		"Data",
		common.PathTypeConfig,
		hashStr[0:2],
		hashStr[2:4],
		hashStr))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return bytes.NewReader(b), nil
}

func (l LocalStorage) OpenIndex(hash []byte) (io.ReadSeeker, error) {
	hashStr := hex.EncodeToString(hash)
	b, err := ioutil.ReadFile(path.Join(l.installDir,
		"Data",
		"indices",
		hashStr+".index"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return bytes.NewReader(b), nil
}

func (l LocalStorage) OpenData(hash []byte) (io.ReadSeeker, error) {
	//TODO parse all .idx files during newLocalStorage..
	if len(hash) < 9 {
		return nil, errors.WithStack(errors.New("invalid hash len"))
	}
	i := hash[0] ^ hash[1] ^ hash[2] ^ hash[3] ^ hash[4] ^ hash[5] ^ hash[6] ^ hash[7] ^ hash[8]
	bucket := (i & 0xf) ^ (i >> 4)
	idxName := hex.EncodeToString([]byte{bucket})

	files, err := ioutil.ReadDir(path.Join(l.installDir, "Data", "data"))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	entries := []common.IdxEntry{}
	for _, file := range files {
		name := file.Name()
		if len(name) < 6 {
			continue
		}
		if name[:2] != idxName || name[len(name)-4:] != ".idx" {
			continue
		}
		f, err := os.Open(path.Join(l.installDir, "Data", "data", name))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		e, err := common.ParseIdx(f)
		if err := f.Close(); err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, e...)
	}
	foundEntry := common.IdxEntry{}
	for i, entry := range entries {
		keyLen := len(entry.Key)
		hashLen := len(hash)
		shift := hashLen - keyLen
		if shift < 0 {
			return nil, errors.WithStack(errors.New("invalid key/hash len"))
		}
		h := hash[:len(hash)-shift]
		if bytes.Compare(h, entry.Key) == 0 {
			foundEntry = entry
			fmt.Printf("looking for %x\nfound (entry n°%d/%d):%+v\n", hash, i, len(entries), foundEntry)
			continue //TODO duplicated entries, take the last one? Explicitly sort files
		}
	}
	if foundEntry.Key == nil {
		return nil, errors.WithStack(errors.New("key not found in idx"))
	}
	dataFilename := path.Join(l.installDir, "Data", "data", "data."+fmt.Sprintf("%03d", foundEntry.Index))
	fmt.Printf("openning %s\n", dataFilename)
	f, err := os.Open(dataFilename)
	if err != nil {
		fmt.Println(err)
		return nil, errors.WithStack(err)
	}
	defer f.Close()
	if _, err := f.Seek(int64(foundEntry.Offset), 0); err != nil {
		fmt.Println(err)
		return nil, errors.WithStack(err)
	}
	blteHash := make([]byte, 16)
	if err := binary.Read(f, binary.LittleEndian, &blteHash); err != nil {
		fmt.Println(err)
		return nil, errors.WithStack(err)
	}
	for i := len(blteHash)/2 - 1; i >= 0; i-- { //reverse blteHash
		opp := len(blteHash) - 1 - i
		blteHash[i], blteHash[opp] = blteHash[opp], blteHash[i]
	}
	//TODO check blteHash against hash
	var size uint32
	if err := binary.Read(f, binary.LittleEndian, &size); err != nil {
		return nil, errors.WithStack(err)
	}
	if size != foundEntry.Size {
		return nil, errors.WithStack(errors.New("inconsistent size"))
	}
	if _, err := f.Seek(2, 1); err != nil { //unk
		return nil, errors.WithStack(err)
	}
	var checksumA uint32
	if err := binary.Read(f, binary.LittleEndian, &checksumA); err != nil {
		return nil, errors.WithStack(err)
	}
	var checksumB uint32
	if err := binary.Read(f, binary.LittleEndian, &checksumB); err != nil {
		return nil, errors.WithStack(err)
	}
	encodedFile := make([]byte, foundEntry.Size-30)
	if _, err := io.ReadFull(f, encodedFile); err != nil {
		fmt.Println(err)
		return nil, errors.WithStack(err)
	}
	return bytes.NewReader(encodedFile), nil
}

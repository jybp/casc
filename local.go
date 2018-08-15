package casc

import (
	"bytes"
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

	_, err := os.Stat(filename + ".app")
	fmt.Printf("%+v\n", err)
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
	return nil, errors.New("no implemented")
}

func (l LocalStorage) OpenData(hash []byte) (io.ReadSeeker, error) {
	return nil, errors.New("no implemented")
}

func (l LocalStorage) OpenIndex(hash []byte) (io.ReadSeeker, error) {
	return nil, errors.New("no implemented")
}

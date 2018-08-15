package casc

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/pkg/errors"
)

type LocalStorage struct {
	app    string
	region string
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

	return &LocalStorage{app, ""}, nil
}

func (l LocalStorage) App() string {
	return l.app
}

func (l LocalStorage) Region() string {
	return l.region
}

func (l LocalStorage) OpenVersions() (io.ReadSeeker, error) {
	return nil, errors.New("no implemented")
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

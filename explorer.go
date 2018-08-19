package casc

import (
	"net/http"

	"github.com/jybp/casc/root/diablo3"
	"github.com/pkg/errors"
)

// Regions codes
const (
	RegionUS = "us"
	RegionEU = "eu"
	RegionKR = "kr"
	RegionTW = "tw"
	RegionCN = "cn"
)

// Program codes
const (
	Diablo3 = "d3"
	// HeroesOfTheStorm = "hero"
	// Hearthstone      = "hsb"
	// Overwatch        = "pro"
	// Starcraft1       = "s1"
	// Starcraft2       = "s2"
	// Warcraft3        = "w3"
	// WorldOfWarcraft  = "wow"
)

// Storage descibes how to fetch CASC content.
type Storage interface {
	App() string
	Version() string
	RootHash() []byte
	DataFromContentHash(hash []byte) ([]byte, error)
}

// Each app has its own way of relating file names to content hash.
type root interface {
	Files() ([]string, error)
	ContentHash(filename string) ([]byte, error)
}

// Explorer allows to list and extract CASC files.
type Explorer struct {
	storage Storage
	root    root
}

// NewOnlineExplorer will use client to fetch CASC files.
func NewOnlineExplorer(app, region, cdnRegion string, client *http.Client) (*Explorer, error) {
	storage, err := newOnlineStorage(app, region, cdnRegion, client)
	if err != nil {
		return nil, err
	}
	return newExplorer(storage)
}

// NewLocalExplorer will use files located under installDir to fetch CASC files.
func NewLocalExplorer(installDir string) (*Explorer, error) {
	local, err := newLocalStorage(installDir)
	if err != nil {
		return nil, err
	}
	return newExplorer(local)
}

func newExplorer(storage Storage) (*Explorer, error) {
	var root root
	var err error
	switch storage.App() {
	case Diablo3:
		root, err = diablo3.NewRoot(storage.RootHash(), storage.DataFromContentHash)
	default:
		return nil, errors.WithStack(errors.New("unsupported app"))
	}
	return &Explorer{storage, root}, err
}

// App returns the game code
func (e Explorer) App() string {
	return e.storage.App()
}

// Version returns the version of the game on the given region.
func (e Explorer) Version() string {
	return e.storage.Version()
}

// Files enumerates all files.
// The separator within the filenames is '\'.
func (e Explorer) Files() ([]string, error) {
	return e.root.Files()
}

// Extract extracts the file with the given filename.
func (e Explorer) Extract(filename string) ([]byte, error) {
	contentHash, err := e.root.ContentHash(filename)
	if err != nil {
		return nil, err
	}
	return e.storage.DataFromContentHash(contentHash)
}

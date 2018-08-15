package casc

import (
	"io"
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
	Region() string

	OpenVersions() (io.ReadSeeker, error)
	OpenConfig(hash []byte) (io.ReadSeeker, error)
	OpenIndex(hash []byte) (io.ReadSeeker, error)
	OpenData(hash []byte) (io.ReadSeeker, error)
}

// Each app has its own way of relating file names to content hash.
type root interface {
	Files() ([]string, error)
	ContentHash(filename string) ([]byte, error)
}

// Explorer allows to list and extract CASC files.
type Explorer struct {
	extractor *extractor
	root      root
}

// NewOnlineExplorer will use client to fetch CASC files.
func NewOnlineExplorer(app, region, cdnRegion string, client *http.Client) (*Explorer, error) {
	ngdp, err := newNGDP(app, region, cdnRegion, client)
	if err != nil {
		return nil, err
	}
	return newExplorer(ngdp)
}

// NewLocalExplorer will use files located under installDir to fetch CASC files.
func NewLocalExplorer(installDir string) (*Explorer, error) {
	local, err := newLocalStorage(installDir)
	if err != nil {
		return nil, err
	}
	return newExplorer(local)
}

func newExplorer(Storage Storage) (*Explorer, error) {
	extractor, err := newExtractor(Storage)
	if err != nil {
		return nil, err
	}
	var root root
	switch Storage.App() {
	case Diablo3:
		root, err = diablo3.NewRoot(extractor.build.RootHash, extractor.extract)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.WithStack(errors.New("unsupported app"))
	}
	return &Explorer{extractor, root}, nil
}

// Version returns the version of the game on the given region.
func (e Explorer) Version() string {
	return e.extractor.version.Name
}

// Files enumerates all files.
func (e Explorer) Files() ([]string, error) {
	return e.root.Files()
}

// Extract extracts the file with the given filename.
func (e Explorer) Extract(filename string) ([]byte, error) {
	contentHash, err := e.root.ContentHash(filename)
	if err != nil {
		return nil, err
	}
	return e.extractor.extract(contentHash)
}

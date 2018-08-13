package casc

import (
	"encoding/hex"
	"errors"
	"io"

	"github.com/jybp/casc/downloader"
	"github.com/jybp/casc/root/diablo3"
)

const (
	RegionUS = "us"
	RegionEU = "eu"
	RegionKR = "kr"
	RegionTW = "tw"
	RegionCN = "cn"
)

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

// Downloader is the interface that wraps the Get method.
// Get should retrieve the file associated with rawurl.
type Downloader interface {
	Get(rawurl string) (io.ReadSeeker, error)
}

// each app has its own way of relating file names to content hash
type root interface {
	Files() ([]string, error)
	ContentHash(filename string) ([]byte, error)
}

type Storage struct {
	App        string
	Region     string
	Downloader Downloader

	extractor *extractor
	root      root
}

func (s Storage) app() string {
	if s.App == "" {
		return Diablo3
	}
	return s.App
}

func (s Storage) region() string {
	if s.Region == "" {
		return RegionUS
	}
	return s.Region
}

func (s Storage) downloader() Downloader {
	if s.Downloader == nil {
		return &downloader.HTTP{}
	}
	return s.Downloader
}

// Version returns the version of s.App on s.Region.
func (s Storage) Version() (string, error) {
	if err := s.setup(); err != nil {
		return "", err
	}
	return s.extractor.version.Name, nil
}

// Files enumerates all files
func (s Storage) Files() ([]string, error) {
	if err := s.setup(); err != nil {
		return nil, err
	}
	return s.root.Files()
}

// Extract extracts the file with the filename name
func (s Storage) Extract(filename string) ([]byte, error) {
	if err := s.setup(); err != nil {
		return nil, err
	}
	contentHash, err := s.root.ContentHash(filename)
	if err != nil {
		return nil, err
	}
	return s.extractor.extract(contentHash)
}

// initialize s.extractor and s.root
func (s *Storage) setup() error {
	if s.extractor != nil && s.root != nil {
		return nil
	}
	extractor, err := newExtractor(s.downloader(), s.app(), s.region())
	if err != nil {
		return err
	}
	rootHash := make([]byte, hex.DecodedLen(len(extractor.build.RootHash)))
	if _, err := hex.Decode(rootHash, []byte(extractor.build.RootHash)); err != nil {
		return err
	}
	var root root
	switch s.app() {
	case Diablo3:
		root = &diablo3.Root{RootHash: rootHash, Extract: extractor.extract}
	default:
		return errors.New("unsupported app")
	}
	s.extractor = extractor
	s.root = root
	return nil
}

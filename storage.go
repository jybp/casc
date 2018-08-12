package casc

import (
	"io"

	"github.com/jybp/casc/common"
	"github.com/jybp/casc/downloader"
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

	version common.Version
	build   common.BuildConfig

	extractor extractor
	root      root
}

func (s *Storage) app() string {
	if s.App == "" {
		return Diablo3
	}
	return s.App
}

func (s *Storage) region() string {
	if s.Region == "" {
		return RegionUS
	}
	return s.Region
}

func (s *Storage) downloader() Downloader {
	if s.Downloader == nil {
		return &downloader.HTTP{}
	}
	return s.Downloader
}

// Version returns the version of s.App on s.Region.
func (s *Storage) Version() (string, error) {
	if err := s.setupVersion(); err != nil {
		return "", err
	}
	return s.version.Name, nil
}

// Files enumerates all files
func (s *Storage) Files() ([]string, error) {
	if err := s.setupRoot(); err != nil {
		return nil, err
	}
	return s.root.Files()
}

// Extract extracts the file with the filename name
func (s *Storage) Extract(filename string) ([]byte, error) {
	if err := s.setupExtractor(); err != nil {
		return nil, err
	}
	contentHash, err := s.root.ContentHash(filename)
	if err != nil {
		return nil, err
	}
	return s.extractor.extract(contentHash)
}

package casc

import (
	"net/http"

	"github.com/jybp/casc/common"
	"github.com/jybp/casc/local"
	"github.com/jybp/casc/online"
	"github.com/jybp/casc/root/diablo3"
	"github.com/jybp/casc/root/starcraft1"
	"github.com/jybp/casc/root/warcraft3"
	"github.com/pkg/errors"
)

// Storage descibes how to fetch CASC content.
type Storage interface {
	//TODO all methods must be goroutine safe
	App() string
	Version() string
	// Locales() ([]string, error) //todo parse .build.info (/versions) tags to find available locales (one per line)
	RootHash() []byte
	FromContentHash(hash []byte) ([]byte, error)
}

// Each app has its own way of relating file names to content hash.
type root interface {
	//TODO all methods must be goroutine safe
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
	storage, err := online.NewStorage(app, region, cdnRegion, client)
	if err != nil {
		return nil, err
	}
	return newExplorer(storage)
}

// NewLocalExplorer will use files located under installDir to fetch CASC files.
func NewLocalExplorer(installDir string) (*Explorer, error) {
	local, err := local.NewStorage(installDir)
	if err != nil {
		return nil, err
	}
	return newExplorer(local)
}

func newExplorer(storage Storage) (*Explorer, error) {
	rootB, err := storage.FromContentHash(storage.RootHash())
	if err != nil {
		return nil, err
	}
	var root root
	var errRoot error
	switch storage.App() {
	case common.Diablo3:
		root, errRoot = diablo3.NewRoot(rootB, storage.FromContentHash)
	case common.Warcraft3:
		root, errRoot = warcraft3.NewRoot(rootB)
	case common.Starcraft1:
		root, errRoot = starcraft1.NewRoot(rootB)
	default:
		return nil, errors.WithStack(errors.New("unsupported app"))
	}
	return &Explorer{storage, root}, errRoot
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
	return e.storage.FromContentHash(contentHash)
}

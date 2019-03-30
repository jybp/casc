package casc

import (
	"net/http"

	"github.com/jybp/casc/root/diablo3"
	"github.com/jybp/casc/root/starcraft1"
	"github.com/jybp/casc/root/warcraft3"
	"github.com/jybp/casc/root/wow"
	"github.com/pkg/errors"
)

// Program codes
const (
	Diablo3         = "d3"
	Starcraft1      = "s1"
	Warcraft3       = "w3"
	WorldOfWarcraft = "wow"
)

// Regions / CDN Regions
const (
	RegionUS = "us"
	RegionEU = "eu"
	RegionKR = "kr"
	RegionTW = "tw"
	RegionCN = "cn"
)

// ErrNotFound is the error returned by Explorer.Extract if the file was not found within the CASC file system.
// For example, it can occur when extracting a file from a locale not installed.
// This error can be silently ignored by consumers of the casc package.
var ErrNotFound = errors.New("file not found")

// storage descibes how to fetch CASC content.
type storage interface {
	App() string
	Version() string
	RootHash() []byte
	FromContentHash(hash []byte) ([]byte, error)
}

// Each app has its own way of relating file names to content hash.
type root interface {
	Files() ([]string, error)
	ContentHash(filename string) ([]byte, error)
}

// Explorer allows to list and extract CASC files.
type Explorer struct {
	storage storage
	root    root
	//TODO all methods must be goroutine safe
}

// Online will use client to fetch CASC files.
// app is the program code.
// region is the region of the game.
// cdnRegion is the region used to download the files.
// client is used to perform downloads.
func Online(app, region, cdnRegion string, client *http.Client) (*Explorer, error) {
	storage, err := newOnlineStorage(app, region, cdnRegion, client)
	if err != nil {
		return nil, err
	}
	return newExplorer(storage)
}

// Local will use files located under installDir to fetch CASC files.
// Examples:
//  C:\Program Files\Warcraft III
//  /Applications/Warcraft III
func Local(installDir string) (*Explorer, error) {
	local, err := newLocalStorage(installDir)
	if err != nil {
		return nil, err
	}
	return newExplorer(local)
}

func newExplorer(storage storage) (*Explorer, error) {
	rootB, err := storage.FromContentHash(storage.RootHash())
	if err != nil {
		return nil, err
	}
	var root root
	var errRoot error
	switch storage.App() {
	case Diablo3:
		root, errRoot = diablo3.NewRoot(rootB, storage.FromContentHash)
	case Warcraft3:
		root, errRoot = warcraft3.NewRoot(rootB)
	case Starcraft1:
		root, errRoot = starcraft1.NewRoot(rootB)
	case WorldOfWarcraft:
		root, errRoot = wow.NewRoot(rootB)
	default:
		return nil, errors.WithStack(errors.New("unsupported app"))
	}
	return &Explorer{storage, root}, errRoot
}

// App returns the program code.
func (e Explorer) App() string {
	return e.storage.App()
}

// Version returns the version of the game.
func (e Explorer) Version() string {
	return e.storage.Version()
}

// Files enumerates all files.
// The separator within the filenames is '/'.
func (e Explorer) Files() ([]string, error) {
	return e.root.Files()
}

// Extract extracts the file with the given filename.
// Returns casc.NotFound if the file was not found.
func (e Explorer) Extract(filename string) ([]byte, error) {
	contentHash, err := e.root.ContentHash(filename)
	if err != nil {
		return nil, err
	}
	return e.storage.FromContentHash(contentHash)
}

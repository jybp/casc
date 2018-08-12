package downloader

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	fp "path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// FileCache uses the filesystem as a cache and implements Getter.
type FileCache struct {
	HTTP     HTTP
	CacheDir string
}

// filepath creates a file path from rawurl
func (c FileCache) filepath(rawurl string) (string, error) {
	url, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}
	i := strings.LastIndex(url.Path, "/")
	dir := path.Join(c.CacheDir, url.Hostname(), url.Path[:i])
	filepath := dir + url.Path[i:]
	return filepath, nil
}

// download downloads the file at rawurl and stores it at filepath
func (c FileCache) download(rawurl, filepath string) error {
	fmt.Printf("downloading uncached file %s\n", rawurl)
	resp, err := c.HTTP.Get(rawurl)
	if err != nil {
		return errors.WithStack(err)
	}
	buf, err := ioutil.ReadAll(resp)
	if err != nil {
		return errors.WithStack(err)
	}
	dir := fp.Dir(filepath)
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return errors.WithStack(err)
		}
	}
	if err := ioutil.WriteFile(filepath, buf, 0700); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// load loads the cached file
func (c FileCache) load(filepath string) (io.ReadSeeker, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer f.Close()
	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return bytes.NewReader(b), nil
}

// Get tries to retrieve the file associated with rawurl inside the filesystem.
// If unsuccessful, it uses Getter to retrieve the file and writes it to the filesystem.
func (c FileCache) Get(rawurl string) (io.ReadSeeker, error) {
	filepath, err := c.filepath(rawurl)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(filepath)
	fileExists := err == nil
	isVersions := rawurl[len(rawurl)-9:] == "/versions"
	if !fileExists || isVersions { // Always download the versions
		if err := c.download(rawurl, filepath); err != nil {
			return nil, err
		}
	}
	return c.load(filepath)
}

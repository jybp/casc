package d3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
)

// Getter is the interface that wraps the Get method.
// Get should retrieve the file associated with rawurl.
type Getter interface {
	Get(ctx context.Context, rawurl string) (io.ReadSeeker, error)
}

// HTTPGetter is a simple wrapper that makes http.Client implement Getter.
type HTTPGetter struct{ *http.Client }

// Get downloads the file located at rawurl.
func (g *HTTPGetter) Get(ctx context.Context, rawurl string) (io.ReadSeeker, error) {
	resp, err := g.Client.Get(rawurl)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.WithStack(fmt.Errorf("download %s failed (%d)", rawurl, resp.StatusCode))
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return bytes.NewReader(b), nil
}

// FileCache uses the filesystem as a cache and implements Getter.
type FileCache struct {
	Getter   Getter
	CacheDir string
}

// Get tries to retrieve the file associated with rawurl inside the filesystem.
// If unsuccessful, it uses Getter to retrieve the file and writes it to the filesystem.
func (c FileCache) Get(ctx context.Context, rawurl string) (io.ReadSeeker, error) {

	// Always download the versions
	// if rawurl[len(rawurl)-9:] == "/versions" {
	// 	return c.Getter.Get(ctx, rawurl)
	// }

	url, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	loadFileFn := func(filepath string) (io.ReadSeeker, error) {
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

	i := strings.LastIndex(url.Path, "/")
	dir := path.Join(c.CacheDir, url.Hostname(), url.Path[:i])
	filePath := dir + url.Path[i:]
	if _, err := os.Stat(filePath); err == nil {
		return loadFileFn(filePath)
	}

	fmt.Printf("downloading uncached file %s\n", rawurl)
	resp, err := c.Getter.Get(ctx, rawurl)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	buf, err := ioutil.ReadAll(resp)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	if err := ioutil.WriteFile(filePath, buf, 0700); err != nil {
		return nil, errors.WithStack(err)
	}
	return loadFileFn(filePath)
}

package d3

import (
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

type Downloader struct {
	Client *http.Client
}

func (d Downloader) Download(rawurl string) (io.ReadCloser, error) {
	if d.Client == nil {
		d.Client = http.DefaultClient
	}
	resp, err := d.Client.Get(rawurl)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s failed (%d)", rawurl, resp.StatusCode)
	}
	return resp.Body, nil
}

type Cache struct {
	Downloader Downloader
	Output     string
}

func (c Cache) Download(rawurl string) (io.ReadCloser, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	i := strings.LastIndex(u.Path, "/")
	dir := path.Join(c.Output, u.Hostname(), u.Path[:i])
	filePath := dir + u.Path[i:]
	if _, err := os.Stat(filePath); err == nil {
		return os.Open(filePath)
	}

	resp, err := c.Downloader.Download(rawurl)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	buf, err := ioutil.ReadAll(resp)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}

	if err := ioutil.WriteFile(filePath, buf, 0700); err != nil {
		return nil, errors.WithStack(err)
	}
	return os.Open(filePath)
}

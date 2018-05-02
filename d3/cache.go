package d3

import (
	"io"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
)

// Cache will only download files if they can't be found locally under the Output folder
type Cache struct {
	Output string
	Client *http.Client
}

func (c Cache) Download(rawurl string) (io.ReadCloser, error) {

	if c.Client == nil {
		c.Client = http.DefaultClient
	}

	url, err := neturl.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	i := strings.LastIndex(url.Path, "/")
	dir := path.Join(c.Output, url.Hostname(), url.Path[:i])
	filePath := dir + url.Path[i:]

	if _, err := os.Stat(filePath); err == nil {
		return os.Open(filePath)
	}

	resp, err := c.Client.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}

	if err = ioutil.WriteFile(filePath, buf, 0700); err != nil {
		return nil, errors.WithStack(err)
	}

	return os.Open(filePath)
}

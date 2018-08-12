package downloader

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

// HTTPDownloader is a simple wrapper that makes http.Client implement Getter.
type HTTP struct{ Client *http.Client }

// Get downloads the file located at rawurl.
func (d HTTP) Get(rawurl string) (io.ReadSeeker, error) {
	if d.Client == nil {
		d.Client = http.DefaultClient
	}
	resp, err := d.Client.Get(rawurl)
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

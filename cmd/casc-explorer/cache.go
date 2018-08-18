package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"path"

	"os"

	"github.com/pkg/errors"
)

type logTransport struct{}

func (logTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	fmt.Printf("http call (%s) %s\n", r.Method, r.URL)
	return http.DefaultTransport.RoundTrip(r)
}

type cascCache struct {
	cacheDir  string
	transport http.RoundTripper
}

func (c cascCache) RoundTrip(r *http.Request) (*http.Response, error) {
	rawurl := r.URL.String()
	if len(rawurl) >= 9 && rawurl[len(rawurl)-9:] == "/versions" ||
		len(rawurl) >= 5 && rawurl[len(rawurl)-5:] == "/cdns" {
		return c.transport.RoundTrip(r)
	}
	if _, err := os.Stat(c.cacheDir); err != nil {
		if err := os.MkdirAll(c.cacheDir, 0700); err != nil {
			return nil, err
		}
	}
	h := md5.New()
	io.WriteString(h, rawurl)
	filename := path.Join(c.cacheDir, hex.EncodeToString(h.Sum(nil)))
	if _, err := os.Stat(filename); err != nil {
		resp, err := c.transport.RoundTrip(r)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		b, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if err := ioutil.WriteFile(filename, b, 0700); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer f.Close()
	b, err := ioutil.ReadFile(filename) //TODO everything loaded in memory
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return http.ReadResponse(bufio.NewReader(bytes.NewBuffer(b)), r)
}

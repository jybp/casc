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
	"path/filepath"

	"os"

	"github.com/pkg/errors"
)

type logTransport struct{}

func (logTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	fmt.Printf("http call (%s) %s\n", r.Method, r.URL)
	return http.DefaultTransport.RoundTrip(r)
}

var filter = func(r *http.Request) bool {
	rawurl := r.URL.String()
	return (len(rawurl) >= 9 && rawurl[len(rawurl)-9:] == "/versions") ||
		(len(rawurl) >= 5 && rawurl[len(rawurl)-5:] == "/cdns")
}

//TODO move to github.com/jybp/httpcache

const (
	defaultDir                  = "cache"
	defaultFilePerm os.FileMode = 0666
	defaultPathPerm os.FileMode = 0777
)

// Transport implements http.RoundTripper and returns responses from a cache.
type Transport struct {
	Dir       string
	PathPerm  os.FileMode
	FilePerm  os.FileMode
	Transport http.RoundTripper          // used to make requests on cache miss
	Filter    func(r *http.Request) bool // cache won't be used for filtered requests
}

func (t Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.Filter != nil && t.Filter(r) {
		return t.Transport.RoundTrip(r)
	}
	if _, err := os.Stat(t.Dir); err != nil {
		if t.PathPerm == 0 {
			t.PathPerm = defaultPathPerm
		}
		if err := os.MkdirAll(t.Dir, t.PathPerm); err != nil {
			return nil, err
		}
	}
	h := md5.New()
	io.WriteString(h, r.URL.String())
	filename := filepath.Join(t.Dir, hex.EncodeToString(h.Sum(nil)))
	if _, err := os.Stat(filename); err != nil {
		resp, err := t.Transport.RoundTrip(r)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return resp, errors.WithStack(err)
		}
		b, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if t.FilePerm == 0 {
			t.FilePerm = defaultFilePerm
		}
		if err := ioutil.WriteFile(filename, b, t.FilePerm); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return http.ReadResponse(bufio.NewReader(bytes.NewBuffer(b)), r)
}

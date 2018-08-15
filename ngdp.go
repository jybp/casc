package casc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/jybp/casc/common"
	"github.com/pkg/errors"
)

// NGDP fetches the data online using Client.
// NGDP implements the Storage interface.
type NGDP struct {
	app       string
	region    string
	cdnRegion string
	client    *http.Client

	cdn common.Cdn
}

func newNGDP(app, region, cdnRegion string, client *http.Client) (*NGDP, error) {
	cdnR, err := get(client, common.NGDPCdnsURL(app, cdnRegion))
	if err != nil {
		return nil, err
	}
	cdns, err := common.ParseCdn(cdnR)
	if err != nil {
		return nil, err
	}
	cdn, ok := cdns[region]
	if !ok {
		return nil, errors.WithStack(errors.New("cdn region not found"))
	}
	if len(cdn.Hosts) == 0 {
		return nil, errors.WithStack(errors.New("no cdn hosts"))
	}
	return &NGDP{app, region, cdnRegion, client, cdn}, nil
}

func (n NGDP) App() string {
	return n.app
}

func (n NGDP) Region() string {
	return n.region
}

func (n NGDP) OpenVersions() (io.ReadSeeker, error) {
	return get(n.client, common.NGDPVersionsURL(n.app, n.cdnRegion))
}

func (n NGDP) OpenConfig(hash []byte) (io.ReadSeeker, error) {
	return get(n.client, n.url(common.PathTypeConfig, hash, false))
}

//TODO loading all the body to memory is not efficient
func (n NGDP) OpenData(hash []byte) (io.ReadSeeker, error) {
	return get(n.client, n.url(common.PathTypeData, hash, false))
}

func (n NGDP) OpenIndex(hash []byte) (io.ReadSeeker, error) {
	return get(n.client, n.url(common.PathTypeData, hash, true))
}

func (n NGDP) url(pathType string, hash []byte, index bool) string {
	return common.Url(n.cdn.Hosts[0], //TODO we just take the first Host?
		n.cdn.Path,
		pathType,
		hex.EncodeToString(hash),
		index)
}

func get(client *http.Client, rawurl string) (io.ReadSeeker, error) {
	resp, err := client.Get(rawurl)
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

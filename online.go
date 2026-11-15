package casc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"io/ioutil"

	"github.com/jybp/casc/blte"
	"github.com/jybp/casc/common"
	"github.com/pkg/errors"
)

// archiveIndex combines an ArchiveIndexEntry with the archive hash its referring to.
type archiveIndex struct {
	common.ArchiveIndexEntry
	archiveHash []byte
}

type online struct {
	app             string
	versionName     string
	rootEncodedHash []byte
	buildCfg        common.BuildConfig
	encoding        map[string][][]byte
	archivesIndices map[string]archiveIndex // key: first 9 bytes of the encoded hash
	fullEncoded     map[string][]byte       // first 9 bytes of encoded hash => full encoded hash
	client          *http.Client
	cdnHost         string
	cdnPath         string
}

func newOnlineStorage(app, region, cdnRegion string, client *http.Client) (*online, error) {
	downloadFn := func(rawurl string) (b []byte, err error) {
		resp, err := client.Get(rawurl)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer func() {
			if cerr := resp.Body.Close(); cerr != nil {
				err = cerr
			}
		}()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, errors.WithStack(fmt.Errorf("(%d) %s ", resp.StatusCode, rawurl))
		}
		return ioutil.ReadAll(resp.Body)
	}

	//
	// Set versionName
	//

	versionsB, err := downloadFn(common.NGDPVersionsURL(app, cdnRegion))
	if err != nil {
		return nil, err
	}
	versions, err := common.ParseOnlineVersions(bytes.NewReader(versionsB))
	if err != nil {
		return nil, err
	}
	var version common.Version
	for _, v := range versions {
		if v.Region == region {
			version = v
			break
		}
	}
	if len(version.Region) == 0 {
		return nil, errors.WithStack(fmt.Errorf("version with region %s not found", region))
	}

	//
	// Set RootEncodedHash
	//

	cdnsB, err := downloadFn(common.NGDPCdnsURL(app, cdnRegion))
	if err != nil {
		return nil, err
	}
	cdns, err := common.ParseCdn(bytes.NewReader(cdnsB))
	if err != nil {
		return nil, err
	}
	cdn, ok := cdns[region]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("cdn with region %s not found", region))
	}
	if len(cdn.Hosts) == 0 {
		return nil, errors.WithStack(errors.New("no cdn hosts"))
	}
	buildCfgURL, err := common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeConfig, version.BuildConfigHash, false)
	if err != nil {
		return nil, err
	}
	buildCfgB, err := downloadFn(buildCfgURL)
	if err != nil {
		return nil, err
	}

	buildCfg, err := common.ParseBuildConfig(bytes.NewReader(buildCfgB))
	if err != nil {
		return nil, err
	}

	//
	// Set encoding
	//

	if len(buildCfg.EncodingHashes) < 2 {
		return nil, errors.WithStack(errors.New("expected at least two encoding hash"))
	}
	encodingURL, err := common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeData, buildCfg.EncodingHashes[1], false)
	if err != nil {
		return nil, err
	}
	encodingBlteB, err := downloadFn(encodingURL)
	if err != nil {
		return nil, err
	}

	blteReader, err := blte.NewReader(bytes.NewReader(encodingBlteB))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	encodingB, err := ioutil.ReadAll(blteReader)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	encoding, err := common.ParseEncoding(bytes.NewReader(encodingB))
	if err != nil {
		return nil, err
	}

	//
	// Set archivesIndices
	//
	cdnCfgURL, err := common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeConfig, version.CDNConfigHash, false)
	if err != nil {
		return nil, err
	}
	cdnCfgB, err := downloadFn(cdnCfgURL)
	if err != nil {
		return nil, err
	}

	cdnCfg, err := common.ParseCdnConfig(bytes.NewReader(cdnCfgB))
	if err != nil {
		return nil, err
	}
	archivesIndices := map[string]archiveIndex{}
	for _, archiveHash := range cdnCfg.ArchivesHashes {
		archiveIndexURL, err := common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeData, archiveHash, true)
		if err != nil {
			return nil, err
		}
		indicesB, err := downloadFn(archiveIndexURL)
		if err != nil {
			return nil, err
		}
		indices, err := common.ParseArchiveIndex(bytes.NewReader(indicesB))
		if err != nil {
			return nil, err
		}
		for _, index := range indices {
			k := string(index.HeaderHash[:9])
			if _, ok := archivesIndices[k]; !ok {
				archivesIndices[k] = archiveIndex{index, archiveHash}
			}
		}
	}
	fmt.Fprintf(common.Wlog, "%d archive indices parsed\n", len(archivesIndices))
	fullEncoded := map[string][]byte{}
	for _, ekeys := range encoding {
		for _, ekey := range ekeys {
			if len(ekey) >= 9 {
				fullEncoded[string(ekey[:9])] = ekey
			}
		}
	}
	for _, h := range buildCfg.VfsEncodedHashes {
		if len(h) >= 9 {
			fullEncoded[string(h[:9])] = h
		}
	}
	return &online{
		app:             app,
		versionName:     version.Name,
		rootEncodedHash: buildCfg.RootHash,
		buildCfg:        buildCfg,
		encoding:        encoding,
		archivesIndices: archivesIndices,
		fullEncoded:     fullEncoded,
		client:          client,
		cdnHost:         cdn.Hosts[0],
		cdnPath:         cdn.Path,
	}, nil
}

func (s *online) App() string {
	return s.app
}

func (s *online) Version() string {
	return s.versionName
}

func (s *online) RootHash() []byte {
	return s.rootEncodedHash
}

func (s *online) BuildConfig() common.BuildConfig {
	return s.buildCfg
}

// FromEncodedHash accepts full or truncated (at least 9 bytes) encoded hashes.
func (s *online) FromEncodedHash(hash []byte) ([]byte, error) {
	if len(hash) < 9 {
		return nil, errors.WithStack(errors.New("invalid hash len"))
	}
	if full, ok := s.fullEncoded[string(hash[:9])]; ok {
		hash = full
	}
	return s.dataFromEncodedHash(hash)
}

func (s *online) FromContentHash(hash []byte) ([]byte, error) {
	encodedHashes, ok := s.encoding[hex.EncodeToString(hash)]
	if !ok || len(encodedHashes) == 0 {
		return nil, errors.WithStack(errors.Errorf("encoded hash not found for decoded hash %x", hash))
	}
	return s.dataFromEncodedHash(encodedHashes[0])
}

func (s *online) dataFromEncodedHash(hash []byte) ([]byte, error) {
	downloadFn := func(hash []byte, offset, size uint32) (b []byte, err error) {
		url, err := common.Url(s.cdnHost, s.cdnPath, common.PathTypeData, hash, false)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if size > 0 {
			req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+size))
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer func() {
			if cerr := resp.Body.Close(); cerr != nil {
				err = cerr
			}
		}()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, errors.WithStack(fmt.Errorf("(%d) %s ", resp.StatusCode, url))
		}
		return ioutil.ReadAll(resp.Body)
	}
	decodeBlteFn := func(encoded io.Reader) ([]byte, error) {
		blteReader, err := blte.NewReader(encoded)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return ioutil.ReadAll(blteReader)
	}
	if idx, ok := s.archivesIndices[string(hash[:9])]; ok {
		b, err := downloadFn(idx.archiveHash, idx.Offset, idx.EncodedSize)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return decodeBlteFn(bytes.NewReader(b))
	}
	if len(hash) < 16 {
		return nil, ErrNotFound
	}
	b, err := downloadFn(hash, 0, 0)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return decodeBlteFn(bytes.NewReader(b))
}

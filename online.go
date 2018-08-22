package casc

import (
	"bytes"
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
	app string

	versionName     string
	rootEncodedHash []byte
	encoding        common.Encoding
	archivesIndices []archiveIndex
	downloadDataFn  func(hash []byte) ([]byte, error)
}

func newOnlineStorage(app, region, cdnRegion string, client *http.Client) (*online, error) {
	downloadFn := func(rawurl string) ([]byte, error) {
		resp, err := client.Get(rawurl)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, errors.WithStack(fmt.Errorf("(%d) %s ", resp.StatusCode, rawurl))
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return b, nil
	}

	//
	// Set versionName
	//

	versionsB, err := downloadFn(common.NGDPVersionsURL(app, cdnRegion))
	if err != nil {
		return nil, err
	}
	versions, err := common.ParseVersions(bytes.NewReader(versionsB))
	if err != nil {
		return nil, err
	}
	version, ok := versions[region]
	if !ok {
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

	if len(buildCfg.EncodingHash) < 2 {
		return nil, errors.WithStack(errors.New("expected at least two encoding hash"))
	}
	encodingURL, err := common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeData, buildCfg.EncodingHash[1], false)
	if err != nil {
		return nil, err
	}
	encodingBlteB, err := downloadFn(encodingURL)
	if err != nil {
		return nil, err
	}
	encodingBuf := bytes.NewBuffer([]byte{})
	if err := blte.Decode(bytes.NewReader(encodingBlteB), encodingBuf); err != nil {
		return nil, err
	}
	encoding, err := common.ParseEncoding(encodingBuf)
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
	archivesIndices := []archiveIndex{}
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
			archivesIndices = append(archivesIndices, archiveIndex{index, archiveHash})
		}
	}

	//
	// Set downloadDataFn
	//
	downloadDataFn := func(hash []byte) ([]byte, error) {
		url, err := common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeData, hash, false)
		if err != nil {
			return nil, err
		}
		return downloadFn(url)
	}

	return &online{
		app:             app,
		versionName:     version.Name,
		rootEncodedHash: buildCfg.RootHash,
		encoding:        encoding,
		archivesIndices: archivesIndices,
		downloadDataFn:  downloadDataFn,
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

func (s *online) DataFromContentHash(hash []byte) ([]byte, error) {
	encodedHash, err := findEncodedHash(s.encoding, hash)
	if err != nil {
		return nil, err
	}

	decodeBlteFn := func(encoded []byte) ([]byte, error) {
		blteDecoded := bytes.NewBuffer([]byte{})
		if err := blte.Decode(bytes.NewBuffer(encoded), blteDecoded); err != nil {
			return nil, err
		}
		return blteDecoded.Bytes(), nil
	}

	for _, idx := range s.archivesIndices {
		if bytes.Compare(encodedHash, idx.HeaderHash[:]) == 0 {
			archiveB, err := s.downloadDataFn(idx.archiveHash)
			if err != nil {
				return nil, err
			}
			archiveR := bytes.NewReader(archiveB)
			if _, err := archiveR.Seek(int64(idx.Offset), io.SeekStart); err != nil {
				return nil, err
			}
			blteEncoded := make([]byte, idx.EncodedSize)
			if _, err := io.ReadFull(archiveR, blteEncoded); err != nil {
				return nil, err
			}
			return decodeBlteFn(blteEncoded)
		}
	}

	//encoded hash not found in indices, download directly
	blteEncoded, err := s.downloadDataFn(encodedHash)
	if err != nil {
		return nil, err
	}
	return decodeBlteFn(blteEncoded)

}

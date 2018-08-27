package online

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
	encoding        map[string][][]byte
	archivesIndices []archiveIndex
	downloadDataFn  func(hash []byte) ([]byte, error)
}

func NewStorage(app, region, cdnRegion string, client *http.Client) (*online, error) {
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
	versions, err := common.ParseOnlineVersions(bytes.NewReader(versionsB))
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

	if len(buildCfg.EncodingHashes) < 2 { //TODO handle if only content hash provided
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
	fmt.Fprintf(common.Wlog, "%d archive indices parsed\n", len(archivesIndices))
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

func (s *online) FromContentHash(hash []byte) ([]byte, error) {
	encodedHashes, ok := s.encoding[hex.EncodeToString(hash)]
	if !ok || len(encodedHashes) == 0 {
		return nil, errors.WithStack(errors.Errorf("encoded hash not found for decoded hash %x", hash))
	}
	return s.dataFromEncodedHash(encodedHashes[0])
}

func (s *online) dataFromEncodedHash(hash []byte) ([]byte, error) {
	decodeBlteFn := func(encoded io.Reader) ([]byte, error) {
		blteReader, err := blte.NewReader(encoded)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return ioutil.ReadAll(blteReader)
	}
	for _, idx := range s.archivesIndices {
		if bytes.Compare(hash, idx.HeaderHash[:]) == 0 {
			archiveB, err := s.downloadDataFn(idx.archiveHash)
			if err != nil {
				return nil, err
			}
			return decodeBlteFn(io.NewSectionReader(bytes.NewReader(archiveB),
				int64(idx.Offset),
				int64(idx.EncodedSize)))
		}
	}
	blteEncoded, err := s.downloadDataFn(hash)
	if err != nil {
		return nil, err
	}
	return decodeBlteFn(bytes.NewReader(blteEncoded))
}

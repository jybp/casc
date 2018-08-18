package casc

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"encoding/hex"
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
	buildCfgB, err := downloadFn(common.Url(
		cdn.Hosts[0],
		cdn.Path,
		common.PathTypeConfig,
		hex.EncodeToString(version.BuildConfigHash),
		false))
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
	encodingBlteB, err := downloadFn(common.Url(
		cdn.Hosts[0],
		cdn.Path,
		common.PathTypeData,
		hex.EncodeToString(buildCfg.EncodingHash[1]),
		false))
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

	cdnCfgB, err := downloadFn(common.Url(
		cdn.Hosts[0],
		cdn.Path,
		common.PathTypeConfig,
		hex.EncodeToString(version.CDNConfigHash),
		false))
	if err != nil {
		return nil, err
	}
	cdnCfg, err := common.ParseCdnConfig(bytes.NewReader(cdnCfgB))
	if err != nil {
		return nil, err
	}
	archivesIndices := []archiveIndex{}
	for _, archiveHash := range cdnCfg.ArchivesHashes {
		indicesB, err := downloadFn(common.Url(
			cdn.Hosts[0],
			cdn.Path,
			common.PathTypeData,
			hex.EncodeToString(archiveHash),
			true))
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
		return downloadFn(common.Url(
			cdn.Hosts[0],
			cdn.Path,
			common.PathTypeData,
			hex.EncodeToString(hash),
			false))
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
	encodedHash, err := s.encoding.FindEncodedHash(hash)
	if err != nil {
		return nil, err
	}
	var foundIndex archiveIndex
	for _, idx := range s.archivesIndices {
		if bytes.Compare(encodedHash, idx.HeaderHash[:]) == 0 {
			foundIndex = idx
			break
		}
	}
	if foundIndex.archiveHash == nil {
		return nil, errors.WithStack(fmt.Errorf("%x not found in indices", hash))
		//return s.downloadDataFn(encodedHash) //download directly?
	}
	archiveB, err := s.downloadDataFn(foundIndex.archiveHash)
	if err != nil {
		return nil, err
	}
	archiveR := bytes.NewReader(archiveB)
	if _, err := archiveR.Seek(int64(foundIndex.Offset), 0); err != nil {
		return nil, err
	}
	blteEncoded := make([]byte, foundIndex.EncodedSize)
	if _, err := io.ReadFull(archiveR, blteEncoded); err != nil {
		return nil, err
	}
	blteDecoded := bytes.NewBuffer([]byte{})
	if err := blte.Decode(bytes.NewBuffer(blteEncoded), blteDecoded); err != nil {
		return nil, err
	}
	return blteDecoded.Bytes(), nil
}

package casc

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/jybp/casc/blte"
	"github.com/jybp/casc/common"
	"github.com/jybp/casc/internal/d3"
	"github.com/pkg/errors"
)

func (s *Storage) setupVersion() error {
	if s.version != (common.Version{}) {
		return nil
	}
	versionsR, err := s.downloader().Get(common.NGDPVersionsURL(s.app(), s.region()))
	if err != nil {
		return err
	}
	versions, err := common.ParseVersions(versionsR)
	if err != nil {
		return err
	}
	version, ok := versions[s.region()]
	if !ok {
		return fmt.Errorf("version not found for region %s", s.region())
	}
	s.version = version
	return nil
}

func (s *Storage) setupExtractor() error {
	if err := s.setupVersion(); err != nil {
		return err
	}
	if s.build != (common.BuildConfig{}) {
		return nil
	}

	// CDN urls
	cdnR, err := s.downloader().Get(common.NGDPCdnsURL(s.app(), s.region()))
	if err != nil {
		return err
	}

	cdns, err := common.ParseCdn(cdnR)
	if err != nil {
		return err
	}

	cdn, ok := cdns[RegionUS]
	if !ok {
		return errors.New("cdn region not found")
	}

	if len(cdn.Hosts) == 0 {
		return errors.New("no cdn host")
	}

	// Build Config
	buildCfgR, err := s.downloader().Get(common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeConfig, s.version.BuildHash, false))
	if err != nil {
		return err
	}

	buildCfg, err := common.ParseBuildConfig(buildCfgR)
	if err != nil {
		return err
	}

	if len(buildCfg.EncodingHash) != 2 {
		return errors.New("expected 3 build encoding hashes")
	}

	// Encoding File
	encodingR, err := s.downloader().Get(common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeData, buildCfg.EncodingHash[1], false))
	if err != nil {
		return err
	}

	encodingDecodedR := bytes.NewBuffer([]byte{})
	if err := blte.Decode(encodingR, encodingDecodedR); err != nil {
		return err
	}

	encoding, err := common.ParseEncoding(encodingDecodedR)
	if err != nil {
		return err
	}

	// CDN Config
	cdnCfgR, err := s.downloader().Get(common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeConfig, s.version.CDNHash, false))
	if err != nil {
		return err
	}

	cdnCfg, err := common.ParseCdnConfig(cdnCfgR)
	if err != nil {
		return err
	}

	// Load all Archives Index
	// fmt.Printf("loading archive indices (%d)\n", len(cdnCfg.ArchivesHashes))
	// map of archive hash => archive indices
	archivesIdxs := map[string][]common.ArchiveIndexEntry{}
	for _, archiveHash := range cdnCfg.ArchivesHashes {
		idxR, err := s.downloader().Get(common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeData, archiveHash, true))
		if err != nil {
			return err
		}
		idxs, err := common.ParseArchiveIndex(idxR)
		if err != nil {
			return err
		}
		archivesIdxs[archiveHash] = append(archivesIdxs[archiveHash], idxs...)
	}

	s.build = buildCfg
	s.extractor = extractor{
		downloader:   s.downloader(),
		cdn:          cdn,
		encoding:     encoding,
		archivesIdxs: archivesIdxs,
	}
	return nil
}

func (s *Storage) setupRoot() error {
	if err := s.setupExtractor(); err != nil {
		return err
	}
	if s.root != nil {
		return nil
	}

	rootHash := make([]byte, hex.DecodedLen(len(s.build.RootHash)))
	if _, err := hex.Decode(rootHash, []byte(s.build.RootHash)); err != nil {
		return err
	}
	switch s.App {
	case Diablo3:
		s.root = &d3.Root{RootHash: rootHash, Extract: s.extractor.extract}
	default:
		return errors.New("unsupported app")
	}
	return nil
}

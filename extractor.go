package casc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/jybp/casc/blte"
	"github.com/jybp/casc/common"
)

// extractor allows to retrieve a file from a content hash
type extractor struct {
	downloader   Downloader
	version      common.Version
	build        common.BuildConfig
	cdn          common.Cdn
	encoding     common.Encoding
	archivesIdxs map[string][]common.ArchiveIndexEntry
}

// newExtractor is not trivial and makes use of downloader
func newExtractor(downloader Downloader, app, region string) (*extractor, error) {
	versionsR, err := downloader.Get(common.NGDPVersionsURL(app, region))
	if err != nil {
		return nil, err
	}
	versions, err := common.ParseVersions(versionsR)
	if err != nil {
		return nil, err
	}
	version, ok := versions[region]
	if !ok {
		return nil, fmt.Errorf("region %s not found", region)
	}

	// CDN urls
	cdnR, err := downloader.Get(common.NGDPCdnsURL(app, region))
	if err != nil {
		return nil, err
	}

	cdns, err := common.ParseCdn(cdnR)
	if err != nil {
		return nil, err
	}

	cdn, ok := cdns[RegionUS]
	if !ok {
		return nil, errors.New("cdn region not found")
	}

	if len(cdn.Hosts) == 0 {
		return nil, errors.New("no cdn host")
	}

	// Build Config
	buildCfgR, err := downloader.Get(common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeConfig, version.BuildHash, false))
	if err != nil {
		return nil, err
	}

	buildCfg, err := common.ParseBuildConfig(buildCfgR)
	if err != nil {
		return nil, err
	}

	if len(buildCfg.EncodingHash) != 2 {
		return nil, errors.New("expected 3 build encoding hashes")
	}

	fmt.Println("encoding:", buildCfg.EncodingHash[1])

	// Encoding File
	encodingR, err := downloader.Get(common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeData, buildCfg.EncodingHash[1], false))
	if err != nil {
		return nil, err
	}

	encodingDecodedR := bytes.NewBuffer([]byte{})
	if err := blte.Decode(encodingR, encodingDecodedR); err != nil {
		return nil, err
	}

	encoding, err := common.ParseEncoding(encodingDecodedR)
	if err != nil {
		return nil, err
	}

	// CDN Config
	cdnCfgR, err := downloader.Get(common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeConfig, version.CDNHash, false))
	if err != nil {
		return nil, err
	}

	cdnCfg, err := common.ParseCdnConfig(cdnCfgR)
	if err != nil {
		return nil, err
	}

	// Load all Archives Index
	// fmt.Printf("loading archive indices (%d)\n", len(cdnCfg.ArchivesHashes))
	// map of archive hash => archive indices
	archivesIdxs := map[string][]common.ArchiveIndexEntry{}
	for _, archiveHash := range cdnCfg.ArchivesHashes {
		idxR, err := downloader.Get(common.Url(cdn.Hosts[0], cdn.Path, common.PathTypeData, archiveHash, true))
		if err != nil {
			return nil, err
		}
		idxs, err := common.ParseArchiveIndex(idxR)
		if err != nil {
			return nil, err
		}
		archivesIdxs[archiveHash] = append(archivesIdxs[archiveHash], idxs...)
	}

	return &extractor{
		downloader:   downloader,
		version:      version,
		build:        buildCfg,
		cdn:          cdn,
		encoding:     encoding,
		archivesIdxs: archivesIdxs,
	}, nil
}

// extract retrieves a file from a content hash
func (s *extractor) extract(contentHash []byte) ([]byte, error) {
	encodedHash, err := s.encoding.FindEncodedHash(contentHash)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("encoded hash for decoded hash %x is %x\n", contentHash, encodedHash)

	archiveInfo := struct {
		ArchiveHash string
		Index       common.ArchiveIndexEntry
	}{}
	for archiveHash, indices := range s.archivesIdxs {
		for _, idx := range indices {
			if bytes.Compare(encodedHash, idx.HeaderHash[:]) == 0 {
				archiveInfo = struct {
					ArchiveHash string
					Index       common.ArchiveIndexEntry
				}{archiveHash, idx}
				break
			}
		}
	}

	if archiveInfo.ArchiveHash == "" || archiveInfo.Index == (common.ArchiveIndexEntry{}) {
		// encodedHash was not found inside archive indices, try to download the whole file
		r, err := s.downloader.Get(common.Url(s.cdn.Hosts[0], s.cdn.Path, common.PathTypeData, fmt.Sprintf("%x", encodedHash), false))
		if err != nil {
			return nil, err
		}
		//TODO no blte decode?
		return ioutil.ReadAll(r)
	}

	// TODO should only download relevant part of the archive using
	// http header Content-Range bytes start-end/total:
	//  1. check file exist
	//  2. check file size is >= offset+size
	//  3. check the content of file offset-size is not just 0 padded
	//
	//  To store downloaded content-range open the file and write
	//  the content to the correct offset.
	archive, err := s.downloader.Get(common.Url(s.cdn.Hosts[0], s.cdn.Path, common.PathTypeData, archiveInfo.ArchiveHash, false))
	if err != nil {
		return nil, err
	}
	if _, err := archive.Seek(int64(archiveInfo.Index.Offset), 0); err != nil {
		return nil, err
	}
	encodedFile := make([]byte, archiveInfo.Index.EncodedSize)
	if _, err := io.ReadFull(archive, encodedFile); err != nil {
		return nil, err
	}
	decodedFile := bytes.NewBuffer([]byte{})
	if err := blte.Decode(bytes.NewBuffer(encodedFile), decodedFile); err != nil {
		return nil, err
	}
	return decodedFile.Bytes(), nil
}

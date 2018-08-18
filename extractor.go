package casc

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/jybp/casc/blte"
	"github.com/jybp/casc/common"
	"github.com/pkg/errors"
)

// archiveIndex combines an index entry with the archive hash its referring to.
type archiveIndex struct {
	common.ArchiveIndexEntry
	archiveHash []byte
}

// extractor allows to retrieve a file from a content hash.
type extractor struct {
	Storage Storage

	version         common.Version
	build           common.BuildConfig
	cdn             common.Cdn
	encoding        common.Encoding
	archivesIndices []archiveIndex
}

// newExtractor makes use of Storage.
func newExtractor(Storage Storage) (*extractor, error) {
	// Versions
	versionsR, err := Storage.OpenVersions()
	if err != nil {
		return nil, err
	}
	versions, err := common.ParseVersions(versionsR)
	if err != nil {
		return nil, err
	}
	version, ok := versions[Storage.Region()]
	if !ok {
		return nil, fmt.Errorf("region %s not found", Storage.Region())
	}

	// Build Config
	buildCfgR, err := Storage.OpenConfig(version.BuildConfigHash)
	if err != nil {
		return nil, err
	}
	buildCfg, err := common.ParseBuildConfig(buildCfgR)
	if err != nil {
		return nil, err
	}
	if len(buildCfg.EncodingHash) != 2 {
		return nil, errors.New("expected 2 encoding hashes inside the build config")
	}
	fmt.Printf("encoding: %x\n", buildCfg.EncodingHash[1])

	// Encoding File
	// TODO handle cases where only 1 encoding hash is provided
	encodingR, err := Storage.OpenData(buildCfg.EncodingHash[1])
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
	cdnCfgR, err := Storage.OpenConfig(version.CDNConfigHash)
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
	archivesIndices := []archiveIndex{}
	for _, archiveHash := range cdnCfg.ArchivesHashes {
		idxR, err := Storage.OpenIndex(archiveHash)
		if err != nil {
			return nil, err
		}
		idxs, err := common.ParseArchiveIndex(idxR)
		if err != nil {
			return nil, err
		}
		for _, idx := range idxs {
			archivesIndices = append(archivesIndices, archiveIndex{idx, archiveHash})
		}
	}

	return &extractor{
		Storage:         Storage,
		version:         version,
		build:           buildCfg,
		encoding:        encoding,
		archivesIndices: archivesIndices,
	}, nil
}

// extract retrieves a file from a content hash
func (e extractor) extract(contentHash []byte) ([]byte, error) {
	encodedHash, err := e.encoding.FindEncodedHash(contentHash)
	if err != nil {
		return nil, err
	}
	// fmt.Printf("encoded hash for decoded hash %x is %x\n", contentHash, encodedHash)
	var foundIndex archiveIndex
	for _, idx := range e.archivesIndices {
		if bytes.Compare(encodedHash, idx.HeaderHash[:]) == 0 {
			foundIndex = idx
			break
		}
	}
	if foundIndex.archiveHash == nil {
		// encodedHash was not found inside archive indices, try to download the whole file
		r, err := e.Storage.OpenData(encodedHash)
		if err != nil {
			return nil, err
		}

		b, err := ioutil.ReadAll(r) //TODO no blte decode?? unlike if found inside archive index
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return b, nil
	}
	archive, err := e.Storage.OpenData(foundIndex.archiveHash)
	if err != nil {
		return nil, err
	}
	if _, err := archive.Seek(int64(foundIndex.Offset), 0); err != nil {
		return nil, err
	}
	encodedFile := make([]byte, foundIndex.EncodedSize)
	if _, err := io.ReadFull(archive, encodedFile); err != nil {
		return nil, err
	}
	decodedFile := bytes.NewBuffer([]byte{})
	if err := blte.Decode(bytes.NewBuffer(encodedFile), decodedFile); err != nil {
		return nil, err
	}
	return decodedFile.Bytes(), nil
}

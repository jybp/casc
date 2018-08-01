package d3

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/jybp/go-d3-auto-parser/blte"

	"github.com/jybp/go-d3-auto-parser/casc"
)

func Parse(app string) error {
	ctx := context.Background()
	hostURL := casc.HostURL(casc.RegionUS)

	// Download and check version to set cache folder
	httpGetter := HTTPGetter{http.DefaultClient}
	verR, err := httpGetter.Get(ctx, casc.VersionsURL(hostURL, app))
	if err != nil {
		return err
	}

	vers, err := casc.ParseVersions(verR)
	if err != nil {
		return err
	}

	ver, ok := vers[casc.RegionUS]
	if !ok {
		return errors.New("ver region not found")
	}

	cache := FileCache{Getter: &httpGetter, CacheDir: "cache/" + app + "/" + strconv.Itoa(ver.ID)}

	// Download version file locally
	// verRcache, err := cache.Get(ctx, casc.VersionsURL(hostURL, app))
	// if err != nil {
	// 	return err
	// }

	// CDN urls
	cdnR, err := cache.Get(ctx, casc.CdnsURL(hostURL, app))
	if err != nil {
		return err
	}

	cdns, err := casc.ParseCdn(cdnR)
	if err != nil {
		return err
	}

	cdn, ok := cdns[casc.RegionUS]
	if !ok {
		return errors.New("cdn region not found")
	}

	// Build Config
	cfgR, err := cache.Get(ctx, cdn.Url(casc.TypeConfig, ver.BuildHash, false))
	if err != nil {
		return err
	}

	cfg, err := casc.ParseBuildConfig(cfgR)
	if err != nil {
		return err
	}

	// Encoding File
	encR, err := cache.Get(ctx, cdn.Url(casc.TypeData, cfg.EncodingHash[1], false))
	if err != nil {
		return err
	}

	encRdec := bytes.NewBuffer([]byte{})
	if err := blte.Decode(encR, encRdec); err != nil {
		return err
	}

	enc, err := casc.ParseEncoding(encRdec)
	if err != nil {
		return err
	}

	fmt.Println("encTable: ", len(enc.EncCTable))

	// CDN Config
	cdnCfgR, err := cache.Get(ctx, cdn.Url(casc.TypeConfig, ver.CDNHash, false))
	if err != nil {
		return err
	}

	cdnCfg, err := casc.ParseCdnConfig(cdnCfgR)
	if err != nil {
		return err
	}

	// Load all Archives Index
	fmt.Printf("loading archive indices (%d)\n", len(cdnCfg.ArchivesHashes))
	// map of archive hash => archive indices
	archivesIdxs := map[string][]casc.ArchiveIndexEntry{}
	for _, archiveHash := range cdnCfg.ArchivesHashes {
		idxR, err := cache.Get(ctx, cdn.Url(casc.TypeData, archiveHash, true))
		if err != nil {
			return err
		}
		idxs, err := casc.ParseArchiveIndex(idxR)

		if err != nil {
			return err
		}
		archivesIdxs[archiveHash] = append(archivesIdxs[archiveHash], idxs...)
	}

	//TOOD this should be already done inside cfg parsing?
	rootHash := make([]byte, hex.DecodedLen(len(cfg.RootHash)))
	if _, err := hex.Decode(rootHash, []byte(cfg.RootHash)); err != nil {
		return err
	}

	rootB, err := LoadFromContentHash(ctx, cache, cdn, rootHash, enc, archivesIdxs)
	if err != nil {
		return err
	}
	d3root, err := casc.ParseD3Root(bytes.NewReader(rootB))
	if err != nil {
		return err
	}
	for _, entry := range d3root.NamedEntries {
		fmt.Printf("getting \"%s\" with hash %x\n", entry.Filename, entry.ContentKey)

		file, err := LoadFromContentHash(ctx, cache, cdn, entry.ContentKey[:], enc, archivesIdxs)
		if err != nil {
			return err
		}
		fmt.Printf("%s len is: %d\n", entry.Filename, len(file))
	}

	return nil
}

func LoadFromContentHash(
	ctx context.Context,
	getter Getter,
	cdn casc.Cdn,
	contentHash []byte,
	enc casc.Encoding,
	archivesIdxs map[string][]casc.ArchiveIndexEntry) ([]byte, error) {

	encodedHash, err := enc.FindEncodedHash(contentHash)
	if err != nil {
		return nil, err
	}

	archiveInfo := struct {
		ArchiveHash string
		Index       casc.ArchiveIndexEntry
	}{}
	for archiveHash, indices := range archivesIdxs {
		for _, idx := range indices {

			//TODO emove useless check
			if len(encodedHash) != len(idx.HeaderHash[:]) {
				return nil, fmt.Errorf("inconsistent hash len %d and %d", len(encodedHash), len(idx.HeaderHash[:]))
			}

			if bytes.Compare(encodedHash, idx.HeaderHash[:]) == 0 {
				archiveInfo = struct {
					ArchiveHash string
					Index       casc.ArchiveIndexEntry
				}{archiveHash, idx}
				break
			}
		}
	}

	if archiveInfo.ArchiveHash == "" || archiveInfo.Index == (casc.ArchiveIndexEntry{}) {
		// encodedHash was not found inside archive indices, try to download the whole file
		r, err := getter.Get(ctx, cdn.Url(casc.TypeData, fmt.Sprintf("%x", encodedHash), false))
		if err != nil {
			return nil, err
		}
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
	archive, err := getter.Get(ctx, cdn.Url(casc.TypeData, archiveInfo.ArchiveHash, false))
	if err != nil {
		return nil, err
	}
	if _, err := archive.Seek(int64(archiveInfo.Index.Offset), 0); err != nil {
		return nil, err
	}
	encRootFile := make([]byte, archiveInfo.Index.EncodedSize)
	if _, err := io.ReadFull(archive, encRootFile); err != nil {
		return nil, err
	}
	rootFile := bytes.NewBuffer([]byte{})
	if err := blte.Decode(bytes.NewBuffer(encRootFile), rootFile); err != nil {
		return nil, err
	}
	return rootFile.Bytes(), nil
}

package d3

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/jybp/go-d3-auto-parser/blte"

	"github.com/jybp/go-d3-auto-parser/casc"
)

func Parse() error {

	hostURL := casc.HostURL(casc.RegionUS)
	app := "d3"

	// Download and check version to set cache folder
	dl := Downloader{}
	verR, err := dl.Download(casc.VersionsURL(hostURL, app))
	if err != nil {
		return err
	}
	defer verR.Close()

	vers, err := casc.ParseVersions(verR)
	if err != nil {
		return err
	}

	ver, ok := vers[casc.RegionUS]
	if !ok {
		return errors.New("ver region not found")
	}

	cache := Cache{Downloader: dl, Output: "cache/" + app + "_" + strconv.Itoa(ver.ID)}

	// Download version file locally
	verRcache, err := cache.Download(casc.VersionsURL(hostURL, app))
	if err != nil {
		return err
	}
	defer verRcache.Close()

	// CDN urls
	cdnR, err := cache.Download(casc.CdnsURL(hostURL, app))
	if err != nil {
		return err
	}
	defer cdnR.Close()

	cdns, err := casc.ParseCdn(cdnR)
	if err != nil {
		return err
	}

	cdn, ok := cdns[casc.RegionUS]
	if !ok {
		return errors.New("cdn region not found")
	}

	// Build Config
	cfgR, err := cache.Download(cdn.Url(casc.TypeConfig, ver.BuildHash, false))
	if err != nil {
		return err
	}
	defer cfgR.Close()

	cfg, err := casc.ParseBuildConfig(cfgR)
	if err != nil {
		return err
	}

	// Encoding File
	encR, err := cache.Download(cdn.Url(casc.TypeData, cfg.EncodingHash[1], false))
	if err != nil {
		return err
	}
	defer encR.Close()

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
	cdnCfgR, err := cache.Download(cdn.Url(casc.TypeConfig, ver.CDNHash, false))
	if err != nil {
		return err
	}
	defer cdnCfgR.Close()

	cdnCfg, err := casc.ParseCdnConfig(cdnCfgR)
	if err != nil {
		return err
	}

	// Root file: need decoded hash first
	// rootFileR, err := cache.Download(cdn.Url(casc.TypeData, cfg.RootHash, false))
	// if err != nil {
	// 	return err
	// }
	// defer cdnCfgR.Close()

	// _, err = casc.ParseRoot(rootFileR)
	// if err != nil {
	// 	return err
	// }

	// Look up Root hash
	// TODO process is the same for all file to DL
	// Get decoded hash of file, look up corresponding encodedHash in encoding table
	// download and blte decode the content
	decodedRootHashStr := cfg.RootHash
	decodedRootHash := make([]byte, hex.DecodedLen(len(decodedRootHashStr)))
	if _, err := hex.Decode(decodedRootHash, []byte(decodedRootHashStr)); err != nil {
		return err
	}

	for _, e := range enc.EncCTable {

		// a faster way would be to first look at e.Index.Hash which is the first content key of the table entries
		for _, entry := range e.Entries {
			if bytes.Compare(decodedRootHash, entry.Ckey) != 0 {
				continue
			}

			if len(entry.Ekey) == 0 {
				return fmt.Errorf("no encoding key for content key %x", entry.Ckey)
			}

			// pick encoded key at random, it doesnt matter
			fmt.Printf("MATCH: %x : enc hash is %x\n", decodedRootHash, string(entry.Ekey[0]))

			rootFile, err := cache.Download(cdn.Url(casc.TypeData, hex.EncodeToString(entry.Ekey[0]), false))
			if err != nil {
				return err
			}
			defer rootFile.Close()

			decodedRootFile := bytes.NewBuffer([]byte{})
			blte.Decode(rootFile, decodedRootFile)

			fmt.Println(decodedRootFile)
			return nil
			//Todo blte encoded
		}

	}

	return nil
	fmt.Println("archives hashes: ", len(cdnCfg.ArchivesHashes))

	// Archives Index
	// map HeaderHash to ArchiveIndexChunk
	archivesIdx := map[string]casc.ArchiveIndexEntry{}
	for i, archiveHash := range cdnCfg.ArchivesHashes {
		idxR, err := cache.Download(cdn.Url(casc.TypeData, archiveHash, true))
		if err != nil {
			return err
		}
		defer idxR.Close()

		idxs, err := casc.ParseArchiveIndex(idxR)
		if err != nil {
			return err
		}

		for j, idx := range idxs {
			archivesIdx[string(idx.HeaderHash[:])] = idx

			//TODO debug print
			if i == 0 && j < 10 {
				fmt.Println("archivesIdxEntry", idx)
			}
		}

	}

	return nil
}

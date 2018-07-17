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

	// return nil

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

		fmt.Printf("roothash: %s \t compared to: %s\n", decodedRootHashStr, hex.EncodeToString(e.Index.Hash))

		// for _, ekey := range e.Entry.Ekey {
		// 	fmt.Printf("roothash: %s \t compared to ekey: %s\n", decodedRootHashStr, hex.EncodeToString(ekey))

		// 	if bytes.Compare(decodedRootHash, ekey) == 0 {
		// 		fmt.Printf("MATCH: %s : enc hash is %s\n", decodedRootHash, ekey)
		// 		return nil
		// 	}
		// }

		if bytes.Compare(decodedRootHash, e.Index.Hash) == 0 ||
			bytes.Compare(decodedRootHash, e.Index.Checksum[:]) == 0 ||
			bytes.Compare(decodedRootHash, e.Entry.Ckey) == 0 ||
			bytes.Compare(decodedRootHash, e.Entry.Ekey[0]) == 0 {

			// pick encoded key at random, it doesnt matter
			encodedHash := string(e.Entry.Ekey[0])

			fmt.Printf("MATCH: %s : enc hash is %s\n", decodedRootHash, encodedHash)

			rootFile, err := cache.Download(cdn.Url(casc.TypeData, encodedHash, false))
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
	fmt.Printf("looking for decoded roohHash %s in %d entries", decodedRootHash, len(enc.EncCTable))

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

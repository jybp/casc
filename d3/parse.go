package d3

import (
	"bytes"
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
	if err = blte.Decode(encR, encRdec); err != nil {
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

	fmt.Println("archives hashes: ", len(cdnCfg.ArchivesHashes))

	// Archives
	// map archive hash to archive data
	archivesIdx := map[string]casc.ArchiveIndex{}
	for _, hash := range cdnCfg.ArchivesHashes {

		fmt.Println("archivesIdx", hash)

		idxR, err := cache.Download(cdn.Url(casc.TypeData, hash, true))
		if err != nil {
			return err
		}
		defer idxR.Close()

		idx, err := casc.ParseArchiveIndex(idxR)
		if err != nil {
			return err
		}

		archivesIdx[hash] = idx
	}

	return nil
}

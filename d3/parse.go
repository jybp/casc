package d3

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/jybp/go-d3-auto-parser/blte"

	"github.com/jybp/go-d3-auto-parser/csac"
)

func Parse() error {

	hostURL := csac.HostURL(csac.RegionUS)

	cache := Cache{Output: "cache"}

	app := "d3"
	verR, err := cache.Download(csac.VersionsURL(hostURL, app))
	if err != nil {
		return err
	}

	vers, err := csac.ParseVersions(verR)
	if err != nil {
		return err
	}

	cdnR, err := cache.Download(csac.CdnsURL(hostURL, app))
	if err != nil {
		return err
	}

	cdns, err := csac.ParseCdn(cdnR)
	if err != nil {
		return err
	}

	cdn, ok := cdns[csac.RegionUS]
	if !ok {
		return errors.New("cdn region not found")
	}

	ver, ok := vers[csac.RegionUS]
	if !ok {
		return errors.New("ver region not found")
	}

	cfgR, err := cache.Download(cdn.Url(csac.TypeConfig, ver.BuildHash, false))
	if err != nil {
		return err
	}

	cfg, err := csac.ParseBuildConfig(cfgR)
	if err != nil {
		return err
	}

	encR, err := cache.Download(cdn.Url(csac.TypeData, cfg.EncodingHash[1], false))
	if err != nil {
		return err
	}

	encRdec := bytes.NewBuffer([]byte{})
	if err = blte.Decode(encR, encRdec); err != nil {
		return err
	}

	enc, err := csac.ParseEncoding(encRdec)
	if err != nil {
		return err
	}

	fmt.Println("encTable: ", len(enc.EncCTable))

	cdnCfgR, err := cache.Download(cdn.Url(csac.TypeConfig, ver.CDNHash, false))
	if err != nil {
		return err
	}

	cdnCfg, err := csac.ParseCdnConfig(cdnCfgR)
	if err != nil {
		return err
	}

	fmt.Println("archives hashes: ", len(cdnCfg.ArchivesHashes))

	archivesIdx := []csac.ArchiveIndex{}
	for _, hash := range cdnCfg.ArchivesHashes {

		fmt.Println("archivesIdx", hash)

		idxR, err := cache.Download(cdn.Url(csac.TypeData, hash, true))
		if err != nil {
			return err
		}

		idx, err := csac.ParseArchiveIndex(idxR)
		if err != nil {
			return err
		}

		archivesIdx = append(archivesIdx, idx)
	}

	return nil
}

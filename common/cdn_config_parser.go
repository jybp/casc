package common

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
)

type CdnConfig struct {
	ArchivesHashes [][]byte
}

func ParseCdnConfig(r io.Reader) (CdnConfig, error) {
	cdnCfg := parseConfig(r)
	archivesHashes, err := configToHashes(cdnCfg, "archives")
	if err != nil {
		return CdnConfig{}, err
	}
	if len(archivesHashes) == 0 {
		return CdnConfig{}, errors.WithStack(fmt.Errorf("no archives hashes found in cdn config"))
	}
	return CdnConfig{archivesHashes}, nil
}

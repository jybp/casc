package common

import (
	"io"
)

type CdnConfig struct {
	ArchivesHashes [][]byte
}

func ParseCdnConfig(r io.Reader) (CdnConfig, error) {
	archives := "archives"
	_, hashes, err := parseConfig(r, nil, []string{archives})
	if err != nil {
		return CdnConfig{}, err
	}
	return CdnConfig{hashes[archives]}, nil
}

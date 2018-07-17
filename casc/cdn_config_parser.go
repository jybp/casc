package casc

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type CdnConfig struct {
	ArchivesHashes []string
}

func ParseCdnConfig(r io.Reader) (CdnConfig, error) {
	p := parseConfig(r)
	cfg := CdnConfig{}

	v, ok := p["archives"]
	if !ok {
		return cfg, errors.New("'archives' not found in config")
	}
	s := strings.Split(v, " ")
	if len(s) == 0 {
		return CdnConfig{}, fmt.Errorf("invalid 'archives' in config")
	}
	cfg.ArchivesHashes = s

	return cfg, nil
}

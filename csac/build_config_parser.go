package csac

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

//Build config
type BuildConfig struct {
	RootHash     string
	EncodingHash [2]string
	InstallHash  [2]string
}

func ParseBuildConfig(r io.Reader) (BuildConfig, error) {

	p := parseConfig(r)
	cfg := BuildConfig{}

	v, ok := p["root"]
	if !ok {
		return cfg, errors.New("'root' not found in config")
	}
	cfg.RootHash = v

	v, ok = p["install"]
	if !ok {
		return cfg, errors.New("'install' not found in config")
	}
	s := strings.Split(v, " ")
	if len(s) != 2 {
		return BuildConfig{}, fmt.Errorf("invalid 'install' in config")
	}
	cfg.InstallHash = [2]string{s[0], s[1]}

	v, ok = p["encoding"]
	if !ok {
		return cfg, errors.New("'encoding' not found in config")
	}
	s = strings.Split(v, " ")
	if len(s) != 2 {
		return BuildConfig{}, fmt.Errorf("invalid 'encoding' in config")
	}
	cfg.EncodingHash = [2]string{s[0], s[1]}

	return cfg, nil
}

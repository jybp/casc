package common

import (
	"errors"
	"io"
)

//Build config
type BuildConfig struct {
	RootHash     []byte
	EncodingHash [][]byte
}

func ParseBuildConfig(r io.Reader) (BuildConfig, error) {
	buildCfg := parseConfig(r)
	rootHashes, err := configToHashes(buildCfg, "root")
	if err != nil {
		return BuildConfig{}, err
	}
	if len(rootHashes) != 1 {
		return BuildConfig{}, errors.New("build config doesn't contain exactly one root hash")
	}
	encodingHashes, err := configToHashes(buildCfg, "encoding")
	if err != nil {
		return BuildConfig{}, err
	}
	return BuildConfig{
		RootHash:     rootHashes[0],
		EncodingHash: encodingHashes,
	}, nil
}

package common

import (
	"io"

	"github.com/pkg/errors"
)

//Build config
type BuildConfig struct {
	RootHash       []byte
	InstallHashes  [][]byte
	EncodingHashes [][]byte
}

func ParseBuildConfig(r io.Reader) (BuildConfig, error) {
	buildCfg := parseConfig(r)
	rootHashes, err := configToHashes(buildCfg, "root")
	if err != nil {
		return BuildConfig{}, err
	}
	if len(rootHashes) != 1 {
		return BuildConfig{}, errors.WithStack(errors.New("build config hash missing"))
	}
	encodingHashes, err := configToHashes(buildCfg, "encoding")
	if err != nil {
		return BuildConfig{}, err
	}
	if len(encodingHashes) < 1 {
		return BuildConfig{}, errors.WithStack(errors.New("build config hash missing"))
	}
	installHashes, err := configToHashes(buildCfg, "install")
	if err != nil {
		return BuildConfig{}, err
	}
	if len(installHashes) < 1 {
		return BuildConfig{}, errors.WithStack(errors.New("build config hash missing"))
	}
	return BuildConfig{
		RootHash:       rootHashes[0],
		InstallHashes:  installHashes,
		EncodingHashes: encodingHashes,
	}, nil
}

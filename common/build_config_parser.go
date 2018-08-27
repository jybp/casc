package common

import (
	"io"
)

type BuildConfig struct {
	BuildProduct   string
	RootHash       []byte
	EncodingHashes [][]byte
}

func ParseBuildConfig(r io.Reader) (BuildConfig, error) {
	buildProduct, root, encoding := "build-product", "root", "encoding"
	values, hashes, err := parseConfig(r, []string{buildProduct}, []string{root, encoding})
	if err != nil {
		return BuildConfig{}, err
	}
	return BuildConfig{
		BuildProduct:   values[buildProduct],
		RootHash:       hashes[root][0],
		EncodingHashes: hashes[encoding],
	}, nil
}

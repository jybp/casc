package common

import (
	"io"
)

type BuildConfig struct {
	RootHash       []byte
	EncodingHashes [][]byte
}

func ParseBuildConfig(r io.Reader) (BuildConfig, error) {
	root, encoding := "root", "encoding"
	hashes, err := parseConfig(r, root, encoding)
	if err != nil {
		return BuildConfig{}, err
	}
	return BuildConfig{
		RootHash:       hashes[root][0],
		EncodingHashes: hashes[encoding],
	}, nil
}

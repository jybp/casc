package common

import (
	"bytes"
	"encoding/hex"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type BuildConfig struct {
	BuildProduct   string
	RootHash       []byte
	EncodingHashes [][]byte

	// VfsRoot is set for products using a TVFS root (i.e. Diablo II Resurrected).
	// VfsRoot[0] is the content hash and VfsRoot[1] the encoded hash.
	VfsRoot [][]byte
	// VfsEncodedHashes contains the encoded hashes of every vfs-N entry (including vfs-root).
	// Those are used to detect nested TVFS directories.
	VfsEncodedHashes [][]byte
}

func ParseBuildConfig(r io.Reader) (BuildConfig, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return BuildConfig{}, errors.WithStack(err)
	}
	buildProduct, root, encoding := "build-product", "root", "encoding"
	values, hashes, err := parseConfig(bytes.NewReader(b), []string{buildProduct}, []string{root, encoding})
	if err != nil {
		return BuildConfig{}, err
	}
	cfg := BuildConfig{
		BuildProduct:   values[buildProduct],
		RootHash:       hashes[root][0],
		EncodingHashes: hashes[encoding],
	}

	// Optional TVFS entries.
	for _, line := range strings.Split(string(b), "\n") {
		kv := strings.SplitN(strings.TrimSpace(line), " = ", 2)
		if len(kv) != 2 || !strings.HasPrefix(kv[0], "vfs-") || strings.HasSuffix(kv[0], "-size") {
			continue
		}
		suffix := strings.TrimPrefix(kv[0], "vfs-")
		if _, err := strconv.Atoi(suffix); err != nil && suffix != "root" {
			continue
		}
		var hs [][]byte
		for _, hashStr := range strings.Fields(kv[1]) {
			h, err := hex.DecodeString(hashStr)
			if err != nil {
				return BuildConfig{}, errors.WithStack(errors.Errorf("invalid %s hash", kv[0]))
			}
			hs = append(hs, h)
		}
		if len(hs) < 2 {
			continue
		}
		if suffix == "root" {
			cfg.VfsRoot = hs
		}
		cfg.VfsEncodedHashes = append(cfg.VfsEncodedHashes, hs[1])
	}
	return cfg, nil
}

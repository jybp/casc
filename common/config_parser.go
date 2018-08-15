package common

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

func parseConfig(r io.Reader) map[string]string {
	cfg := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		i := strings.Index(line, " = ")
		if i <= 0 {
			continue
		}
		cfg[line[0:i]] = line[i+3:]
	}
	return cfg
}

func configToHashes(cfg map[string]string, name string) ([][]byte, error) {
	v, ok := cfg[name]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("%s not found in build config", name))
	}
	split := strings.Split(v, " ")
	hashes := [][]byte{}
	for _, s := range split {
		hash, err := hex.DecodeString(s)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

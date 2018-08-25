package common

import (
	"bufio"
	"encoding/hex"
	"io"
	"strings"

	"github.com/pkg/errors"
)

// parseConfig returns an error if not all keys are found.
// Values must be hex encoded hashes separated by space characters.
// At least one hash must be present by key.
func parseConfig(r io.Reader, keys ...string) (map[string][][]byte, error) {
	keysCheck := map[string]struct{}{}
	for _, k := range keys {
		keysCheck[k] = struct{}{}
	}
	lookup := map[string][][]byte{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		kv := strings.Split(line, " = ")
		if len(kv) != 2 {
			continue
		}
		if _, ok := keysCheck[kv[0]]; !ok {
			continue
		}
		delete(keysCheck, kv[0])
		hashesStr := strings.Split(kv[1], " ")
		if len(hashesStr) == 0 {
			return nil, errors.WithStack(errors.New("invalid config"))
		}
		for _, hashStr := range hashesStr {
			hash, err := hex.DecodeString(hashStr)
			if err != nil {
				return nil, errors.WithStack(errors.New("invalid config"))
			}
			lookup[kv[0]] = append(lookup[kv[0]], hash)
		}
	}
	if len(keysCheck) > 0 {
		return nil, errors.WithStack(errors.New("invalid config"))
	}
	return lookup, nil
}

// func parseConfig(r io.Reader) map[string]string {
// 	cfg := map[string]string{}
// 	scanner := bufio.NewScanner(r)
// 	for scanner.Scan() {
// 		line := scanner.Text()
// 		i := strings.Index(line, " = ")
// 		if i <= 0 {
// 			continue
// 		}
// 		cfg[line[0:i]] = line[i+3:]
// 	}
// 	return cfg
// }

// func configToHashes(cfg map[string]string, name string) ([][]byte, error) {
// 	v, ok := cfg[name]
// 	if !ok {
// 		return nil, errors.WithStack(fmt.Errorf("%s not found in build config", name))
// 	}
// 	split := strings.Split(v, " ")
// 	hashes := [][]byte{}
// 	for _, s := range split {
// 		hash, err := hex.DecodeString(s)
// 		if err != nil {
// 			return nil, errors.WithStack(err)
// 		}
// 		hashes = append(hashes, hash)
// 	}
// 	return hashes, nil
// }

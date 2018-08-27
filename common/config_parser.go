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
func parseConfig(r io.Reader, keys []string, hashesKeys []string) (map[string]string, map[string][][]byte, error) {
	keysCheck := map[string]struct{}{}
	for _, k := range keys {
		keysCheck[k] = struct{}{}
	}
	hashesKeysCheck := map[string]struct{}{}
	for _, k := range hashesKeys {
		hashesKeysCheck[k] = struct{}{}
	}
	keysLookup := map[string]string{}
	hashesLookup := map[string][][]byte{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		kv := strings.Split(line, " = ")
		if len(kv) != 2 {
			continue
		}
		if _, ok := keysCheck[kv[0]]; ok {
			delete(keysCheck, kv[0])
			keysLookup[kv[0]] = kv[1]
		}
		if _, ok := hashesKeysCheck[kv[0]]; ok {
			delete(hashesKeysCheck, kv[0])
			hashesStr := strings.Split(kv[1], " ")
			if len(hashesStr) == 0 {
				return nil, nil, errors.WithStack(errors.New("invalid config"))
			}
			for _, hashStr := range hashesStr {
				hash, err := hex.DecodeString(hashStr)
				if err != nil {
					return nil, nil, errors.WithStack(errors.New("invalid config"))
				}
				hashesLookup[kv[0]] = append(hashesLookup[kv[0]], hash)
			}
		}

	}
	if err := scanner.Err(); err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if len(hashesKeysCheck) > 0 || len(keysCheck) > 0 {
		return nil, nil, errors.WithStack(errors.New("invalid config"))
	}
	return keysLookup, hashesLookup, nil
}

package warcraft3

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/jybp/casc/common"
	"github.com/pkg/errors"
)

type Root struct {
	nameToContentHash map[string][]byte
}

func (r *Root) Files() ([]string, error) {
	//TODO same code everywhere, and executed at each func call
	names := []string{}
	for name := range r.nameToContentHash {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (r *Root) ContentHash(filename string) ([]byte, error) {
	contentHash, ok := r.nameToContentHash[filename]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("%s file name not found", filename))
	}
	return contentHash[:], nil
}

func NewRoot(root []byte) (*Root, error) {
	nameToContentHash := map[string][]byte{}
	scanner := bufio.NewScanner(bytes.NewReader(root))
	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, "|")
		if len(splits) < 2 {
			return nil, errors.WithStack(errors.New("invalid Warcraft 3 root"))
		}
		name := splits[0]
		hashStr := splits[1]
		hash, err := hex.DecodeString(hashStr)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		nameToContentHash[common.CleanPath(name)] = hash
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.WithStack(err)
	}
	return &Root{nameToContentHash}, nil
}

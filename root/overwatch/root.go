package overwatch

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/jybp/casc/common"
	"github.com/pkg/errors"
)

type Root struct {
	nameToContentHash map[string][]byte
}

func (r *Root) Files() ([]string, error) {
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

func NewRoot(root []byte, fetchFn func(contentHash []byte) ([]byte, error)) (*Root, error) {
	// #FILEID | MD5 | CHUNK_ID | PRIORITY | MPRIORITY | FILENAME | INSTALLPATH
	csv := csv.NewReader(bytes.NewReader(root))
	csv.Comma = '|'
	csv.Comment = '#'
	lines, err := csv.ReadAll()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(lines) == 0 {
		return nil, errors.WithStack(errors.New("invalid csv"))
	}
	nameToContentHash := map[string][]byte{}
	for _, line := range lines {
		md5 := line[1]
		filename := line[5]
		hash, err := hex.DecodeString(md5)
		if err != nil {
			return nil, errors.WithStack(errors.New("invalid md5"))
		}
		nameToContentHash[common.CleanPath(filename)] = hash
	}
	//TODO nearly all files are missing from the root file
	return &Root{nameToContentHash}, nil
}

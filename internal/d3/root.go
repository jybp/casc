package d3

import (
	"fmt"
)

type Root struct {
	RootHash []byte
	Extract  func(contentHash []byte) ([]byte, error)

	filenameToContentHash map[string][]byte
}

func (r *Root) Files() ([]string, error) {
	if err := r.setup(); err != nil {
		return nil, err
	}
	var names []string
	for name := range r.filenameToContentHash {
		names = append(names, name)
	}
	return names, nil
}

func (r *Root) ContentHash(filename string) ([]byte, error) {
	if err := r.setup(); err != nil {
		return nil, err
	}
	contentHash, ok := r.filenameToContentHash[filename]
	if !ok {
		return nil, fmt.Errorf("%s file name not found", filename)
	}
	return contentHash, nil
}

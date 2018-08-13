package diablo3

import (
	"fmt"
)

type Root struct {
	filenameToContentHash map[string][]byte
}

func (r *Root) Files() ([]string, error) {
	var names []string
	for name := range r.filenameToContentHash {
		names = append(names, name)
	}
	return names, nil
}

func (r *Root) ContentHash(filename string) ([]byte, error) {
	contentHash, ok := r.filenameToContentHash[filename]
	if !ok {
		return nil, fmt.Errorf("%s file name not found", filename)
	}
	return contentHash, nil
}

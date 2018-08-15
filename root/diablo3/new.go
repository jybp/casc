package diablo3

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
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
		return nil, errors.WithStack(fmt.Errorf("%s file name not found", filename))
	}
	return contentHash, nil
}

func NewRoot(hash []byte, extract func(contentHash []byte) ([]byte, error)) (*Root, error) {
	b, err := extract(hash)
	if err != nil {
		return nil, err
	}
	d3root, err := parseD3RootFile(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	filenameToContentHash := map[string][]byte{}
	for _, entry := range d3root.NamedEntries {
		// fmt.Printf("getting \"%s\" with hash %x\n", entry.Filename, entry.ContentKey)
		if entry.Filename == "Windows" || entry.Filename == "Mac" {
			// Those files cannot be downloaded for some reason
			continue
		}
		filenameToContentHash[entry.Filename] = entry.ContentKey[:]
		// file, err := r.Extract(entry.ContentKey[:])
		// if err != nil {
		// 	return err
		// }
		// fmt.Printf("%s len is: %s\n", entry.Filename, size(len(file)))
	}
	return &Root{
		filenameToContentHash: filenameToContentHash,
	}, nil
}

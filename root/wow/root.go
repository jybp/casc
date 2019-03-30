package wow

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"

	"github.com/pkg/errors"
)

type BlockHeader struct {
	Count        uint32
	ContentFlags uint32
	LocalFlags   uint32
}

type Record struct {
	ContentHash [0x10]byte
	NameHash    uint64 // Jenkins96 (lookup3) hash of the file's path
}

type Root struct {
	nameToContentHash map[string][0x10]byte
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

// https://wowdev.wiki/TACT#World_of_Warcraft
func NewRoot(root []byte) (*Root, error) {
	r := bytes.NewReader(root)
	nameToContentHash := map[string][0x10]byte{}
	for {
		blockHeader := BlockHeader{}
		if err := binary.Read(r, binary.LittleEndian, &blockHeader); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.WithStack(err)
		}
		fileDataIDDeltas := make([]uint32, blockHeader.Count)
		if err := binary.Read(r, binary.LittleEndian, &fileDataIDDeltas); err != nil {
			return nil, errors.WithStack(err)
		}
		records := make([]Record, blockHeader.Count)
		if err := binary.Read(r, binary.LittleEndian, &records); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return &Root{nameToContentHash}, nil
}

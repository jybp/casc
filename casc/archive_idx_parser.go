package casc

import (
	"bytes"
	"encoding/binary"
	"io"
)

const arBlockSize uint32 = 1 << 20

type ArchiveIndexEntry struct {
	HeaderHash  [8]uint8
	EncodedSize uint32
	Offset      uint32
}

//TODO simpler impl
func ParseArchiveIndex(r io.Reader) ([]ArchiveIndexEntry, error) {
	idxs := []ArchiveIndexEntry{}
	for {
		var chunk [4096]uint8
		if err := binary.Read(r, binary.BigEndian, &chunk); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return idxs, nil
			}
			return nil, err
		}

		buf := bytes.NewBuffer(chunk[:])
		for {
			idxEntry := ArchiveIndexEntry{}
			if err := binary.Read(buf, binary.BigEndian, &idxEntry); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return idxs, nil
				}
				return nil, err
			}

			if idxEntry == (ArchiveIndexEntry{}) {
				break
			}
			idxs = append(idxs, idxEntry)
		}
	}
}

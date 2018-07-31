package casc

import (
	"bytes"
	"encoding/binary"
	"io"
)

type ArchiveIndexEntry struct {
	HeaderHash  [16]uint8 /*size is actually present in footer...*/
	EncodedSize uint32
	Offset      uint32
}

//TODO simpler impl
func ParseArchiveIndex(r io.Reader) ([]ArchiveIndexEntry, error) {
	idxs := []ArchiveIndexEntry{}
	for {
		var chunk [1 << 12]uint8 /*fixed size*/
		if err := binary.Read(r, binary.BigEndian, &chunk); err != nil {
			/*TODO footer reached*/
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

			// zero padding reached
			if idxEntry.HeaderHash == [16]uint8{} {
				break
			}
			idxs = append(idxs, idxEntry)
		}
	}
}

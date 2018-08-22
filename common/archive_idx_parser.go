package common

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/pkg/errors"
)

type ArchiveIndexEntry struct {
	HeaderHash  [0x10]uint8 /* first checksumSize bytes of the MD5 of the respective data. Size is actually to be found in the footer. */
	EncodedSize uint32      /* encoding size of the respective data inside the archive */ /*todo byte size is actually in the footer*/
	Offset      uint32      /* offset of the respective data inside the archive */        /*todo byte size is actually in the footer*/
}

func ParseArchiveIndex(r io.Reader) ([]ArchiveIndexEntry, error) {
	idxs := []ArchiveIndexEntry{}
	for {
		var chunk [1 << 12]uint8 /*fixed size*/
		if err := binary.Read(r, binary.BigEndian, &chunk); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return idxs, nil
			}
			return nil, errors.WithStack(err)
		}
		buf := bytes.NewBuffer(chunk[:])
		for {
			idxEntry := ArchiveIndexEntry{}
			if err := binary.Read(buf, binary.BigEndian, &idxEntry); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return idxs, nil
				}
				return nil, errors.WithStack(err)
			}
			// zero padding reached
			if idxEntry.HeaderHash == [16]uint8{} {
				break
			}
			idxs = append(idxs, idxEntry)
		}
	}
}

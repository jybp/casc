package common

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"
)

type ArchiveIndexEntry struct {
	HeaderHash  [0x10]uint8
	EncodedSize uint32
	Offset      uint32
}

func ParseArchiveIndex(r io.ReadSeeker) ([]ArchiveIndexEntry, error) {
	pos, err := r.Seek(-12, io.SeekEnd)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	length := pos + 12
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, errors.WithStack(err)
	}
	if int64(count*(16+4+4)) > length {
		return nil, errors.WithStack(errors.New("archive index invalid length"))
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, errors.WithStack(err)
	}
	indices := []ArchiveIndexEntry{}
	for i := uint32(0); i < count; i++ {
		indexEntry := ArchiveIndexEntry{}
		if err := binary.Read(r, binary.BigEndian, &indexEntry.HeaderHash); err != nil {
			return nil, errors.WithStack(err)
		}
		if indexEntry.HeaderHash == ([0x10]byte{}) {
			i = i - 1
			continue
		}
		if err := binary.Read(r, binary.BigEndian, &indexEntry.EncodedSize); err != nil {
			return nil, errors.WithStack(err)
		}
		if err := binary.Read(r, binary.BigEndian, &indexEntry.Offset); err != nil {
			return nil, errors.WithStack(err)
		}
		indices = append(indices, indexEntry)
	}
	return indices, nil
}

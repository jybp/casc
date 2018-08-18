package common

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

type IndexHeader struct {
	HeaderHashSize          uint32
	HeaderHash              uint32
	Unk0                    uint16
	BucketIndex             uint8
	Unk1                    uint8
	EntrySizeBytes          uint8
	EntryOffsetBytes        uint8
	EntryKeyBytes           uint8
	ArchiveFileHeaderBytes  uint8
	ArchiveTotalSizeMaximum uint64
	Padding                 [8]uint8
	EntriesSize             uint32
	EntriesHash             uint32
}

type IdxEntry struct {
	Key    []byte //first len(Key) bytes of the key
	Index  int    //archive name
	Offset int
	Size   uint32
}

func ParseIdx(r io.Reader) ([]IdxEntry, error) {
	h := IndexHeader{}
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, errors.WithStack(err)
	}

	entrySize := int(h.EntrySizeBytes) + int(h.EntryOffsetBytes) + int(h.EntryKeyBytes)
	numberOfEntries := int(h.EntriesSize) / entrySize
	fmt.Printf("%+v %d\n", h, numberOfEntries)

	entries := []IdxEntry{}
	for i := 0; i < numberOfEntries; i++ {
		key := make([]uint8, h.EntryKeyBytes)
		if err := binary.Read(r, binary.LittleEndian, &key); err != nil {
			return nil, errors.WithStack(err)
		}

		//40 bits int.
		//top 10 bits = name of the archive: data.XXX ; bottom 30 bits = offset in that archive.
		var high uint8
		if err := binary.Read(r, binary.BigEndian, &high); err != nil {
			return nil, errors.WithStack(err)
		}
		var low uint32
		if err := binary.Read(r, binary.BigEndian, &low); err != nil {
			return nil, errors.WithStack(err)
		}
		u64 := (uint64(high) << 32) | uint64(low)
		offset := u64 & 0x3fffffff
		index := u64 >> 30

		var size uint32
		if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
			return nil, errors.WithStack(err)
		}
		entries = append(entries, IdxEntry{
			Key:    key,
			Index:  int(index),
			Offset: int(offset),
			Size:   size,
		})
	}
	return entries, nil
}

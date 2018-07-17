package casc

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type encEPageEntry struct {
	Hash   []uint8 /*EHashSize*/
	Eindex uint32
	Unk    uint8
	Size   uint32
}

type encCPageEntry struct {
	KeyCount uint16
	FileSize uint32
	Ckey     []uint8   /*CHashSize*/
	Ekey     [][]uint8 /*[EHashSize][KeyCount]*/
}

type encPageIndex struct {
	Hash     []uint8 /*XHashSize*/
	Checksum [16]uint8
}

type encFileHeader struct {
	Signature      uint16
	Version        uint8
	CHashSize      uint8
	EHashSize      uint8
	CPageSize      uint16
	EPageSize      uint16
	CPageCount     uint32
	EPageCount     uint32
	Unknown        uint8
	EspecBlockSize uint32
}

type encCTableEntry struct {
	Index encPageIndex
	Entry encCPageEntry
}

type Encoding struct {
	EncCTable []encCTableEntry
}

//Warning: BLTE encoded
func ParseEncoding(r io.Reader) (Encoding, error) {
	h := &encFileHeader{}
	err := binary.Read(r, binary.BigEndian, h)
	if err != nil {
		return Encoding{}, err
	}

	fmt.Println("header", h)

	if h.Signature != 0x454e /*EN*/ {
		return Encoding{}, errors.New("invalid encoding header")
	}

	_, err = io.ReadFull(r, make([]uint8, h.EspecBlockSize))
	if err != nil {
		return Encoding{}, err
	}

	var cPageIndices []encPageIndex
	for i := uint32(0); i < h.CPageCount; i++ {
		idx := encPageIndex{}
		idx.Hash = make([]uint8, h.CHashSize)
		err := binary.Read(r, binary.BigEndian, &idx.Hash)
		if err != nil {
			return Encoding{}, err
		}

		err = binary.Read(r, binary.BigEndian, &idx.Checksum)
		if err != nil {
			return Encoding{}, err
		}

		cPageIndices = append(cPageIndices, idx)
	}

	encoding := Encoding{}
	for _, idx := range cPageIndices {
		CTableData := make([]byte, h.CPageSize*1024)
		err = binary.Read(r, binary.BigEndian, &CTableData)
		if err != nil {
			return Encoding{}, err
		}

		hash := md5.Sum(CTableData)
		if string(hash[:]) != string(idx.Checksum[:]) {
			return Encoding{}, errors.New("encoding file invalid checksum")
		}

		CTableDataBuf := bytes.NewBuffer(CTableData)
		cEntry := encCPageEntry{}
		err = binary.Read(CTableDataBuf, binary.LittleEndian, &cEntry.KeyCount)
		if err != nil {
			return Encoding{}, err
		}

		err = binary.Read(CTableDataBuf, binary.BigEndian, &cEntry.FileSize)
		if err != nil {
			return Encoding{}, err
		}

		cEntry.Ckey = make([]uint8, h.CHashSize)
		err = binary.Read(CTableDataBuf, binary.BigEndian, &cEntry.Ckey)
		if err != nil {
			return Encoding{}, err
		}

		for i := uint16(0); i < cEntry.KeyCount; i++ {
			ekey := make([]uint8, h.EHashSize)
			err = binary.Read(CTableDataBuf, binary.BigEndian, &ekey)
			if err != nil {
				return Encoding{}, err
			}

			cEntry.Ekey = append(cEntry.Ekey, ekey)
		}

		encoding.EncCTable = append(encoding.EncCTable, encCTableEntry{Index: idx, Entry: cEntry})
	}

	//CETable is next

	return encoding, nil
}

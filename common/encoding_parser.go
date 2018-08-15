package common

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pkg/errors"
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
	Checksum [0x10]uint8
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
	Index   encPageIndex
	Entries []encCPageEntry
}

type Encoding struct {
	EncCTable []encCTableEntry
}

func (e Encoding) FindEncodedHash(decodedHash []byte) (encodedHash []byte, err error) {
	for _, tableEntry := range e.EncCTable {
		// a faster is to first look at e.Index.Hash which is the first content key of the table entries
		// indices hash are ordered asc
		// Once we reach an index hash that is > to the hash we look for, we know the entry we look for is inside the previous encCTableEntry
		for _, entry := range tableEntry.Entries {
			if bytes.Compare(decodedHash, entry.Ckey) != 0 {
				continue
			}
			if len(entry.Ekey) == 0 {
				return nil, errors.WithStack(fmt.Errorf("no encoding key for content key %x", entry.Ckey))
			}
			// return any encoded hash
			return entry.Ekey[0], nil
		}
	}
	return nil, errors.WithStack(fmt.Errorf("no encoded hash found for decoded hash %x", decodedHash))
}

//Warning: BLTE encoded
func ParseEncoding(r io.Reader) (Encoding, error) {
	h := &encFileHeader{}
	if err := binary.Read(r, binary.BigEndian, h); err != nil {
		return Encoding{}, errors.WithStack(err)
	}

	if h.Signature != 0x454e /*EN*/ {
		return Encoding{}, errors.WithStack(errors.New("invalid encoding header"))
	}

	if _, err := io.ReadFull(r, make([]uint8, h.EspecBlockSize)); err != nil {
		return Encoding{}, errors.WithStack(err)
	}

	var cPageIndices []encPageIndex
	for i := uint32(0); i < h.CPageCount; i++ {
		idx := encPageIndex{}
		idx.Hash = make([]uint8, h.CHashSize)
		if err := binary.Read(r, binary.BigEndian, &idx.Hash); err != nil {
			return Encoding{}, errors.WithStack(err)
		}
		if err := binary.Read(r, binary.BigEndian, &idx.Checksum); err != nil {
			return Encoding{}, errors.WithStack(err)
		}
		cPageIndices = append(cPageIndices, idx)
	}

	encoding := Encoding{}
	for _, idx := range cPageIndices {
		CTableData := make([]byte, int(h.CPageSize)*1024)
		if err := binary.Read(r, binary.BigEndian, &CTableData); err != nil {
			return Encoding{}, errors.WithStack(err)
		}

		if hash := md5.Sum(CTableData); bytes.Compare(hash[:], idx.Checksum[:]) != 0 {
			return Encoding{}, errors.WithStack(errors.New("encoding file invalid checksum"))
		}

		entries := []encCPageEntry{}
		CTableDataBuf := bytes.NewBuffer(CTableData)

		for i := uint32(0); ; /*until EOF or until padding (cEntry.KeyCount == 0)*/ i++ {
			cEntry := encCPageEntry{}
			if err := binary.Read(CTableDataBuf, binary.LittleEndian, &cEntry.KeyCount); err != nil {
				// Not sure this check is actually needed. Never encountered a table that is perfectly filled yet. Always hit a zero padding beforehand
				// if err == io.EOF{
				// 	break
				// }
				return Encoding{}, errors.WithStack(err)
			}

			if cEntry.KeyCount == 0 {
				//a page is zero padded once entries have filled it
				break
			}

			if err := binary.Read(CTableDataBuf, binary.BigEndian, &cEntry.FileSize); err != nil {
				return Encoding{}, errors.WithStack(err)
			}

			cEntry.Ckey = make([]uint8, h.CHashSize)
			if err := binary.Read(CTableDataBuf, binary.BigEndian, &cEntry.Ckey); err != nil {
				return Encoding{}, errors.WithStack(err)
			}

			for i := uint16(0); i < cEntry.KeyCount; i++ {
				ekey := make([]uint8, h.EHashSize)
				if err := binary.Read(CTableDataBuf, binary.BigEndian, &ekey); err != nil {
					return Encoding{}, errors.WithStack(err)
				}
				cEntry.Ekey = append(cEntry.Ekey, ekey)
			}
			entries = append(entries, cEntry)
		}
		encoding.EncCTable = append(encoding.EncCTable, encCTableEntry{Index: idx, Entries: entries})
	}

	//EKeySpecPageTable is next
	return encoding, nil
}

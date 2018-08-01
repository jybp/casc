package casc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

//d3 specific

type NamedEntry struct {
	ContentKey [0x10]uint8
	Filename   string
}

type D3Root struct {
	NamedEntries []NamedEntry
}

func ParseD3Root(r io.Reader) (D3Root, error) {
	var sig uint32
	if err := binary.Read(r, binary.LittleEndian, &sig); err != nil {
		return D3Root{}, err
	}

	if sig != 0x8007D0C4 /* Diablo III */ {
		return D3Root{}, fmt.Errorf("invalid Diablo III root signature %x", sig)
	}

	//Root only contains named entries
	namedEntries, err := parseNamedEntries(r)
	if err != nil {
		return D3Root{}, err
	}

	return D3Root{namedEntries}, nil
}

func parseNamedEntries(r io.Reader) ([]NamedEntry, error) {
	var numberNamedEntries uint32
	if err := binary.Read(r, binary.LittleEndian, &numberNamedEntries); err != nil { //TODO should be BigEndian?!
		return nil, err
	}
	fmt.Printf("named entries in root: %d\n", numberNamedEntries)
	namedEntries := []NamedEntry{}
	for i := uint32(0); i < numberNamedEntries; i++ {
		namedEntry := NamedEntry{}
		if err := binary.Read(r, binary.BigEndian, &namedEntry.ContentKey); err != nil {
			return nil, err
		}
		filenameBuf := bytes.NewBufferString("")
		for {
			var c byte
			if err := binary.Read(r, binary.BigEndian, &c); err != nil {
				return nil, err
			}
			if c == 0 { //ASCIIZ
				break
			}
			filenameBuf.WriteByte(c)
		}
		namedEntry.Filename = filenameBuf.String()
		fmt.Printf("NamedEntry: %x %s\n", namedEntry.ContentKey, namedEntry.Filename)
		namedEntries = append(namedEntries, namedEntry)
	}
	return namedEntries, nil
}

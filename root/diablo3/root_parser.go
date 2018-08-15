package diablo3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

type namedEntry struct {
	ContentKey [0x10]uint8
	Filename   string
}

type d3RootFile struct {
	NamedEntries []namedEntry
}

func parseD3RootFile(r io.Reader) (d3RootFile, error) {
	var sig uint32
	if err := binary.Read(r, binary.LittleEndian, &sig); err != nil {
		return d3RootFile{}, errors.WithStack(err)
	}
	if sig != 0x8007D0C4 /* Diablo III */ {
		return d3RootFile{}, errors.WithStack(fmt.Errorf("invalid Diablo III root signature %x", sig))
	}
	//Root only contains named entries
	namedEntries, err := parseNamedEntries(r)
	if err != nil {
		return d3RootFile{}, err
	}
	return d3RootFile{namedEntries}, nil
}

func parseNamedEntries(r io.Reader) ([]namedEntry, error) {
	var numberNamedEntries uint32
	if err := binary.Read(r, binary.LittleEndian, &numberNamedEntries); err != nil { //TODO should be BigEndian?!
		return nil, errors.WithStack(err)
	}
	// fmt.Printf("named entries in root: %d\n", numberNamedEntries)
	namedEntries := []namedEntry{}
	for i := uint32(0); i < numberNamedEntries; i++ {
		namedEntry := namedEntry{}
		if err := binary.Read(r, binary.BigEndian, &namedEntry.ContentKey); err != nil {
			return nil, errors.WithStack(err)
		}
		filenameBuf := bytes.NewBufferString("")
		for {
			var c byte
			if err := binary.Read(r, binary.BigEndian, &c); err != nil {
				return nil, errors.WithStack(err)
			}
			if c == 0 { //ASCIIZ
				break
			}
			filenameBuf.WriteByte(c)
		}
		namedEntry.Filename = filenameBuf.String()
		// fmt.Printf("namedEntry: %x %s\n", namedEntry.ContentKey, namedEntry.Filename)
		namedEntries = append(namedEntries, namedEntry)
	}
	return namedEntries, nil
}

func size(b int) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

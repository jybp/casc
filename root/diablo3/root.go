package diablo3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

type NamedEntry struct {
	ContentHash [0x10]uint8
	Filename    string
}

type AssetEntry struct {
	ContentHash [0x10]uint8
	SNOID       uint32
}

type AssetIdxEntry struct {
	ContentHash [0x10]uint8
	SNOID       uint32
	FileIndex   uint32
}

type Root struct {
	lookup map[string][]byte
}

func (r *Root) Files() ([]string, error) {
	var names []string
	for name := range r.lookup {
		names = append(names, name)
	}
	return names, nil
}

func (r *Root) ContentHash(filename string) ([]byte, error) {
	contentHash, ok := r.lookup[filename]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("%s file name not found", filename))
	}
	return contentHash, nil
}

func NewRoot(rootHash []byte, dataFromContentHashFn func(contentHash []byte) ([]byte, error)) (*Root, error) {
	rootB, err := dataFromContentHashFn(rootHash)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(rootB)
	var sig uint32
	if err := binary.Read(r, binary.LittleEndian, &sig); err != nil {
		return nil, errors.WithStack(err)
	}
	if sig != 0x8007D0C4 /* Diablo III */ {
		return nil, errors.WithStack(fmt.Errorf("invalid Diablo III root signature %x", sig))
	}

	var namedEntriesCount uint32
	if err := binary.Read(r, binary.LittleEndian, &namedEntriesCount); err != nil {
		return nil, errors.WithStack(err)
	}

	lookup := map[string][]byte{}
	for i := uint32(0); i < namedEntriesCount; i++ {
		dirEntry := NamedEntry{}
		if err := binary.Read(r, binary.BigEndian, &dirEntry.ContentHash); err != nil {
			return nil, errors.WithStack(err)
		}
		if err := readASCIIZ(r, &dirEntry.Filename); err != nil {
			return nil, errors.WithStack(err)
		}

		dirB, err := dataFromContentHashFn(dirEntry.ContentHash[:])
		if err != nil {
			fmt.Printf("fetching %s (%x) err: %+v\n", dirEntry.Filename, dirEntry.ContentHash, err)
			continue
		}
		dirR := bytes.NewReader(dirB)

		// sig uint32
		// number of AssetEntry uint32
		// []AssetEntry
		// number of AssetIdxEntry uint32
		// []AssetIdxEntry
		// number of NamedEntry uint32
		// []NamedEntry

		var sig uint32
		if err := binary.Read(dirR, binary.LittleEndian, &sig); err != nil {
			return nil, errors.WithStack(err)
		}
		var assetCount uint32
		if err := binary.Read(dirR, binary.LittleEndian, &assetCount); err != nil {
			return nil, errors.WithStack(err)
		}
		for i := uint32(0); i < assetCount; i++ {
			assetEntry := AssetEntry{}
			if err := binary.Read(dirR, binary.LittleEndian, &assetEntry.ContentHash); err != nil {
				return nil, errors.WithStack(err)
			}
			if err := binary.Read(dirR, binary.LittleEndian, &assetEntry.SNOID); err != nil {
				return nil, errors.WithStack(err)
			}
		}

		var assetIdxCount uint32
		if err := binary.Read(dirR, binary.LittleEndian, &assetIdxCount); err != nil {
			return nil, errors.WithStack(err)
		}
		for i := uint32(0); i < assetIdxCount; i++ {
			assetIdxEntry := AssetIdxEntry{}
			if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.ContentHash); err != nil {
				return nil, errors.WithStack(err)
			}
			if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.SNOID); err != nil {
				return nil, errors.WithStack(err)
			}
			if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.FileIndex); err != nil {
				return nil, errors.WithStack(err)
			}
		}

		var namedCount uint32
		if err := binary.Read(dirR, binary.LittleEndian, &namedCount); err != nil {
			return nil, errors.WithStack(err)
		}
		for i := uint32(0); i < namedCount; i++ {
			namedEntry := NamedEntry{}
			if err := binary.Read(dirR, binary.BigEndian, &namedEntry.ContentHash); err != nil {
				return nil, errors.WithStack(err)
			}
			if err := readASCIIZ(dirR, &namedEntry.Filename); err != nil {
				return nil, errors.WithStack(err)
			}
			lookup[namedEntry.Filename] = namedEntry.ContentHash[:]
		}

		//only add the folder if its not empty
		if (assetCount + assetIdxCount + namedCount) > 0 {
			lookup[dirEntry.Filename] = dirEntry.ContentHash[:]
		}

	}

	return &Root{
		lookup: lookup,
	}, nil
}

func readASCIIZ(r io.Reader, dest *string) error {
	buf := bytes.NewBufferString("")
	for {
		var c byte
		if err := binary.Read(r, binary.BigEndian, &c); err != nil {
			return errors.WithStack(err)
		}
		if c == 0 { //ASCIIZ
			break
		}
		buf.WriteByte(c)
	}
	*dest = buf.String()
	return nil
}

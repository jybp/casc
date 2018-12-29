package mndx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"unsafe"

	"github.com/pkg/errors"
)

type header struct {
	Signature             uint32
	HeaderVersion         uint32
	FormatVersion         uint32
	Build1                uint32
	Build2                uint32
	MarInfoOffset         uint32
	MarInfoCount          uint32
	MarInfoSize           uint32
	MndxEntriesOffset     uint32
	MndxEntriesCount      uint32
	MndxEntriesValidCount uint32
	MndxEntriySize        uint32
}

type marInfo struct {
	MarIndex      uint32
	MarDataSize   uint32
	MarUnk0       uint32
	MarDataOffset uint32
	MarUnk1       uint32
}

type Root struct {
	nameToContentHash map[string][]byte
}

func (r *Root) Files() ([]string, error) {
	names := []string{}
	for name := range r.nameToContentHash {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (r *Root) ContentHash(filename string) ([]byte, error) {
	contentHash, ok := r.nameToContentHash[filename]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("%s file name not found", filename))
	}
	return contentHash[:], nil
}

func NewRoot(root []byte) (*Root, error) {
	r := bytes.NewReader(root)
	header := header{}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, errors.WithStack(err)
	}
	if header.Signature != 0x58444E4D /* MNDX */ {
		return nil, errors.WithStack(fmt.Errorf("invalid root signature %x", header.Signature))
	}
	if header.HeaderVersion != 2 && header.FormatVersion != 2 {
		return nil, errors.WithStack(fmt.Errorf("invalid root versions header:%d format:%d", header.HeaderVersion, header.FormatVersion))
	}
	const maxMarFiles = 3
	if header.MarInfoCount > maxMarFiles || header.MarInfoSize != uint32(unsafe.Sizeof(marInfo{})) {
		return nil, errors.WithStack(fmt.Errorf("invalid mar info count:%d size:%d", header.MarInfoCount, header.MarInfoSize))
	}
	if _, err := r.Seek(int64(header.MarInfoOffset), io.SeekStart); err != nil {
		return nil, errors.WithStack(err)
	}

	for i := uint32(0); i < header.MarInfoCount; i++ {
		marInfo := marInfo{}
		if err := binary.Read(r, binary.LittleEndian, &marInfo); err != nil {
			return nil, errors.WithStack(err)
		}

		var marSig uint32
		if err := binary.Read(r, binary.LittleEndian, &marSig); err != nil {
			return nil, errors.WithStack(err)
		}
		if marSig != 0x0052414d {
			return nil, errors.WithStack(fmt.Errorf("invalid mar file signature %x", marSig))
		}

		marFile, err := parseMarFile(r)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	// for (int i = 0; i < MarInfoCount; i++)
	// {
	// 	stream.BaseStream.Position = MarInfoOffset + (MarInfoSize * i);

	// 	MARInfo marInfo = stream.Read<MARInfo>();

	// 	stream.BaseStream.Position = marInfo.MarDataOffset;

	// 	MarFiles[i] = new MARFileNameDB(stream);

	// 	if (stream.BaseStream.Position != marInfo.MarDataOffset + marInfo.MarDataSize)
	// 		throw new Exception("MAR parsing error!");
	// }

	// stream.BaseStream.Position = MndxEntriesOffset;

	// CASC_ROOT_ENTRY_MNDX prevEntry = null;

	// for (int i = 0; i < MndxEntriesTotal; i++)
	// {
	// 	CASC_ROOT_ENTRY_MNDX entry = new CASC_ROOT_ENTRY_MNDX();

	// 	if (prevEntry != null)
	// 		prevEntry.Next = entry;

	// 	prevEntry = entry;
	// 	entry.Flags = stream.ReadInt32();
	// 	entry.MD5 = stream.Read<MD5Hash>();
	// 	entry.FileSize = stream.ReadInt32();
	// 	mndxRootEntries.Add(i, entry);

	// 	worker?.ReportProgress((int)((i + 1) / (float)MndxEntriesTotal * 100));
	// }

	nameToContentHash := map[string][]byte{}
	return &Root{nameToContentHash}, nil
}

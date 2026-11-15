// Package tvfs parses TACT Virtual File System roots (TVFS).
// TVFS is used by newer Blizzard products such as Diablo II Resurrected.
//
// References:
//
//	https://wowdev.wiki/TACT#TVFS
//	CascLib CascRootFile_TVFS.cpp
package tvfs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

const (
	flagIncludeCKey   = 0x01
	flagWriteSupport  = 0x02
	flagPatchSupport  = 0x04
	folderNodeFlag    = 0x80000000
	folderSizeMask    = 0x7FFFFFFF
	nestedSeparator   = ":"
	maxSpanCount      = 224
	headerMinimumSize = 0x26
)

// Span is a portion of a file. Most files are made of a single span.
type Span struct {
	EncodedHash []byte // usually truncated to 9 bytes
	Size        uint32 // decoded size of the span
}

// Root relates filenames to their encoded hashes.
type Root struct {
	nameToSpans map[string][]Span
	names       []string
}

type header struct {
	FormatVersion   uint8
	HeaderSize      uint8
	EKeySize        uint8
	PatchKeySize    uint8
	Flags           uint32
	PathTableOffset uint32
	PathTableSize   uint32
	VfsTableOffset  uint32
	VfsTableSize    uint32
	CftTableOffset  uint32
	CftTableSize    uint32
	MaxDepth        uint16
}

type directory struct {
	h         header
	pathTable []byte
	vfsTable  []byte
	cftTable  []byte
	cftOffLen int
}

// FetchFn returns the decoded content of the given encoded hash.
type FetchFn func(encodedHash []byte) ([]byte, error)

// NewRoot parses the TVFS root located at rootEncodedHash.
// vfsEncodedHashes lists the encoded hashes of every vfs-N entry of the build config.
// Files referencing one of those hashes are nested TVFS directories and are expanded
// using ':' as the separator (i.e. "data:data/global/excel/armor.txt").
func NewRoot(rootEncodedHash []byte, vfsEncodedHashes [][]byte, fetch FetchFn) (*Root, error) {
	r := &Root{nameToSpans: map[string][]Span{}}
	nested := map[string]struct{}{}
	for _, h := range vfsEncodedHashes {
		if len(h) >= 9 {
			nested[string(h[:9])] = struct{}{}
		}
	}
	visited := map[string]struct{}{}
	if err := r.parse(rootEncodedHash, "", nested, visited, fetch); err != nil {
		return nil, err
	}
	r.names = make([]string, 0, len(r.nameToSpans))
	for name := range r.nameToSpans {
		r.names = append(r.names, name)
	}
	sort.Strings(r.names)
	return r, nil
}

// Files returns all the filenames sorted.
func (r *Root) Files() ([]string, error) {
	names := make([]string, len(r.names))
	copy(names, r.names)
	return names, nil
}

// Spans returns the spans of filename.
func (r *Root) Spans(filename string) ([]Span, error) {
	spans, ok := r.nameToSpans[filename]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("%s file name not found", filename))
	}
	return spans, nil
}

// EncodedHashes returns the encoded hash of each span of filename.
func (r *Root) EncodedHashes(filename string) ([][]byte, error) {
	spans, err := r.Spans(filename)
	if err != nil {
		return nil, err
	}
	hashes := make([][]byte, 0, len(spans))
	for _, s := range spans {
		hashes = append(hashes, s.EncodedHash)
	}
	return hashes, nil
}

func (r *Root) parse(encodedHash []byte, prefix string, nested, visited map[string]struct{}, fetch FetchFn) error {
	key := string(encodedHash[:9])
	if _, ok := visited[key]; ok {
		return nil
	}
	visited[key] = struct{}{}
	b, err := fetch(encodedHash)
	if err != nil {
		return errors.Wrapf(err, "cannot fetch TVFS directory %x", encodedHash)
	}
	dir, err := parseDirectory(b)
	if err != nil {
		return err
	}
	type file struct {
		name  string
		spans []Span
	}
	var files []file
	var walk func(table []byte, path []byte) error
	walk = func(table []byte, path []byte) error {
		saved := len(path)
		// continued is true when the previous entry was a name fragment without node value.
		continued := false
		for len(table) > 0 {
			// Optional path separator before the name.
			if table[0] == 0 {
				path = append(path, '/')
				table = table[1:]
			} else if continued && table[0] != 0xFF {
				// Two consecutive name fragments are implicitly separated.
				path = append(path, '/')
			}
			continued = false
			// Name fragment.
			if len(table) > 0 && table[0] != 0xFF {
				l := int(table[0])
				if 1+l > len(table) {
					return errors.WithStack(errors.New("invalid TVFS path table"))
				}
				path = append(path, table[1:1+l]...)
				table = table[1+l:]
			}
			// Optional path separator after the name.
			if len(table) > 0 && table[0] == 0 {
				path = append(path, '/')
				table = table[1:]
			}
			if len(table) == 0 || table[0] != 0xFF {
				// Name continues with the next fragment.
				continued = len(path) > 0 && path[len(path)-1] != '/'
				continue
			}
			if len(table) < 5 {
				return errors.WithStack(errors.New("invalid TVFS node value"))
			}
			value := binary.BigEndian.Uint32(table[1:5])
			table = table[5:]
			if value&folderNodeFlag != 0 {
				size := int(value&folderSizeMask) - 4
				if size < 0 || size > len(table) {
					return errors.WithStack(errors.New("invalid TVFS folder size"))
				}
				if err := walk(table[:size], path); err != nil {
					return err
				}
				table = table[size:]
			} else {
				spans, err := dir.spans(value)
				if err != nil {
					return err
				}
				if spans != nil {
					files = append(files, file{name: string(path), spans: spans})
				}
			}
			path = path[:saved]
		}
		return nil
	}
	if err := walk(dir.pathTable, nil); err != nil {
		return err
	}
	for _, f := range files {
		name := strings.TrimLeft(f.name, "/")
		if len(f.spans) == 1 {
			if _, ok := nested[string(f.spans[0].EncodedHash[:9])]; ok {
				if err := r.parse(f.spans[0].EncodedHash, prefix+name+nestedSeparator, nested, visited, fetch); err != nil {
					return err
				}
				continue
			}
		}
		r.nameToSpans[prefix+name] = f.spans
	}
	return nil
}

func parseDirectory(b []byte) (*directory, error) {
	if len(b) < 4+headerMinimumSize || !bytes.Equal(b[:4], []byte("TVFS")) {
		return nil, errors.WithStack(errors.New("invalid TVFS signature"))
	}
	h := header{}
	if err := binary.Read(bytes.NewReader(b[4:]), binary.BigEndian, &h); err != nil {
		return nil, errors.WithStack(err)
	}
	if h.FormatVersion != 1 {
		return nil, errors.WithStack(fmt.Errorf("unsupported TVFS version %d", h.FormatVersion))
	}
	if h.EKeySize < 9 {
		return nil, errors.WithStack(fmt.Errorf("unsupported TVFS ekey size %d", h.EKeySize))
	}
	sub := func(off, size uint32) ([]byte, error) {
		if uint64(off)+uint64(size) > uint64(len(b)) {
			return nil, errors.WithStack(errors.New("invalid TVFS table bounds"))
		}
		return b[off : off+size], nil
	}
	d := &directory{h: h}
	var err error
	if d.pathTable, err = sub(h.PathTableOffset, h.PathTableSize); err != nil {
		return nil, err
	}
	if d.vfsTable, err = sub(h.VfsTableOffset, h.VfsTableSize); err != nil {
		return nil, err
	}
	if d.cftTable, err = sub(h.CftTableOffset, h.CftTableSize); err != nil {
		return nil, err
	}
	d.cftOffLen = offsetFieldSize(h.CftTableSize)
	return d, nil
}

func offsetFieldSize(tableSize uint32) int {
	switch {
	case tableSize > 0xFFFFFF:
		return 4
	case tableSize > 0xFFFF:
		return 3
	case tableSize > 0xFF:
		return 2
	default:
		return 1
	}
}

// spans reads the VFS entry at offset. Returns nil, nil for entries without content.
func (d *directory) spans(offset uint32) ([]Span, error) {
	if int(offset) >= len(d.vfsTable) {
		return nil, errors.WithStack(errors.New("invalid TVFS vfs offset"))
	}
	p := d.vfsTable[offset:]
	count := int(p[0])
	p = p[1:]
	if count == 0 || count > maxSpanCount {
		return nil, nil
	}
	entrySize := 4 + 4 + d.cftOffLen
	if len(p) < count*entrySize {
		return nil, errors.WithStack(errors.New("invalid TVFS vfs entry"))
	}
	spans := make([]Span, 0, count)
	for i := 0; i < count; i++ {
		e := p[i*entrySize:]
		size := binary.BigEndian.Uint32(e[4:8])
		var cftOff uint32
		for j := 0; j < d.cftOffLen; j++ {
			cftOff = cftOff<<8 | uint32(e[8+j])
		}
		ekeyLen := uint32(d.h.EKeySize)
		if uint64(cftOff)+uint64(ekeyLen) > uint64(len(d.cftTable)) {
			return nil, errors.WithStack(errors.New("invalid TVFS cft offset"))
		}
		ekey := make([]byte, ekeyLen)
		copy(ekey, d.cftTable[cftOff:cftOff+ekeyLen])
		spans = append(spans, Span{EncodedHash: ekey, Size: size})
	}
	return spans, nil
}

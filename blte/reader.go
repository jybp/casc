// Package blte implements reading of LBTE format compressed data
package blte

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
)

type header struct {
	Sig  uint32
	Size uint32
}

type chunkInfo struct {
	Unknown uint16
	Count   uint16
}

type chunkInfoEntry struct {
	Csize    uint32
	USize    uint32
	Checksum [0x10]uint8
}

// NewReader creates a new io.Reader. Reads from the returned Reader read and decompress data from r.
func NewReader(r io.Reader) (io.Reader, error) {
	h := header{}
	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
		return nil, errors.WithStack(err)
	}
	if h.Sig != 0x424c5445 {
		return nil, errors.WithStack(fmt.Errorf("invalid signature %x", h.Sig))
	}
	if h.Size == 0 {
		return createReader(r, 0, 0, [0x10]byte{})
	}
	info := chunkInfo{}
	if err := binary.Read(r, binary.BigEndian, &info); err != nil {
		return nil, errors.WithStack(err)
	}
	entries := []chunkInfoEntry{}
	for i := uint16(0); i < uint16(info.Count); i++ {
		entry := chunkInfoEntry{}
		if err := binary.Read(r, binary.BigEndian, &entry); err != nil {
			return nil, errors.WithStack(err)
		}
		entries = append(entries, entry)
	}
	if h.Size != 12+uint32(info.Count)*24 {
		return nil, errors.WithStack(errors.Errorf("expected header size %d", h.Size))
	}
	return newBlteReader(r, entries), nil
}

func createReader(r io.Reader, usize, csize int, checksum [0x10]byte) (io.Reader, error) {
	var typ uint8
	if err := binary.Read(r, binary.BigEndian, &typ); err != nil {
		return nil, errors.WithStack(err)
	}
	var compressed []byte
	var err error
	if csize <= 0 {
		compressed, err = ioutil.ReadAll(r)
	} else {
		compressed = make([]byte, csize)
		_, err = io.ReadFull(r, compressed)
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if checksum != ([0x10]byte{}) {
		hash := md5.Sum(append([]byte{typ}, compressed...))
		if bytes.Compare(checksum[:], hash[:]) != 0 {
			return nil, errors.WithStack(errors.Errorf("expected checksum %x, got %x", checksum, hash))
		}
	}
	switch typ {
	case 'N':
		if csize != usize {
			return nil, errors.WithStack(
				errors.New("compressed and uncompressed size should be the same"))
		}
		return bytes.NewReader(compressed), nil
	case 'Z':
		zlibReader, err := zlib.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		var uncompressed []byte
		if usize <= 0 {
			uncompressed, err = ioutil.ReadAll(zlibReader)
			uncompressed = uncompressed[:len(uncompressed)-1]
		} else {
			uncompressed = make([]uint8, int(usize))
			_, err = io.ReadFull(zlibReader, uncompressed)
		}
		if errc := zlibReader.Close(); errc != nil {
			return nil, errors.WithStack(errc)
		}
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return bytes.NewReader(uncompressed), nil
	default:
		return nil, errors.WithStack(errors.Errorf("unsuported encoding type %+q", typ))
	}
}

type blteReader struct {
	r       io.Reader
	entries []chunkInfoEntry
	index   int
	next    io.Reader
}

func newBlteReader(r io.Reader, entries []chunkInfoEntry) io.Reader {
	return &blteReader{r: r, entries: entries}
}

func (r *blteReader) step() error {
	if r.index >= len(r.entries) {
		r.next = nil
		return nil
	}
	var err error
	r.next, err = createReader(r.r,
		int(r.entries[r.index].USize),
		int(r.entries[r.index].Csize)-1,
		r.entries[r.index].Checksum)
	r.index++
	return err
}

func (r *blteReader) Read(b []byte) (int, error) {
	for {
		if r.next != nil {
			n, err := r.next.Read(b)
			if err == io.EOF {
				err = nil
				r.next = nil
			}
			return n, errors.WithStack(err)
		}
		if err := r.step(); err != nil {
			return 0, err
		}
		if r.next == nil {
			return 0, io.EOF
		}
	}
}

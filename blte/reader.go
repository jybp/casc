// Package blte implements reading of BLTE format compressed data.
package blte

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash"
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

// NewReader creates a new io.Reader.
// Reads from the returned Reader read and decompress data from r.
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
	return ioutil.NopCloser(newBlteReader(r, entries)), nil
}

// createReader returns a io.ReadCloser that decompress a data chunk.
// Provide the zero values of usize, csize and checksum if the blte file has no header.
func createReader(r io.Reader, usize, csize int, checksum [0x10]byte) (io.Reader, error) {
	if !((csize > 0) == (usize > 0) == (checksum != ([0x10]byte{}))) {
		return nil, errors.WithStack(errors.New("invalid chunk info entry"))
	}
	var typ uint8
	if err := binary.Read(r, binary.BigEndian, &typ); err != nil {
		return nil, errors.WithStack(err)
	}
	if csize > 0 {
		r = io.LimitReader(r, int64(csize))
	}
	if checksum != ([0x10]byte{}) {
		digest := md5.New()
		digest.Write([]byte{typ}) // md5 never returns an error.
		r = &checksumReader{r: r, digest: digest, checksum: checksum}
	}

	switch typ {
	case 'N':
		if csize != usize {
			return nil, errors.WithStack(
				fmt.Errorf("compressed and uncompressed size should be equal %d != %d", csize, usize))
		}
	case 'Z':
		zreader, err := zlib.NewReader(r)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		r = &eofCloser{r: zreader}
	default:
		return nil, errors.WithStack(errors.Errorf("unsuported encoding type %+q", typ))
	}

	if usize > 0 {
		r = &sizeReader{r: r, size: usize}
	}
	return r, nil
}

// blteReader reads blte data consisting of multiple data chunks.
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

// checksumReader checks the provided checksum when reaching io.EOF.
type checksumReader struct {
	r        io.Reader
	digest   hash.Hash
	checksum [0x10]byte

	err error
}

func (c *checksumReader) Read(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	var n int
	n, c.err = c.r.Read(p)
	c.digest.Write(p[0:n]) // MD5 never returns an error.
	if c.err != io.EOF {
		return n, c.err
	}
	hash := c.digest.Sum([]byte{})
	if bytes.Compare(c.checksum[:], hash[:]) != 0 {
		return n, errors.WithStack(errors.Errorf("invalid checksum %x expected %x", hash, c.checksum))
	}
	return n, io.EOF
}

// sizeReader checks the provided size when reaching io.EOF.
type sizeReader struct {
	r    io.Reader
	size int

	actual int
	err    error
}

func (c *sizeReader) Read(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	var n int
	n, c.err = c.r.Read(p)
	c.actual += n
	if c.err != io.EOF {
		return n, c.err
	}
	if c.size != c.actual {
		return n, errors.WithStack(errors.Errorf("invalid size %d expected %d", c.actual, c.size))
	}
	return n, io.EOF
}

// silentCloser closes the provided io.ReadCloser when reaching io.EOF.
type eofCloser struct {
	r   io.ReadCloser
	err error
}

func (c *eofCloser) Read(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	var n int
	n, c.err = c.r.Read(p)
	if c.err != io.EOF {
		return n, c.err
	}
	if cerr := c.r.Close(); cerr != nil {
		return n, errors.WithStack(cerr)
	}
	return n, io.EOF
}

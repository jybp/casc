// BitKnit2 decompression, ported to Go from pybg3's rans.h.
//
// Copyright (C) 2024 Mackenzie Straight.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the “Software”), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// References:
//  https://github.com/eiz/pybg3 (src/rans.h)
//  https://github.com/eiz/libbg3/blob/main/docs/bitknit2.txt
//  https://fgiesen.wordpress.com/2016/03/07/repeated-match-offsets-in-bitknit/

package granny

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

const bitknit2Magic = 0x75B1

// adaptiveModel is a deferred adaptive frequency model: 15 bits frequencies,
// redistributed every 1024 symbols, with a 10 bits lookup table.
type adaptiveModel struct {
	vocab   int
	sums    []uint16 // vocab+1 entries, sums[vocab] == 1<<15
	acc     []uint16
	lookup  [1 << 10]uint16
	counter int
	incr    uint16
	last    uint16
}

func newAdaptiveModel(vocab, minProbable int) *adaptiveModel {
	const total = 1 << 15
	m := &adaptiveModel{
		vocab: vocab,
		sums:  make([]uint16, vocab+1),
		acc:   make([]uint16, vocab),
	}
	m.incr = uint16((total - vocab) / 1024)
	m.last = uint16(1 + total - vocab - int(m.incr)*1024)
	equi := vocab - minProbable
	for i := 0; i < equi; i++ {
		m.sums[i] = uint16((total - minProbable) * i / equi)
	}
	for i := equi; i <= vocab; i++ {
		m.sums[i] = uint16(total - vocab + i)
	}
	for i := range m.acc {
		m.acc[i] = 1
	}
	m.finishUpdate()
	return m
}

func (m *adaptiveModel) finishUpdate() {
	const shift = 15 - 10
	code, sym, next := 0, 0, int(m.sums[1])
	for code < 1<<15 {
		if code < next {
			m.lookup[code>>shift] = uint16(sym)
			code += 1 << shift
		} else {
			sym++
			next = int(m.sums[sym+1])
		}
	}
}

func (m *adaptiveModel) find(code uint32) int {
	sym := int(m.lookup[code>>5])
	for code >= uint32(m.sums[sym+1]) {
		sym++
	}
	return sym
}

func (m *adaptiveModel) observe(sym int) {
	m.acc[sym] += m.incr
	m.counter = (m.counter + 1) % 1024
	if m.counter != 0 {
		return
	}
	m.acc[sym] += m.last
	var sum uint32
	for i := 1; i <= m.vocab; i++ {
		sum += uint32(m.acc[i-1])
		// Unsigned 32 bits arithmetic on purpose: it must wrap like the reference.
		m.sums[i] = uint16(uint32(m.sums[i]) + (sum-uint32(m.sums[i]))/2)
		m.acc[i-1] = 1
	}
	m.finishUpdate()
}

type bitknitDecoder struct {
	src    []byte
	pos    int // byte offset in src, always even
	dst    []byte
	cmd    [4]*adaptiveModel
	cache  [4]*adaptiveModel
	offLen *adaptiveModel

	lruEntries [8]uint32
	lruOrder   uint32
	delta      int
	s1, s2     uint32
}

func (d *bitknitDecoder) pop() (uint32, error) {
	if d.pos+2 > len(d.src) {
		return 0, errors.New("bitknit: unexpected end of bitstream")
	}
	v := binary.LittleEndian.Uint16(d.src[d.pos:])
	d.pos += 2
	return uint32(v), nil
}

func (d *bitknitDecoder) refill(s *uint32) error {
	if *s < 0x10000 {
		w, err := d.pop()
		if err != nil {
			return err
		}
		*s = *s<<16 | w
	}
	return nil
}

func (d *bitknitDecoder) popBits(n uint) (uint32, error) {
	v := d.s1 & (1<<n - 1)
	d.s1 >>= n
	if err := d.refill(&d.s1); err != nil {
		return 0, err
	}
	d.s1, d.s2 = d.s2, d.s1
	return v, nil
}

func (d *bitknitDecoder) popModel(m *adaptiveModel) (int, error) {
	code := d.s1 & 0x7FFF
	sym := m.find(code)
	freq := uint32(m.sums[sym+1] - m.sums[sym])
	d.s1 = (d.s1>>15)*freq + code - uint32(m.sums[sym])
	if err := d.refill(&d.s1); err != nil {
		return 0, err
	}
	m.observe(sym)
	d.s1, d.s2 = d.s2, d.s1
	return sym, nil
}

func (d *bitknitDecoder) initState() error {
	i0, err := d.pop()
	if err != nil {
		return err
	}
	i1, err := d.pop()
	if err != nil {
		return err
	}
	merged := i0<<16 | i1
	split := merged & 15
	merged >>= 4
	if err := d.refill(&merged); err != nil {
		return err
	}
	d.s1 = merged >> split
	if err := d.refill(&d.s1); err != nil {
		return err
	}
	w, err := d.pop()
	if err != nil {
		return err
	}
	d.s2 = merged<<16 | w
	d.s2 &= 1<<(16+split) - 1
	d.s2 |= 1 << (16 + split)
	return nil
}

func (d *bitknitDecoder) lruHit(index uint32) uint32 {
	slot := (d.lruOrder >> (index * 4)) & 15
	mask := uint32(16)<<(index*4) - 1
	rotated := ((d.lruOrder << 4) | slot) & mask
	d.lruOrder = (d.lruOrder &^ mask) | rotated
	return d.lruEntries[slot]
}

func (d *bitknitDecoder) lruInsert(v uint32) {
	d.lruEntries[d.lruOrder>>28] = d.lruEntries[(d.lruOrder>>24)&15]
	d.lruEntries[(d.lruOrder>>24)&15] = v
}

// DecompressBitKnit2 decompresses a BitKnit2 stream into exactly size bytes.
func DecompressBitKnit2(src []byte, size int) ([]byte, error) {
	d := &bitknitDecoder{src: src, dst: make([]byte, size), lruOrder: 0x76543210, delta: 1}
	for i := range d.cmd {
		d.cmd[i] = newAdaptiveModel(300, 36)
		d.cache[i] = newAdaptiveModel(40, 0)
	}
	d.offLen = newAdaptiveModel(21, 0)
	for i := range d.lruEntries {
		d.lruEntries[i] = 1
	}
	if size == 0 {
		return d.dst, nil
	}
	if m, err := d.pop(); err != nil || m != bitknit2Magic {
		return nil, errors.New("bitknit: invalid magic")
	}
	cur := 0
	for cur < size {
		if d.pos >= len(d.src) {
			return nil, errors.New("bitknit: truncated stream")
		}
		var err error
		if cur, err = d.quantum(cur); err != nil {
			return nil, err
		}
	}
	return d.dst, nil
}

func (d *bitknitDecoder) quantum(cur int) (int, error) {
	end := (cur &^ 0xFFFF) + 0x10000
	if end > len(d.dst) {
		end = len(d.dst)
	}
	// A zero word at the start of a quantum means raw data.
	if binary.LittleEndian.Uint16(d.src[d.pos:]) == 0 {
		d.pos += 2
		n := end - cur
		if avail := (len(d.src) - d.pos) &^ 1; avail < n {
			n = avail
		}
		copy(d.dst[cur:cur+n], d.src[d.pos:d.pos+n])
		d.pos += n / 2 * 2
		return cur + n, nil
	}
	if err := d.initState(); err != nil {
		return 0, err
	}
	if cur == 0 {
		v, err := d.popBits(8)
		if err != nil {
			return 0, err
		}
		d.dst[0] = byte(v)
		cur = 1
	}
	for cur < end {
		command, err := d.popModel(d.cmd[cur%4])
		if err != nil {
			return 0, err
		}
		if command < 256 {
			d.dst[cur] = byte(command) + d.dst[cur-d.delta]
			cur++
			continue
		}
		if cur, err = d.copy(command, cur); err != nil {
			return 0, err
		}
	}
	if d.s1 != 0x10000 || d.s2 != 0x10000 {
		return 0, errors.New("bitknit: rANS stream corrupted")
	}
	return cur, nil
}

func (d *bitknitDecoder) copy(command, cur int) (int, error) {
	model := cur % 4
	var length int
	if command < 288 {
		length = command - 254
	} else {
		n := uint(command - 287)
		bits, err := d.popBits(n)
		if err != nil {
			return 0, err
		}
		length = 1<<n + int(bits) + 32
	}
	ref, err := d.popModel(d.cache[model])
	if err != nil {
		return 0, err
	}
	var offset uint32
	if ref < 8 {
		offset = d.lruHit(uint32(ref))
	} else {
		ol, err := d.popModel(d.offLen)
		if err != nil {
			return 0, err
		}
		bits, err := d.popBits(uint(ol % 16))
		if err != nil {
			return 0, err
		}
		if ol >= 16 {
			w, err := d.pop()
			if err != nil {
				return 0, err
			}
			bits = bits<<16 | w
		}
		offset = uint32(32)<<uint(ol) + bits<<5 - 32 + uint32(ref-7)
		d.lruInsert(offset)
	}
	if int(offset) > cur {
		return 0, errors.New("bitknit: invalid copy offset")
	}
	if length > len(d.dst)-cur {
		return 0, errors.New("bitknit: invalid copy length")
	}
	d.delta = int(offset)
	for i := 0; i < length; i++ {
		d.dst[cur] = d.dst[cur-int(offset)]
		cur++
	}
	return cur, nil
}

// Package granny reads Granny 3D (.gr2) files: format 7, 64-bit little endian, the variant
// used by Diablo II Resurrected (.model, .skeleton, .animations). Sections may be stored
// raw or BitKnit2 compressed.
//
// Layout references: libbg3 (MIT, https://github.com/eiz/libbg3), opengr2, LSLib.
package granny

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/pkg/errors"
)

var magic64LE = [16]byte{0xE5, 0x9B, 0x49, 0x5E, 0x6F, 0x63, 0x1F, 0x14, 0x1E, 0x13, 0xEB, 0xA9, 0x90, 0xBE, 0xED, 0xC4}

// Member types of Granny type definitions.
const (
	tEnd                   = 0
	tInline                = 1
	tReference             = 2
	tReferenceToArray      = 3
	tArrayOfReferences     = 4
	tVariantReference      = 5
	tReferenceToVariantArr = 7
	tString                = 8
	tTransform             = 9
	tFloat                 = 10
	tInt8                  = 11
	tUInt8                 = 12
	tBinormalInt8          = 13
	tNormalUInt8           = 14
	tInt16                 = 15
	tUInt16                = 16
	tBinormalInt16         = 17
	tNormalUInt16          = 18
	tInt32                 = 19
	tUInt32                = 20
	tHalf                  = 21
	tEmptyReference        = 22
)

type ptr struct{ section, offset uint32 }

// Member is a field of a Granny struct definition.
type Member struct {
	Type  int
	Name  string
	Ref   []Member // referenced struct, for inline / reference types
	Count int      // fixed array size, 0 for a scalar
}

// Object is a decoded struct: field name to value.
// Values are float64, int64, string, Transform, *Object, []*Object, []float64 or nil.
type Object struct {
	Fields map[string]interface{}
	Order  []string
}

// Get returns a field value or nil.
func (o *Object) Get(name string) interface{} {
	if o == nil {
		return nil
	}
	return o.Fields[name]
}

// Transform is a Granny transform: position, orientation quaternion (x, y, z, w) and scale/shear.
type Transform struct {
	Flags       uint32
	Position    [3]float32
	Orientation [4]float32
	ScaleShear  [9]float32
}

// File is a decompressed Granny file.
type File struct {
	sections [][]byte
	fixups   []map[uint32]ptr
	types    map[ptr][]Member
	objects  map[string]*Object
	Root     *Object
}

// IsGranny reports if b starts with the supported Granny magic.
func IsGranny(b []byte) bool {
	if len(b) < 16 {
		return false
	}
	var m [16]byte
	copy(m[:], b)
	return m == magic64LE
}

// Parse decompresses and decodes a Granny file.
func Parse(b []byte) (*File, error) {
	if !IsGranny(b) {
		return nil, errors.New("granny: unsupported magic (only 64-bit little endian format 7)")
	}
	le := binary.LittleEndian
	if len(b) < 0x68 {
		return nil, errors.New("granny: truncated header")
	}
	if v := le.Uint32(b[0x20:]); v != 7 {
		return nil, errors.Errorf("granny: unsupported format %d", v)
	}
	sectionTable := int(le.Uint32(b[0x2C:])) + 0x20
	numSections := int(le.Uint32(b[0x30:]))
	rootType := ptr{le.Uint32(b[0x34:]), le.Uint32(b[0x38:])}
	rootObj := ptr{le.Uint32(b[0x3C:]), le.Uint32(b[0x40:])}
	if numSections <= 0 || numSections > 64 || sectionTable+numSections*44 > len(b) {
		return nil, errors.New("granny: invalid section table")
	}
	f := &File{
		sections: make([][]byte, numSections),
		fixups:   make([]map[uint32]ptr, numSections),
		types:    map[ptr][]Member{},
		objects:  map[string]*Object{},
	}
	type header struct {
		compression, offset, compressedLen, uncompressedLen uint32
		fixupsOffset, numFixups                             uint32
	}
	headers := make([]header, numSections)
	for i := range headers {
		h := b[sectionTable+i*44:]
		headers[i] = header{
			compression:     le.Uint32(h[0:]),
			offset:          le.Uint32(h[4:]),
			compressedLen:   le.Uint32(h[8:]),
			uncompressedLen: le.Uint32(h[12:]),
			fixupsOffset:    le.Uint32(h[28:]),
			numFixups:       le.Uint32(h[32:]),
		}
	}
	for i, h := range headers {
		if uint64(h.offset)+uint64(h.compressedLen) > uint64(len(b)) {
			return nil, errors.Errorf("granny: section %d out of bounds", i)
		}
		raw := b[h.offset : h.offset+h.compressedLen]
		switch h.compression {
		case 0:
			data := make([]byte, h.uncompressedLen)
			copy(data, raw)
			f.sections[i] = data
		case 4:
			data, err := DecompressBitKnit2(raw, int(h.uncompressedLen))
			if err != nil {
				return nil, errors.Wrapf(err, "section %d", i)
			}
			f.sections[i] = data
		default:
			return nil, errors.Errorf("granny: unsupported compression %d", h.compression)
		}
	}
	for i, h := range headers {
		size := int(h.numFixups) * 12
		var fx []byte
		if h.compression == 4 && size > 0 {
			if int(h.fixupsOffset)+4 > len(b) {
				return nil, errors.New("granny: invalid fixups offset")
			}
			n := int(le.Uint32(b[h.fixupsOffset:]))
			start := int(h.fixupsOffset) + 4
			if start+n > len(b) {
				return nil, errors.New("granny: invalid fixups size")
			}
			var err error
			if fx, err = DecompressBitKnit2(b[start:start+n], size); err != nil {
				return nil, errors.Wrapf(err, "section %d fixups", i)
			}
		} else {
			if int(h.fixupsOffset)+size > len(b) {
				return nil, errors.New("granny: invalid fixups")
			}
			fx = b[h.fixupsOffset : int(h.fixupsOffset)+size]
		}
		m := make(map[uint32]ptr, h.numFixups)
		for k := 0; k+12 <= len(fx); k += 12 {
			m[le.Uint32(fx[k:])] = ptr{le.Uint32(fx[k+4:]), le.Uint32(fx[k+8:])}
		}
		f.fixups[i] = m
	}
	def, err := f.typeAt(rootType)
	if err != nil {
		return nil, err
	}
	root, err := f.object(rootObj, def)
	if err != nil {
		return nil, err
	}
	f.Root = root
	return f, nil
}

// pointer reads the pointer stored at p (resolved through the fixups); ok is false for null.
func (f *File) pointer(p ptr) (ptr, bool) {
	if int(p.section) >= len(f.fixups) {
		return ptr{}, false
	}
	t, ok := f.fixups[p.section][p.offset]
	return t, ok
}

func (f *File) bytes(p ptr, n int) ([]byte, error) {
	if int(p.section) >= len(f.sections) {
		return nil, errors.New("granny: invalid section")
	}
	s := f.sections[p.section]
	if int(p.offset)+n > len(s) || n < 0 {
		return nil, errors.New("granny: read out of bounds")
	}
	return s[p.offset : int(p.offset)+n], nil
}

func (f *File) cstring(p ptr) (string, error) {
	if int(p.section) >= len(f.sections) {
		return "", errors.New("granny: invalid section")
	}
	s := f.sections[p.section]
	end := int(p.offset)
	for end < len(s) && s[end] != 0 {
		end++
	}
	if int(p.offset) > len(s) {
		return "", errors.New("granny: string out of bounds")
	}
	return string(s[p.offset:end]), nil
}

// typeAt decodes the type definition (array of 44 bytes members ending with type 0).
func (f *File) typeAt(p ptr) ([]Member, error) {
	if def, ok := f.types[p]; ok {
		return def, nil
	}
	f.types[p] = nil // break cycles; filled below
	var def []Member
	for i := 0; ; i++ {
		mp := ptr{p.section, p.offset + uint32(i*44)}
		b, err := f.bytes(mp, 44)
		if err != nil {
			return nil, err
		}
		t := int(binary.LittleEndian.Uint32(b))
		if t == tEnd {
			break
		}
		if i > 512 {
			return nil, errors.New("granny: type definition too long")
		}
		m := Member{Type: t, Count: int(binary.LittleEndian.Uint32(b[20:]))}
		if np, ok := f.pointer(ptr{mp.section, mp.offset + 4}); ok {
			if m.Name, err = f.cstring(np); err != nil {
				return nil, err
			}
		}
		if rp, ok := f.pointer(ptr{mp.section, mp.offset + 12}); ok {
			ref, err := f.typeAt(rp)
			if err != nil {
				return nil, err
			}
			m.Ref = ref // nil while a recursive type is being decoded
			f.lazyRef(&m, rp)
		}
		def = append(def, m)
	}
	f.types[p] = def
	return def, nil
}

// lazyRefs remember members whose referenced type was still being decoded.
func (f *File) lazyRef(m *Member, p ptr) {
	if m.Ref == nil {
		m.Ref = []Member{{Type: -1, Name: fmt.Sprintf("%d:%d", p.section, p.offset)}}
	}
}

func (f *File) resolve(def []Member) []Member {
	if len(def) == 1 && def[0].Type == -1 {
		var p ptr
		fmt.Sscanf(def[0].Name, "%d:%d", &p.section, &p.offset)
		return f.types[p]
	}
	return def
}

func memberSize(f *File, m Member) int {
	n := m.Count
	if n == 0 {
		n = 1
	}
	var s int
	switch m.Type {
	case tInline:
		s = structSize(f, f.resolve(m.Ref))
	case tReference, tString, tEmptyReference:
		s = 8
	case tReferenceToArray, tArrayOfReferences:
		s = 12
	case tVariantReference:
		s = 16
	case tReferenceToVariantArr:
		s = 20
	case tTransform:
		s = 68
	case tFloat, tInt32, tUInt32:
		s = 4
	case tInt8, tUInt8, tBinormalInt8, tNormalUInt8:
		s = 1
	case tInt16, tUInt16, tBinormalInt16, tNormalUInt16, tHalf:
		s = 2
	}
	return s * n
}

func structSize(f *File, def []Member) int {
	s := 0
	for _, m := range def {
		s += memberSize(f, m)
	}
	return s
}

func (f *File) object(p ptr, def []Member) (*Object, error) {
	def = f.resolve(def)
	key := fmt.Sprintf("%d:%d:%p", p.section, p.offset, def)
	if len(def) > 0 {
		key = fmt.Sprintf("%d:%d:%p", p.section, p.offset, &def[0])
	}
	if o, ok := f.objects[key]; ok {
		return o, nil
	}
	o := &Object{Fields: map[string]interface{}{}}
	f.objects[key] = o
	off := p.offset
	for _, m := range def {
		v, err := f.value(ptr{p.section, off}, m)
		if err != nil {
			return nil, errors.Wrapf(err, "field %s", m.Name)
		}
		o.Fields[m.Name] = v
		o.Order = append(o.Order, m.Name)
		off += uint32(memberSize(f, m))
	}
	return o, nil
}

func (f *File) objects0(p ptr, def []Member, n int) ([]*Object, error) {
	def = f.resolve(def)
	size := uint32(structSize(f, def))
	out := make([]*Object, 0, n)
	for i := 0; i < n; i++ {
		o, err := f.object(ptr{p.section, p.offset + uint32(i)*size}, def)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func (f *File) value(p ptr, m Member) (interface{}, error) {
	le := binary.LittleEndian
	count := m.Count
	switch m.Type {
	case tInline:
		if count > 0 {
			return f.objects0(p, m.Ref, count)
		}
		return f.object(p, m.Ref)
	case tReference:
		t, ok := f.pointer(p)
		if !ok || m.Ref == nil {
			return nil, nil
		}
		return f.object(t, m.Ref)
	case tReferenceToArray:
		b, err := f.bytes(p, 4)
		if err != nil {
			return nil, err
		}
		n := int(int32(le.Uint32(b)))
		t, ok := f.pointer(ptr{p.section, p.offset + 4})
		def := f.resolve(m.Ref)
		if !ok || n <= 0 {
			return &VariantArray{Type: def, file: f}, nil
		}
		size := structSize(f, def)
		raw, err := f.bytes(t, size*n)
		if err != nil {
			return nil, err
		}
		return &VariantArray{Type: def, Count: n, Stride: size, Data: raw, file: f, at: t}, nil
	case tArrayOfReferences:
		b, err := f.bytes(p, 4)
		if err != nil {
			return nil, err
		}
		n := int(int32(le.Uint32(b)))
		t, ok := f.pointer(ptr{p.section, p.offset + 4})
		if !ok || n <= 0 {
			return []*Object{}, nil
		}
		out := make([]*Object, 0, n)
		for i := 0; i < n; i++ {
			e, ok := f.pointer(ptr{t.section, t.offset + uint32(i*8)})
			if !ok {
				out = append(out, nil)
				continue
			}
			o, err := f.object(e, m.Ref)
			if err != nil {
				return nil, err
			}
			out = append(out, o)
		}
		return out, nil
	case tVariantReference:
		tp, ok1 := f.pointer(p)
		op, ok2 := f.pointer(ptr{p.section, p.offset + 8})
		if !ok1 || !ok2 {
			return nil, nil
		}
		def, err := f.typeAt(tp)
		if err != nil {
			return nil, err
		}
		return f.object(op, def)
	case tReferenceToVariantArr:
		tp, ok1 := f.pointer(p)
		b, err := f.bytes(ptr{p.section, p.offset + 8}, 4)
		if err != nil {
			return nil, err
		}
		n := int(int32(le.Uint32(b)))
		ip, ok2 := f.pointer(ptr{p.section, p.offset + 12})
		if !ok1 || !ok2 || n <= 0 {
			return &VariantArray{}, nil
		}
		def, err := f.typeAt(tp)
		if err != nil {
			return nil, err
		}
		size := structSize(f, def)
		raw, err := f.bytes(ip, size*n)
		if err != nil {
			return nil, err
		}
		return &VariantArray{Type: def, Count: n, Stride: size, Data: raw, file: f, at: ip}, nil
	case tString:
		t, ok := f.pointer(p)
		if !ok {
			return "", nil
		}
		return f.cstring(t)
	case tTransform:
		b, err := f.bytes(p, 68)
		if err != nil {
			return nil, err
		}
		var t Transform
		t.Flags = le.Uint32(b)
		for i := 0; i < 3; i++ {
			t.Position[i] = math.Float32frombits(le.Uint32(b[4+4*i:]))
		}
		for i := 0; i < 4; i++ {
			t.Orientation[i] = math.Float32frombits(le.Uint32(b[16+4*i:]))
		}
		for i := 0; i < 9; i++ {
			t.ScaleShear[i] = math.Float32frombits(le.Uint32(b[32+4*i:]))
		}
		return t, nil
	case tEmptyReference:
		return nil, nil
	}
	// Numbers.
	n := count
	if n == 0 {
		n = 1
	}
	size := memberSize(f, Member{Type: m.Type})
	b, err := f.bytes(p, size*n)
	if err != nil {
		return nil, err
	}
	vals := make([]float64, n)
	for i := 0; i < n; i++ {
		vals[i] = scalar(m.Type, b[i*size:])
	}
	if count == 0 {
		return vals[0], nil
	}
	return vals, nil
}

func scalar(t int, b []byte) float64 {
	le := binary.LittleEndian
	switch t {
	case tFloat:
		return float64(math.Float32frombits(le.Uint32(b)))
	case tInt8:
		return float64(int8(b[0]))
	case tUInt8:
		return float64(b[0])
	case tBinormalInt8:
		return float64(int8(b[0])) / 127
	case tNormalUInt8:
		return float64(b[0]) / 255
	case tInt16:
		return float64(int16(le.Uint16(b)))
	case tUInt16:
		return float64(le.Uint16(b))
	case tBinormalInt16:
		return float64(int16(le.Uint16(b))) / 32767
	case tNormalUInt16:
		return float64(le.Uint16(b)) / 65535
	case tInt32:
		return float64(int32(le.Uint32(b)))
	case tUInt32:
		return float64(le.Uint32(b))
	case tHalf:
		return float64(halfToFloat(le.Uint16(b)))
	}
	return 0
}

func halfToFloat(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := int((h >> 10) & 0x1f)
	mant := uint32(h & 0x3ff)
	switch {
	case exp == 0:
		if mant == 0 {
			return math.Float32frombits(sign)
		}
		// Subnormal.
		f := float32(mant) / 1024 / 16384
		if sign != 0 {
			return -f
		}
		return f
	case exp == 31:
		return math.Float32frombits(sign | 0x7f800000 | mant<<13)
	}
	return math.Float32frombits(sign | uint32(exp-15+127)<<23 | mant<<13)
}

// VariantArray is an array of structs (i.e. vertices) kept raw for speed.
type VariantArray struct {
	Type   []Member
	Count  int
	Stride int
	Data   []byte
	file   *File
	at     ptr
}

// Field returns the byte offset and member of a field of the element type.
func (v *VariantArray) Field(name string) (int, Member, bool) {
	off := 0
	for _, m := range v.Type {
		if m.Name == name {
			return off, m, true
		}
		off += memberSize(v.file, m)
	}
	return 0, Member{}, false
}

// Floats decodes numeric field name of element i (fixed arrays return Count values).
func (v *VariantArray) Floats(i int, off int, m Member) []float64 {
	n := m.Count
	if n == 0 {
		n = 1
	}
	size := memberSize(v.file, Member{Type: m.Type})
	out := make([]float64, n)
	base := i*v.Stride + off
	for k := 0; k < n; k++ {
		out[k] = scalar(m.Type, v.Data[base+k*size:])
	}
	return out
}

// Objects decodes every element as an Object (slow; use Floats for vertices).
func (v *VariantArray) Objects() ([]*Object, error) {
	if v == nil || v.file == nil || v.Count == 0 {
		return nil, nil
	}
	return v.file.objects0(v.at, v.Type, v.Count)
}

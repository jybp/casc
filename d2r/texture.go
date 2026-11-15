// Package d2r converts Diablo II Resurrected asset formats to standard formats.
package d2r

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"path"
	"strings"

	"github.com/pkg/errors"
)

// TextureMagic is the first 4 bytes of a .texture file.
const TextureMagic = 0x2845443C

// Format is the pixel format of a texture.
type Format uint16

// Known formats. Values are the ones found inside .texture headers.
// The mapping was verified by decoding textures of every kind (albedo, normal, orm, masks, ...).
const (
	FormatRGBA8   Format = 31 // 32 bits per pixel, uncompressed (gradients, noise)
	FormatBC1     Format = 57 // DXT1, linear (flow maps, ...)
	FormatBC1SRGB Format = 58 // DXT1, sRGB (albedo without alpha)
	FormatBC3     Format = 61 // DXT5, linear (_nrm normal maps stored as DXT5nm, _orm, ...)
	FormatBC3SRGB Format = 62 // DXT5, sRGB (_alb albedo with alpha)
	FormatBC4     Format = 63 // single channel (masks, alpha, minimap tiles, ...)
	FormatBC5     Format = 65 // two channels (vfx distortion maps)
)

type formatInfo struct {
	name       string
	blockBytes int // bytes per 4x4 block, 0 if uncompressed
	pixelBytes int // bytes per pixel when uncompressed
	fourCC     string
	decode     func(block []byte, out *[16][4]uint8)
}

var formats = map[Format]formatInfo{
	FormatRGBA8:   {"RGBA8", 0, 4, "", nil},
	FormatBC1:     {"BC1", 8, 0, "DXT1", decodeBC1},
	FormatBC1SRGB: {"BC1_SRGB", 8, 0, "DXT1", decodeBC1},
	FormatBC3:     {"BC3", 16, 0, "DXT5", decodeBC3},
	FormatBC3SRGB: {"BC3_SRGB", 16, 0, "DXT5", decodeBC3},
	FormatBC4:     {"BC4", 8, 0, "ATI1", decodeBC4Gray},
	FormatBC5:     {"BC5", 16, 0, "ATI2", decodeBC5},
}

func (f Format) String() string {
	if info, ok := formats[f]; ok {
		return info.name
	}
	return fmt.Sprintf("Format(%d)", uint16(f))
}

// Mip is a mipmap level.
type Mip struct {
	Width, Height int
	Data          []byte
}

// Texture is a decoded .texture header with its mipmaps (largest first).
type Texture struct {
	Format Format
	Width  int
	Height int
	Mips   []Mip
}

type textureHeader struct {
	Magic     uint32
	Format    uint16
	Unk0      uint8
	MipCount0 uint8
	Width     uint32
	Height    uint32
	Depth     uint32
	Unk1      uint32
	Unk2      uint32
	MipCount  uint32
	Unk3      uint32
}

// IsTexture reports if b starts with the .texture signature.
func IsTexture(b []byte) bool {
	return len(b) >= 4 && binary.LittleEndian.Uint32(b) == TextureMagic
}

// ParseTexture parses a Diablo II Resurrected .texture file.
func ParseTexture(b []byte) (*Texture, error) {
	h := textureHeader{}
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &h); err != nil {
		return nil, errors.WithStack(err)
	}
	if h.Magic != TextureMagic {
		return nil, errors.New("invalid texture signature")
	}
	info, ok := formats[Format(h.Format)]
	if !ok {
		return nil, errors.Errorf("unsupported texture format %d", h.Format)
	}
	if h.Depth > 1 || h.Width == 0 || h.Height == 0 || h.MipCount == 0 || h.MipCount > 16 {
		return nil, errors.Errorf("unsupported texture layout %dx%dx%d (%d mips)", h.Width, h.Height, h.Depth, h.MipCount)
	}
	t := &Texture{Format: Format(h.Format), Width: int(h.Width), Height: int(h.Height)}
	const tableOffset = 0x24 // mip table: {u32 size; u32 offset relative to this field}
	if len(b) < tableOffset+int(h.MipCount)*8 {
		return nil, errors.New("truncated texture mip table")
	}
	w, hh := t.Width, t.Height
	for i := 0; i < int(h.MipCount); i++ {
		size := int(binary.LittleEndian.Uint32(b[tableOffset+i*8:]))
		offField := tableOffset + i*8 + 4
		offset := offField + int(binary.LittleEndian.Uint32(b[offField:]))
		if offset < 0 || size < 0 || offset+size > len(b) {
			return nil, errors.Errorf("invalid mip %d bounds", i)
		}
		if expected := dataSize(info, w, hh); expected != size {
			return nil, errors.Errorf("mip %d: %s %dx%d expected %d bytes got %d", i, t.Format, w, hh, expected, size)
		}
		t.Mips = append(t.Mips, Mip{Width: w, Height: hh, Data: b[offset : offset+size]})
		w, hh = max1(w/2), max1(hh/2)
	}
	return t, nil
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func dataSize(info formatInfo, w, h int) int {
	if info.blockBytes == 0 {
		return w * h * info.pixelBytes
	}
	return ((w + 3) / 4) * ((h + 3) / 4) * info.blockBytes
}

// WriteDDS writes the texture with all its mipmaps as a .dds file. No data is lost.
func (t *Texture) WriteDDS(w io.Writer) error {
	info := formats[t.Format]
	const (
		ddsdCaps        = 0x1
		ddsdHeight      = 0x2
		ddsdWidth       = 0x4
		ddsdPitch       = 0x8
		ddsdPixelFormat = 0x1000
		ddsdMipMapCount = 0x20000
		ddsdLinearSize  = 0x80000
		ddpfFourCC      = 0x4
		ddpfRGB         = 0x40
		ddpfAlphaPixels = 0x1
		capsComplex     = 0x8
		capsTexture     = 0x1000
		capsMipMap      = 0x400000
	)
	flags := uint32(ddsdCaps | ddsdHeight | ddsdWidth | ddsdPixelFormat)
	caps := uint32(capsTexture)
	if len(t.Mips) > 1 {
		flags |= ddsdMipMapCount
		caps |= capsComplex | capsMipMap
	}
	pitch := uint32(dataSize(info, t.Width, t.Height))
	if info.blockBytes == 0 {
		flags |= ddsdPitch
		pitch = uint32(t.Width * info.pixelBytes)
	} else {
		flags |= ddsdLinearSize
	}
	hdr := make([]uint32, 31)
	hdr[0] = 124
	hdr[1] = flags
	hdr[2] = uint32(t.Height)
	hdr[3] = uint32(t.Width)
	hdr[4] = pitch
	hdr[6] = uint32(len(t.Mips))
	// pixel format at index 18
	hdr[18] = 32
	switch {
	case info.blockBytes == 0:
		hdr[19] = ddpfRGB | ddpfAlphaPixels
		hdr[21] = 32
		hdr[22], hdr[23], hdr[24], hdr[25] = 0x000000ff, 0x0000ff00, 0x00ff0000, 0xff000000
	default:
		hdr[19] = ddpfFourCC
		hdr[20] = binary.LittleEndian.Uint32([]byte(info.fourCC))
	}
	hdr[26] = caps
	if _, err := w.Write([]byte("DDS ")); err != nil {
		return errors.WithStack(err)
	}
	if err := binary.Write(w, binary.LittleEndian, hdr); err != nil {
		return errors.WithStack(err)
	}
	for _, m := range t.Mips {
		if _, err := w.Write(m.Data); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

// Image decodes the largest mipmap.
// When normalMap is true, a DXT5nm normal map (X in alpha, Y in green) is
// converted to a regular opaque RGB normal map with Z reconstructed.
func (t *Texture) Image(normalMap bool) (*image.NRGBA, error) {
	return t.MipImage(0, normalMap)
}

// SmallestMipAtLeast returns the index of the smallest mip whose width and height are >= size.
func (t *Texture) SmallestMipAtLeast(size int) int {
	best := 0
	for i, m := range t.Mips {
		if m.Width >= size && m.Height >= size {
			best = i
		}
	}
	return best
}

// MipImage decodes the given mip level. See Image for normalMap.
func (t *Texture) MipImage(level int, normalMap bool) (*image.NRGBA, error) {
	if level < 0 || level >= len(t.Mips) {
		return nil, errors.Errorf("invalid mip level %d", level)
	}
	m := t.Mips[level]
	img := image.NewNRGBA(image.Rect(0, 0, m.Width, m.Height))
	info := formats[t.Format]
	if info.decode == nil {
		copy(img.Pix, m.Data)
		return img, nil
	}
	var px [16][4]uint8
	bw := (m.Width + 3) / 4
	bh := (m.Height + 3) / 4
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			off := (by*bw + bx) * info.blockBytes
			info.decode(m.Data[off:off+info.blockBytes], &px)
			for i := 0; i < 16; i++ {
				x, y := bx*4+i%4, by*4+i/4
				if x >= m.Width || y >= m.Height {
					continue
				}
				o := img.PixOffset(x, y)
				copy(img.Pix[o:o+4], px[i][:])
			}
		}
	}
	if normalMap && (t.Format == FormatBC3 || t.Format == FormatBC3SRGB) {
		for o := 0; o < len(img.Pix); o += 4 {
			nx, ny := img.Pix[o+3], img.Pix[o+1]
			img.Pix[o+0], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = nx, ny, normalZ(nx, ny), 255
		}
	}
	return img, nil
}

// WritePNG writes the largest mipmap as a PNG. See Image for normalMap.
func (t *Texture) WritePNG(w io.Writer, normalMap bool) error {
	img, err := t.Image(normalMap)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	return errors.WithStack(enc.Encode(w, img))
}

// IsNormalMapName reports if a texture filename follows the normal map naming convention.
func IsNormalMapName(name string) bool {
	name = strings.ToLower(path.Base(name))
	return strings.Contains(name, "_nrm")
}

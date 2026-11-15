package d2r

import (
	"image"
	"image/color"

	"github.com/pkg/errors"
)

// Palette is a 256 colors palette. Index 0 is transparent.
type Palette [256]color.NRGBA

// ParsePalette parses a pal.dat file (256 BGR triplets).
func ParsePalette(b []byte) (*Palette, error) {
	if len(b) < 768 {
		return nil, errors.Errorf("invalid palette size %d", len(b))
	}
	p := &Palette{}
	for i := 0; i < 256; i++ {
		p[i] = color.NRGBA{R: b[i*3+2], G: b[i*3+1], B: b[i*3], A: 255}
	}
	p[0].A = 0
	return p, nil
}

// Indexed is an 8 bits image whose pixels are palette indices.
// Index 0 is transparent.
type Indexed struct {
	Width, Height int
	Pix           []byte
}

func newIndexed(w, h int) *Indexed {
	return &Indexed{Width: w, Height: h, Pix: make([]byte, w*h)}
}

// Image converts the indexed image using p.
func (img *Indexed) Image(p *Palette) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, img.Width, img.Height))
	for i, v := range img.Pix {
		if v == 0 {
			continue
		}
		c := p[v]
		copy(out.Pix[i*4:i*4+4], []byte{c.R, c.G, c.B, 255})
	}
	return out
}

// bitStream reads bits LSB first.
type bitStream struct {
	b   []byte
	pos int // in bits
	end int // in bits
}

func newBitStream(b []byte, start, end int) *bitStream {
	if end > len(b)*8 {
		end = len(b) * 8
	}
	return &bitStream{b: b, pos: start, end: end}
}

func (s *bitStream) bit() int {
	if s.pos >= s.end {
		s.pos++
		return 0
	}
	v := int(s.b[s.pos>>3]>>uint(s.pos&7)) & 1
	s.pos++
	return v
}

func (s *bitStream) bits(n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v |= s.bit() << uint(i)
	}
	return v
}

func (s *bitStream) signedBits(n int) int {
	v := s.bits(n)
	if n > 0 && v&(1<<uint(n-1)) != 0 {
		v -= 1 << uint(n)
	}
	return v
}

func (s *bitStream) overflow() bool {
	return s.pos > s.end
}

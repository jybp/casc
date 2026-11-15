package d2r

import (
	"encoding/binary"
	"image"
	"math"

	"github.com/pkg/errors"
)

// HDSpriteMagic is the signature of the remastered 2D sprites (.sprite).
const HDSpriteMagic = "SpA1"

// HDSprite is a remastered 2D sprite: a horizontal strip of frames.
type HDSprite struct {
	Version    int // 31 RGBA8, 50 RGBA half float, 61 BC3
	FrameWidth int
	Frames     int
	// Image is the whole strip. Frame i starts at i*Stride() and is FrameWidth wide.
	Image *image.NRGBA
}

// Stride returns the distance in pixels between the start of two frames in the strip.
func (s *HDSprite) Stride() int {
	if s.Frames <= 0 {
		return s.Image.Rect.Dx()
	}
	return s.Image.Rect.Dx() / s.Frames
}

// Frame returns frame i cropped to FrameWidth.
func (s *HDSprite) Frame(i int) *image.NRGBA {
	x := i * s.Stride()
	w := s.FrameWidth
	if w <= 0 || x+w > s.Image.Rect.Dx() {
		w = s.Stride()
	}
	return s.Image.SubImage(image.Rect(x, 0, x+w, s.Image.Rect.Dy())).(*image.NRGBA)
}

// IsHDSprite reports if b starts with the SpA1 signature.
func IsHDSprite(b []byte) bool {
	return len(b) >= 4 && string(b[:4]) == HDSpriteMagic
}

// ParseHDSprite decodes a .sprite file.
func ParseHDSprite(b []byte) (*HDSprite, error) {
	if !IsHDSprite(b) || len(b) < 40 {
		return nil, errors.New("invalid sprite signature")
	}
	le := binary.LittleEndian
	s := &HDSprite{
		Version:    int(le.Uint16(b[4:])),
		FrameWidth: int(le.Uint16(b[6:])),
		Frames:     int(le.Uint32(b[20:])),
	}
	w, h := int(le.Uint32(b[8:])), int(le.Uint32(b[12:]))
	if w <= 0 || h <= 0 || w*h > 1<<26 {
		return nil, errors.Errorf("invalid sprite size %dx%d", w, h)
	}
	data := b[40:]
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	switch s.Version {
	case 31:
		if len(data) < w*h*4 {
			return nil, errors.New("truncated sprite")
		}
		copy(img.Pix, data[:w*h*4])
	case 50:
		if len(data) < w*h*8 {
			return nil, errors.New("truncated sprite")
		}
		for i := 0; i < w*h*4; i++ {
			v := float64(halfFloat(le.Uint16(data[2*i:])))
			if i%4 != 3 {
				// Linear to sRGB for display.
				if v <= 0.0031308 {
					v *= 12.92
				} else {
					v = 1.055*math.Pow(v, 1/2.4) - 0.055
				}
			}
			img.Pix[i] = uint8(math.Max(0, math.Min(255, math.Round(v*255))))
		}
	case 61:
		bw, bh := (w+3)/4, (h+3)/4
		if len(data) < bw*bh*16 {
			return nil, errors.New("truncated sprite")
		}
		var px [16][4]uint8
		for by := 0; by < bh; by++ {
			for bx := 0; bx < bw; bx++ {
				decodeBC3(data[(by*bw+bx)*16:], &px)
				for i := 0; i < 16; i++ {
					x, y := bx*4+i%4, by*4+i/4
					if x < w && y < h {
						copy(img.Pix[img.PixOffset(x, y):], px[i][:])
					}
				}
			}
		}
	default:
		return nil, errors.Errorf("unsupported sprite version %d", s.Version)
	}
	s.Image = img
	return s, nil
}

func halfFloat(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := int((h >> 10) & 0x1f)
	mant := uint32(h & 0x3ff)
	switch exp {
	case 0:
		f := float32(mant) / (1 << 24)
		if sign != 0 {
			return -f
		}
		return f
	case 31:
		return math.Float32frombits(sign | 0x7f800000 | mant<<13)
	}
	return math.Float32frombits(sign | uint32(exp-15+127)<<23 | mant<<13)
}

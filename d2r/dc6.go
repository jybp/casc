package d2r

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

// Frame is a decoded sprite frame. OffsetX/OffsetY are the position of the
// top-left pixel relative to the sprite anchor (the feet of a unit).
type Frame struct {
	*Indexed
	OffsetX, OffsetY int
}

// Sprite is a set of frames per direction.
type Sprite struct {
	Directions [][]Frame
}

// ParseDC6 parses a .dc6 sprite (uncompressed RLE sprites: UI, items, inventory).
func ParseDC6(b []byte) (*Sprite, error) {
	if len(b) < 24 {
		return nil, errors.New("dc6 too small")
	}
	le := binary.LittleEndian
	if v := le.Uint32(b); v != 6 {
		return nil, errors.Errorf("unsupported dc6 version %d", v)
	}
	dirs := int(le.Uint32(b[16:]))
	frames := int(le.Uint32(b[20:]))
	if dirs <= 0 || frames <= 0 || dirs*frames > 1<<16 || 24+4*dirs*frames > len(b) {
		return nil, errors.Errorf("invalid dc6 layout %dx%d", dirs, frames)
	}
	s := &Sprite{Directions: make([][]Frame, dirs)}
	for d := 0; d < dirs; d++ {
		for f := 0; f < frames; f++ {
			off := int(le.Uint32(b[24+4*(d*frames+f):]))
			if off+32 > len(b) {
				return nil, errors.New("invalid dc6 frame offset")
			}
			h := b[off:]
			flip := int32(le.Uint32(h[0:]))
			w := int(int32(le.Uint32(h[4:])))
			hh := int(int32(le.Uint32(h[8:])))
			ox := int(int32(le.Uint32(h[12:])))
			oy := int(int32(le.Uint32(h[16:])))
			length := int(le.Uint32(h[28:]))
			if w < 0 || hh < 0 || w*hh > 1<<24 || off+32+length > len(b) {
				return nil, errors.New("invalid dc6 frame header")
			}
			img := newIndexed(w, hh)
			data := b[off+32 : off+32+length]
			x, y, dy := 0, hh-1, -1
			if flip != 0 {
				y, dy = 0, 1
			}
			for i := 0; i < len(data) && y >= 0 && y < hh; {
				c := data[i]
				i++
				switch {
				case c == 0x80:
					x = 0
					y += dy
				case c&0x80 != 0:
					x += int(c & 0x7f)
				default:
					for n := 0; n < int(c) && i < len(data); n++ {
						if x < w {
							img.Pix[y*w+x] = data[i]
						}
						x++
						i++
					}
				}
			}
			s.Directions[d] = append(s.Directions[d], Frame{Indexed: img, OffsetX: ox, OffsetY: oy - hh + 1})
		}
	}
	return s, nil
}

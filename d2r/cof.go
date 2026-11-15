package d2r

import (
	"image"

	"github.com/pkg/errors"
)

// Components are the layers a unit is made of, in COF index order.
var Components = []string{"hd", "tr", "lg", "ra", "la", "rh", "lh", "sh",
	"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8"}

// COFLayer is a layer of a composite animation.
type COFLayer struct {
	Component   int    // index in Components
	Shadow      bool   // casts a shadow
	Transparent bool   // drawn with a blend effect
	DrawEffect  int    // blend mode when transparent
	WeaponClass string // i.e. "hth", "1hs", "bow"
}

// COF describes how DCC layers compose an animation (one mode and weapon class of a unit).
type COF struct {
	Directions     int
	FramesPerDir   int
	Layers         []COFLayer
	Box            image.Rectangle // sprite box relative to the anchor
	AnimationSpeed int             // 256 = 25 frames per second
	// Events holds a value per frame: 0 none, 1 attack/hit, 2 missile, 3 sound, 4 skill.
	Events []int
	// order[(dir*FramesPerDir+frame)*len(Layers)+i] is a component index, drawn back to front.
	order []int
}

// ParseCOF parses a .cof file.
func ParseCOF(b []byte) (*COF, error) {
	if len(b) < 28 {
		return nil, errors.New("cof too small")
	}
	c := &COF{
		Directions:   int(b[2]),
		FramesPerDir: int(b[1]),
	}
	nLayers := int(b[0])
	i32 := func(o int) int {
		return int(int32(uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24))
	}
	c.Box = image.Rect(i32(8), i32(16), i32(12)+1, i32(20)+1)
	c.AnimationSpeed = int(b[24]) | int(b[25])<<8
	o := 28
	need := o + nLayers*9 + c.FramesPerDir + c.Directions*c.FramesPerDir*nLayers
	if nLayers == 0 || len(b) < need {
		return nil, errors.Errorf("invalid cof size %d (expected %d)", len(b), need)
	}
	for l := 0; l < nLayers; l++ {
		lb := b[o : o+9]
		wc := lb[5:9]
		n := 0
		for n < 4 && wc[n] != 0 {
			n++
		}
		c.Layers = append(c.Layers, COFLayer{
			Component:   int(lb[0]),
			Shadow:      lb[1] != 0,
			Transparent: lb[3] != 0,
			DrawEffect:  int(lb[4]),
			WeaponClass: string(wc[:n]),
		})
		o += 9
	}
	for f := 0; f < c.FramesPerDir; f++ {
		c.Events = append(c.Events, int(b[o+f]))
	}
	o += c.FramesPerDir
	for i := 0; i < c.Directions*c.FramesPerDir*nLayers; i++ {
		c.order = append(c.order, int(b[o+i]))
	}
	return c, nil
}

// HitFrame returns the first frame flagged as an attack/missile/skill event, or -1.
func (c *COF) HitFrame() int {
	for i, e := range c.Events {
		if e == 1 || e == 2 || e == 4 {
			return i
		}
	}
	return -1
}

// Composite draws the layers in the order given by the COF.
// layers maps a component index to its decoded sprite; missing layers are skipped.
func (c *COF) Composite(layers map[int]*Sprite) (*Sprite, error) {
	var bounds image.Rectangle
	first := true
	for _, s := range layers {
		if len(s.Directions) != c.Directions {
			return nil, errors.Errorf("layer has %d directions, cof %d", len(s.Directions), c.Directions)
		}
		b := s.Bounds()
		if first {
			bounds, first = b, false
		} else {
			bounds = bounds.Union(b)
		}
	}
	if first {
		return nil, errors.New("no layer")
	}
	out := &Sprite{Directions: make([][]Frame, c.Directions)}
	n := len(c.Layers)
	for d := 0; d < c.Directions; d++ {
		for f := 0; f < c.FramesPerDir; f++ {
			img := newIndexed(bounds.Dx(), bounds.Dy())
			for i := 0; i < n; i++ {
				s, ok := layers[c.order[(d*c.FramesPerDir+f)*n+i]]
				if !ok || f >= len(s.Directions[d]) {
					continue
				}
				fr := s.Directions[d][f]
				ox, oy := fr.OffsetX-bounds.Min.X, fr.OffsetY-bounds.Min.Y
				for y := 0; y < fr.Height; y++ {
					for x := 0; x < fr.Width; x++ {
						if v := fr.Pix[y*fr.Width+x]; v != 0 {
							img.Pix[(oy+y)*img.Width+ox+x] = v
						}
					}
				}
			}
			out.Directions[d] = append(out.Directions[d], Frame{Indexed: img, OffsetX: bounds.Min.X, OffsetY: bounds.Min.Y})
		}
	}
	return out, nil
}

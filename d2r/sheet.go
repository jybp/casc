package d2r

import (
	"image"
)

// Bounds returns the box containing every frame of s, relative to the anchor.
func (s *Sprite) Bounds() image.Rectangle {
	var r image.Rectangle
	first := true
	for _, dir := range s.Directions {
		for _, f := range dir {
			fr := image.Rect(f.OffsetX, f.OffsetY, f.OffsetX+f.Width, f.OffsetY+f.Height)
			if first {
				r, first = fr, false
			} else {
				r = r.Union(fr)
			}
		}
	}
	return r
}

// Sheet draws every frame on a grid: one row per direction, one column per frame.
// All cells share the same size and anchor so the animation does not jitter.
func (s *Sprite) Sheet(p *Palette) *image.NRGBA {
	b := s.Bounds()
	cols := 0
	for _, d := range s.Directions {
		cols = maxInt(cols, len(d))
	}
	out := image.NewNRGBA(image.Rect(0, 0, maxInt(1, cols*b.Dx()), maxInt(1, len(s.Directions)*b.Dy())))
	for row, dir := range s.Directions {
		for col, f := range dir {
			ox := col*b.Dx() + f.OffsetX - b.Min.X
			oy := row*b.Dy() + f.OffsetY - b.Min.Y
			drawIndexed(out, f.Indexed, p, ox, oy)
		}
	}
	return out
}

// drawIndexed draws img at (ox, oy) on dst, skipping transparent pixels.
func drawIndexed(dst *image.NRGBA, img *Indexed, p *Palette, ox, oy int) {
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			v := img.Pix[y*img.Width+x]
			if v == 0 {
				continue
			}
			dx, dy := ox+x, oy+y
			if !(image.Point{dx, dy}.In(dst.Rect)) {
				continue
			}
			c := p[v]
			o := dst.PixOffset(dx, dy)
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = c.R, c.G, c.B, 255
		}
	}
}

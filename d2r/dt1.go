package d2r

import (
	"encoding/binary"
	"image"
	"sort"

	"github.com/pkg/errors"
)

// Tile is a decoded DT1 tile (floor, wall, roof or shadow piece of a level).
type Tile struct {
	Orientation int // 0 floor, 1-9 walls, 10-11 special, 12 roof-like, 13 shadow, 14 tree, 15 roof
	Main, Sub   int // tile style indices used by level presets
	Rarity      int
	*Indexed
}

var isoRowSkip = [15]int{14, 12, 10, 8, 6, 4, 2, 0, 2, 4, 6, 8, 10, 12, 14}
var isoRowLen = [15]int{4, 8, 12, 16, 20, 24, 28, 32, 28, 24, 20, 16, 12, 8, 4}

// ParseDT1 parses a .dt1 tile set (version 7.6).
func ParseDT1(b []byte) ([]Tile, error) {
	le := binary.LittleEndian
	if len(b) < 276 || le.Uint32(b) != 7 || le.Uint32(b[4:]) != 6 {
		return nil, errors.New("unsupported dt1 version")
	}
	n := int(le.Uint32(b[268:]))
	off := int(le.Uint32(b[272:]))
	if n < 0 || n > 1<<16 || off+n*96 > len(b) {
		return nil, errors.New("invalid dt1 tile table")
	}
	var tiles []Tile
	for t := 0; t < n; t++ {
		h := b[off+t*96:]
		blocksOff := int(le.Uint32(h[72:]))
		nBlocks := int(le.Uint32(h[80:]))
		tile := Tile{
			Orientation: int(int32(le.Uint32(h[20:]))),
			Main:        int(int32(le.Uint32(h[24:]))),
			Sub:         int(int32(le.Uint32(h[28:]))),
			Rarity:      int(int32(le.Uint32(h[32:]))),
		}
		if nBlocks <= 0 || blocksOff+nBlocks*20 > len(b) {
			continue
		}
		type block struct {
			x, y, format, length, offset int
		}
		var blocks []block
		var r image.Rectangle
		for i := 0; i < nBlocks; i++ {
			bh := b[blocksOff+i*20:]
			bl := block{
				x:      int(int16(le.Uint16(bh[0:]))),
				y:      int(int16(le.Uint16(bh[2:]))),
				format: int(le.Uint16(bh[8:])),
				length: int(le.Uint32(bh[10:])),
				offset: blocksOff + int(le.Uint32(bh[16:])),
			}
			if bl.offset < 0 || bl.offset+bl.length > len(b) {
				return nil, errors.New("invalid dt1 block")
			}
			bhgt := 32
			if bl.format == 1 {
				bhgt = 15
			}
			br := image.Rect(bl.x, bl.y, bl.x+32, bl.y+bhgt)
			if i == 0 {
				r = br
			} else {
				r = r.Union(br)
			}
			blocks = append(blocks, bl)
		}
		if r.Dx()*r.Dy() > 1<<22 {
			continue
		}
		img := newIndexed(r.Dx(), r.Dy())
		put := func(x, y int, v byte) {
			x, y = x-r.Min.X, y-r.Min.Y
			if x >= 0 && y >= 0 && x < img.Width && y < img.Height {
				img.Pix[y*img.Width+x] = v
			}
		}
		for _, bl := range blocks {
			data := b[bl.offset : bl.offset+bl.length]
			if bl.format == 1 {
				p := 0
				for row := 0; row < 15 && p < len(data); row++ {
					for k := 0; k < isoRowLen[row] && p < len(data); k++ {
						put(bl.x+isoRowSkip[row]+k, bl.y+row, data[p])
						p++
					}
				}
				continue
			}
			x, y := 0, 0
			for p := 0; p+1 < len(data); {
				skip, count := int(data[p]), int(data[p+1])
				p += 2
				if skip == 0 && count == 0 {
					x = 0
					y++
					continue
				}
				x += skip
				for k := 0; k < count && p < len(data); k++ {
					put(bl.x+x, bl.y+y, data[p])
					x++
					p++
				}
			}
		}
		tile.Indexed = img
		tiles = append(tiles, tile)
	}
	sort.SliceStable(tiles, func(i, j int) bool {
		a, c := tiles[i], tiles[j]
		if a.Orientation != c.Orientation {
			return a.Orientation < c.Orientation
		}
		if a.Main != c.Main {
			return a.Main < c.Main
		}
		return a.Sub < c.Sub
	})
	return tiles, nil
}

// TileSheet draws tiles on a grid of cols columns, each cell sized to the largest tile.
func TileSheet(tiles []Tile, p *Palette, cols int) *image.NRGBA {
	w, h := 1, 1
	for _, t := range tiles {
		w, h = maxInt(w, t.Width), maxInt(h, t.Height)
	}
	if cols <= 0 {
		cols = 8
	}
	rows := (len(tiles) + cols - 1) / cols
	out := image.NewNRGBA(image.Rect(0, 0, cols*w, maxInt(1, rows)*h))
	for i, t := range tiles {
		drawIndexed(out, t.Indexed, p, (i%cols)*w, (i/cols)*h+h-t.Height)
	}
	return out
}

package d2r

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

// DCC is the compressed sprite format of animated units (characters, monsters,
// objects). Each direction is encoded as a grid of 4x4 cells shared by all frames.
// Reference: Paul Siramy's DCC format documentation.

var dccBitWidths = [16]int{0, 1, 2, 4, 6, 8, 10, 12, 14, 16, 20, 24, 26, 28, 30, 32}

type box struct{ left, top, right, bottom int } // inclusive

func (b box) width() int  { return b.right - b.left + 1 }
func (b box) height() int { return b.bottom - b.top + 1 }

type dccCell struct {
	x, y, w, h int // position in direction space (relative to the direction box)
}

type dccFrame struct {
	box    box
	hCells int
	vCells int
	cells  []dccCell
}

type pixelEntry struct {
	value     [4]byte
	frame     int
	frameCell int
}

// ParseDCC parses a .dcc sprite. All frames of a direction share the same size.
func ParseDCC(b []byte) (*Sprite, error) {
	if len(b) < 15 || b[0] != 0x74 {
		return nil, errors.New("invalid dcc signature")
	}
	le := binary.LittleEndian
	dirs := int(b[2])
	frames := int(le.Uint32(b[3:]))
	if dirs == 0 || frames == 0 || frames > 1024 || 15+4*dirs > len(b) {
		return nil, errors.Errorf("invalid dcc layout %dx%d", dirs, frames)
	}
	s := &Sprite{Directions: make([][]Frame, dirs)}
	for d := 0; d < dirs; d++ {
		start := int(le.Uint32(b[15+4*d:]))
		end := len(b)
		if d+1 < dirs {
			end = int(le.Uint32(b[15+4*(d+1):]))
		}
		if start >= end || end > len(b) {
			return nil, errors.New("invalid dcc direction offset")
		}
		fs, err := decodeDCCDirection(b[start:end], frames)
		if err != nil {
			return nil, errors.Wrapf(err, "direction %d", d)
		}
		s.Directions[d] = fs
	}
	return s, nil
}

func decodeDCCDirection(b []byte, nFrames int) ([]Frame, error) {
	bs := newBitStream(b, 0, len(b)*8)
	bs.bits(32) // outsize coded
	compression := bs.bits(2)
	var0Bits := dccBitWidths[bs.bits(4)]
	widthBits := dccBitWidths[bs.bits(4)]
	heightBits := dccBitWidths[bs.bits(4)]
	xOffBits := dccBitWidths[bs.bits(4)]
	yOffBits := dccBitWidths[bs.bits(4)]
	optBits := dccBitWidths[bs.bits(4)]
	codedBits := dccBitWidths[bs.bits(4)]

	frames := make([]dccFrame, nFrames)
	optional := 0
	dir := box{left: 1 << 30, top: 1 << 30, right: -(1 << 30), bottom: -(1 << 30)}
	for i := range frames {
		bs.bits(var0Bits)
		w := bs.bits(widthBits)
		h := bs.bits(heightBits)
		x := bs.signedBits(xOffBits)
		y := bs.signedBits(yOffBits)
		optional += bs.bits(optBits)
		bs.bits(codedBits)
		bottomUp := bs.bit() == 1
		if w <= 0 || h <= 0 || w > 4096 || h > 4096 {
			return nil, errors.Errorf("invalid frame size %dx%d", w, h)
		}
		fb := box{left: x, right: x + w - 1}
		if bottomUp {
			fb.top, fb.bottom = y, y+h-1
		} else {
			fb.bottom, fb.top = y, y-h+1
		}
		frames[i].box = fb
		dir.left, dir.top = minInt(dir.left, fb.left), minInt(dir.top, fb.top)
		dir.right, dir.bottom = maxInt(dir.right, fb.right), maxInt(dir.bottom, fb.bottom)
	}
	if optional > 0 {
		bs.pos = (bs.pos+7)&^7 + optional*8
	}
	equalCellsSize, rawSize, encodingSize := 0, 0, 0
	if compression&2 != 0 {
		equalCellsSize = bs.bits(20)
	}
	pixelMaskSize := bs.bits(20)
	if compression&1 != 0 {
		encodingSize = bs.bits(20)
		rawSize = bs.bits(20)
	}
	var paletteEntries [256]byte
	n := 0
	for i := 0; i < 256; i++ {
		if bs.bit() == 1 {
			paletteEntries[n] = byte(i)
			n++
		}
	}
	pos := bs.pos
	ec := newBitStream(b, pos, pos+equalCellsSize)
	pos += equalCellsSize
	pm := newBitStream(b, pos, pos+pixelMaskSize)
	pos += pixelMaskSize
	et := newBitStream(b, pos, pos+encodingSize)
	pos += encodingSize
	rp := newBitStream(b, pos, pos+rawSize)
	pos += rawSize
	pcd := newBitStream(b, pos, len(b)*8)

	dirW, dirH := dir.width(), dir.height()
	if dirW*dirH > 1<<24 {
		return nil, errors.New("direction too large")
	}
	dirHCells := 1 + (dirW-1)/4
	dirVCells := 1 + (dirH-1)/4

	// Cells of each frame.
	for i := range frames {
		f := &frames[i]
		f.hCells, f.cells = 0, nil
		w0 := 4 - (f.box.left-dir.left)%4
		h0 := 4 - (f.box.top-dir.top)%4
		widths := cellSizes(f.box.width(), w0)
		heights := cellSizes(f.box.height(), h0)
		f.hCells, f.vCells = len(widths), len(heights)
		y := f.box.top - dir.top
		for _, ch := range heights {
			x := f.box.left - dir.left
			for _, cw := range widths {
				f.cells = append(f.cells, dccCell{x: x, y: y, w: cw, h: ch})
				x += cw
			}
			y += ch
		}
	}

	// Fill the pixel buffer.
	current := make([]*pixelEntry, dirHCells*dirVCells)
	var entries []*pixelEntry
	for fi := range frames {
		f := &frames[fi]
		originX := (f.box.left - dir.left) / 4
		originY := (f.box.top - dir.top) / 4
		for cy := 0; cy < f.vCells; cy++ {
			for cx := 0; cx < f.hCells; cx++ {
				ci := originX + cx + (originY+cy)*dirHCells
				if ci < 0 || ci >= len(current) {
					return nil, errors.New("cell out of bounds")
				}
				mask := 0x0f
				if current[ci] != nil {
					same := 0
					if equalCellsSize > 0 {
						same = ec.bit()
					}
					if same == 1 {
						continue
					}
					mask = pm.bits(4)
				}
				var stack [4]int
				last := 0
				decoded := 0
				nbits := popcount4(mask)
				encoding := 0
				if nbits > 0 && encodingSize > 0 {
					encoding = et.bit()
				}
				for i := 0; i < nbits; i++ {
					if encoding != 0 {
						stack[i] = rp.bits(8)
					} else {
						stack[i] = last
						disp := pcd.bits(4)
						stack[i] += disp
						for disp == 15 {
							disp = pcd.bits(4)
							stack[i] += disp
						}
					}
					if stack[i] == last {
						stack[i] = 0
						break
					}
					last = stack[i]
					decoded++
				}
				var old [4]byte
				if current[ci] != nil {
					old = current[ci].value
				}
				e := &pixelEntry{frame: fi, frameCell: cx + cy*f.hCells}
				idx := decoded - 1
				for i := 0; i < 4; i++ {
					if mask&(1<<uint(i)) != 0 {
						if idx >= 0 {
							e.value[i] = byte(stack[idx])
							idx--
						}
					} else {
						e.value[i] = old[i]
					}
				}
				current[ci] = e
				entries = append(entries, e)
			}
		}
	}
	for _, e := range entries {
		for i := range e.value {
			e.value[i] = paletteEntries[e.value[i]]
		}
	}

	// Generate frames in direction space.
	type lastCell struct {
		w, h int
		img  *Indexed
	}
	lasts := make([]*lastCell, dirHCells*dirVCells)
	out := make([]Frame, 0, nFrames)
	ei := 0
	for fi := range frames {
		f := &frames[fi]
		img := newIndexed(dirW, dirH)
		originX := (f.box.left - dir.left) / 4
		originY := (f.box.top - dir.top) / 4
		for cy := 0; cy < f.vCells; cy++ {
			for cx := 0; cx < f.hCells; cx++ {
				cell := f.cells[cx+cy*f.hCells]
				ci := originX + cx + (originY+cy)*dirHCells
				var e *pixelEntry
				if ei < len(entries) && entries[ei].frame == fi && entries[ei].frameCell == cx+cy*f.hCells {
					e = entries[ei]
					ei++
				}
				if e == nil {
					// Same cell as in the previous frame using it.
					if l := lasts[ci]; l != nil && l.w == cell.w && l.h == cell.h {
						for y := 0; y < cell.h; y++ {
							o := (cell.y+y)*dirW + cell.x
							copy(img.Pix[o:o+cell.w], l.img.Pix[o:o+cell.w])
						}
					}
				} else if e.value[0] == e.value[1] {
					for y := 0; y < cell.h; y++ {
						for x := 0; x < cell.w; x++ {
							img.Pix[(cell.y+y)*dirW+cell.x+x] = e.value[0]
						}
					}
				} else {
					nb := 2
					if e.value[1] == e.value[2] {
						nb = 1
					}
					for y := 0; y < cell.h; y++ {
						for x := 0; x < cell.w; x++ {
							img.Pix[(cell.y+y)*dirW+cell.x+x] = e.value[pcd.bits(nb)]
						}
					}
				}
				lasts[ci] = &lastCell{w: cell.w, h: cell.h, img: img}
			}
		}
		out = append(out, Frame{Indexed: img, OffsetX: dir.left, OffsetY: dir.top})
	}
	if pcd.overflow() || pm.overflow() {
		return nil, errors.New("truncated dcc bitstream")
	}
	return out, nil
}

// cellSizes splits size into cells: first cell of first, then cells of 4, the rest last.
func cellSizes(size, first int) []int {
	if size-first <= 1 {
		return []int{size}
	}
	tmp := size - first - 1
	n := 2 + tmp/4
	if tmp%4 == 0 {
		n--
	}
	sizes := make([]int, n)
	sizes[0] = first
	for i := 1; i < n-1; i++ {
		sizes[i] = 4
	}
	sizes[n-1] = size - first - 4*(n-2)
	return sizes
}

func popcount4(v int) int {
	n := 0
	for i := 0; i < 4; i++ {
		n += (v >> uint(i)) & 1
	}
	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

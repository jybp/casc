package d2r

import (
	"encoding/binary"
	"math"
)

// Block compression decoders. Each decoder writes 16 RGBA pixels in row-major order.

func rgb565(c uint16) [3]int {
	r := int(c>>11) & 0x1f
	g := int(c>>5) & 0x3f
	b := int(c) & 0x1f
	return [3]int{r<<3 | r>>2, g<<2 | g>>4, b<<3 | b>>2}
}

func decodeColorBlock(block []byte, out *[16][4]uint8, allowAlpha bool) {
	c0 := binary.LittleEndian.Uint16(block[0:])
	c1 := binary.LittleEndian.Uint16(block[2:])
	a, b := rgb565(c0), rgb565(c1)
	var pal [4][4]uint8
	for i := 0; i < 3; i++ {
		pal[0][i] = uint8(a[i])
		pal[1][i] = uint8(b[i])
	}
	pal[0][3], pal[1][3] = 255, 255
	if c0 > c1 || !allowAlpha {
		for i := 0; i < 3; i++ {
			pal[2][i] = uint8((2*a[i] + b[i] + 1) / 3)
			pal[3][i] = uint8((a[i] + 2*b[i] + 1) / 3)
		}
		pal[2][3], pal[3][3] = 255, 255
	} else {
		for i := 0; i < 3; i++ {
			pal[2][i] = uint8((a[i] + b[i]) / 2)
		}
		pal[2][3] = 255
		pal[3] = [4]uint8{0, 0, 0, 0}
	}
	idx := binary.LittleEndian.Uint32(block[4:])
	for i := 0; i < 16; i++ {
		out[i] = pal[(idx>>(2*uint(i)))&3]
	}
}

func decodeBC1(block []byte, out *[16][4]uint8) {
	decodeColorBlock(block, out, true)
}

// decodeBC4Channel decodes a BC4 block into 16 values.
func decodeBC4Channel(block []byte) [16]uint8 {
	r0, r1 := int(block[0]), int(block[1])
	var pal [8]int
	pal[0], pal[1] = r0, r1
	if r0 > r1 {
		for i := 1; i <= 6; i++ {
			pal[1+i] = ((7-i)*r0 + i*r1 + 3) / 7
		}
	} else {
		for i := 1; i <= 4; i++ {
			pal[1+i] = ((5-i)*r0 + i*r1 + 2) / 5
		}
		pal[6], pal[7] = 0, 255
	}
	var bits uint64
	for i := 0; i < 6; i++ {
		bits |= uint64(block[2+i]) << (8 * uint(i))
	}
	var v [16]uint8
	for i := 0; i < 16; i++ {
		v[i] = uint8(pal[(bits>>(3*uint(i)))&7])
	}
	return v
}

func decodeBC4Gray(block []byte, out *[16][4]uint8) {
	v := decodeBC4Channel(block)
	for i := 0; i < 16; i++ {
		out[i] = [4]uint8{v[i], v[i], v[i], 255}
	}
}

func normalZ(nx, ny uint8) uint8 {
	x := float64(nx)/127.5 - 1
	y := float64(ny)/127.5 - 1
	z := 1 - x*x - y*y
	if z < 0 {
		z = 0
	}
	return uint8(math.Round((math.Sqrt(z) + 1) * 127.5))
}

func decodeBC3(block []byte, out *[16][4]uint8) {
	decodeColorBlock(block[8:16], out, false)
	a := decodeBC4Channel(block[0:8])
	for i := 0; i < 16; i++ {
		out[i][3] = a[i]
	}
}

// decodeBC5 decodes the two channels into red and green.
func decodeBC5(block []byte, out *[16][4]uint8) {
	r := decodeBC4Channel(block[0:8])
	g := decodeBC4Channel(block[8:16])
	for i := 0; i < 16; i++ {
		out[i] = [4]uint8{r[i], g[i], 0, 255}
	}
}

package d2r

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestCellSizes(t *testing.T) {
	cases := []struct {
		size, first int
		want        []int
	}{
		{1, 4, []int{1}},
		{4, 4, []int{4}},
		{5, 4, []int{5}}, // a trailing single pixel is merged
		{10, 4, []int{4, 4, 2}},
		{12, 2, []int{2, 4, 4, 2}},
		{11, 2, []int{2, 4, 5}},
		{13, 4, []int{4, 4, 5}},
	}
	for _, c := range cases {
		if got := cellSizes(c.size, c.first); !reflect.DeepEqual(got, c.want) {
			t.Errorf("cellSizes(%d, %d) = %v want %v", c.size, c.first, got, c.want)
		}
	}
}

func TestParseDC6(t *testing.T) {
	// One direction, one 3x2 frame stored bottom-up:
	// bottom row: 2 transparent then index 7; top row: indices 1, 2.
	data := []byte{0x82, 1, 7, 0x80, 2, 1, 2, 0x80}
	b := make([]byte, 24+4+32+len(data)+3)
	le := binary.LittleEndian
	le.PutUint32(b[0:], 6)
	le.PutUint32(b[4:], 1)
	le.PutUint32(b[16:], 1)
	le.PutUint32(b[20:], 1)
	le.PutUint32(b[24:], 28)
	h := b[28:]
	le.PutUint32(h[4:], 3)
	le.PutUint32(h[8:], 2)
	le.PutUint32(h[12:], uint32(0xFFFFFFFF)) // offset x = -1
	le.PutUint32(h[16:], 5)                  // offset y (bottom) = 5
	le.PutUint32(h[28:], uint32(len(data)))
	copy(h[32:], data)
	s, err := ParseDC6(b)
	if err != nil {
		t.Fatal(err)
	}
	f := s.Directions[0][0]
	if want := []byte{1, 2, 0, 0, 0, 7}; !reflect.DeepEqual(f.Pix, want) {
		t.Fatalf("pixels %v want %v", f.Pix, want)
	}
	if f.OffsetX != -1 || f.OffsetY != 4 {
		t.Fatalf("offset %d,%d", f.OffsetX, f.OffsetY)
	}
}

func TestParseCOF(t *testing.T) {
	// 2 layers, 2 frames, 1 direction.
	b := make([]byte, 28)
	b[0], b[1], b[2], b[3] = 2, 2, 1, 20
	binary.LittleEndian.PutUint32(b[8:], uint32(0xFFFFFFF6)) // left -10
	binary.LittleEndian.PutUint32(b[12:], 10)
	binary.LittleEndian.PutUint32(b[16:], uint32(0xFFFFFFEC)) // top -20
	binary.LittleEndian.PutUint32(b[20:], 0)
	b[24] = 0
	b[25] = 1                                      // speed 256
	b = append(b, 1, 1, 0, 0, 0, 'h', 't', 'h', 0) // TR
	b = append(b, 2, 0, 0, 1, 5, '1', 'h', 's', 0) // LG, transparent
	b = append(b, 0, 1)                            // events: hit on frame 1
	b = append(b, 2, 1, 1, 2)                      // draw order per frame
	c, err := ParseCOF(b)
	if err != nil {
		t.Fatal(err)
	}
	if c.AnimationSpeed != 256 || c.HitFrame() != 1 || c.Box.Min.X != -10 || c.Box.Min.Y != -20 {
		t.Fatalf("unexpected cof %+v", c)
	}
	if c.Layers[0].WeaponClass != "hth" || !c.Layers[1].Transparent || Components[c.Layers[1].Component] != "lg" {
		t.Fatalf("unexpected layers %+v", c.Layers)
	}
	// Composite: the second listed layer is drawn on top.
	one := func(v byte) *Sprite {
		img := newIndexed(1, 1)
		img.Pix[0] = v
		f := Frame{Indexed: img}
		return &Sprite{Directions: [][]Frame{{f, f}}}
	}
	s, err := c.Composite(map[int]*Sprite{1: one(5), 2: one(9)})
	if err != nil {
		t.Fatal(err)
	}
	if s.Directions[0][0].Pix[0] != 5 || s.Directions[0][1].Pix[0] != 9 {
		t.Fatalf("draw order not respected: %d %d", s.Directions[0][0].Pix[0], s.Directions[0][1].Pix[0])
	}
}

func TestParseAnimData(t *testing.T) {
	b := make([]byte, 256*4+160)
	binary.LittleEndian.PutUint32(b[0:], 1)
	r := b[4:]
	copy(r, "XXA1HTH")
	binary.LittleEndian.PutUint32(r[8:], 12)
	binary.LittleEndian.PutUint32(r[12:], 128)
	r[16+7] = 1
	a, err := ParseAnimData(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 1 || a[0].Name != "XXA1HTH" || a[0].FramesPerDir != 12 || a[0].HitFrame() != 7 || a[0].FPS() != 12.5 {
		t.Fatalf("unexpected %+v", a)
	}
}

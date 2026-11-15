package d2r

import "testing"

func TestBC1(t *testing.T) {
	// c0 = white, c1 = black, 4 colors mode, indices: 0,1,2,3 repeated.
	block := []byte{0xff, 0xff, 0x00, 0x00, 0xe4, 0xe4, 0xe4, 0xe4}
	var out [16][4]uint8
	decodeBC1(block, &out)
	want := [4][4]uint8{{255, 255, 255, 255}, {0, 0, 0, 255}, {170, 170, 170, 255}, {85, 85, 85, 255}}
	for i := 0; i < 16; i++ {
		if out[i] != want[i%4] {
			t.Fatalf("pixel %d = %v want %v", i, out[i], want[i%4])
		}
	}
}

func TestBC4(t *testing.T) {
	// r0 = 255, r1 = 0, 8 values mode, all indices = 1 => 0.
	block := []byte{255, 0, 0x49, 0x92, 0x24, 0x49, 0x92, 0x24}
	v := decodeBC4Channel(block)
	for i, x := range v {
		if x != 0 {
			t.Fatalf("value %d = %d", i, x)
		}
	}
}

func TestNormalZ(t *testing.T) {
	if z := normalZ(128, 128); z < 254 {
		t.Fatalf("flat normal z = %d", z)
	}
}

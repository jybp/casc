package granny

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"os"
	"testing"
)

func TestAdaptiveModelInit(t *testing.T) {
	for _, c := range []struct{ vocab, min int }{{300, 36}, {40, 0}, {21, 0}} {
		m := newAdaptiveModel(c.vocab, c.min)
		if m.sums[0] != 0 || m.sums[c.vocab] != 1<<15 {
			t.Fatalf("vocab %d: bounds %d..%d", c.vocab, m.sums[0], m.sums[c.vocab])
		}
		for i := 0; i < c.vocab; i++ {
			if m.sums[i+1] <= m.sums[i] {
				t.Fatalf("vocab %d: symbol %d has no probability", c.vocab, i)
			}
		}
		// Every code maps back to the symbol owning it.
		for code := uint32(0); code < 1<<15; code += 7 {
			s := m.find(code)
			if code < uint32(m.sums[s]) || code >= uint32(m.sums[s+1]) {
				t.Fatalf("vocab %d: code %d mapped to %d", c.vocab, code, s)
			}
		}
		// Adaptation keeps the invariants.
		for i := 0; i < 5000; i++ {
			m.observe(i % 3)
		}
		if m.sums[c.vocab] != 1<<15 {
			t.Fatalf("vocab %d: total %d after adaptation", c.vocab, m.sums[c.vocab])
		}
		for i := 0; i < c.vocab; i++ {
			if m.sums[i+1] <= m.sums[i] {
				t.Fatalf("vocab %d: symbol %d lost its probability", c.vocab, i)
			}
		}
	}
}

func TestBitKnitRawQuantum(t *testing.T) {
	payload := []byte("raw quantum data")
	var src bytes.Buffer
	binary.Write(&src, binary.LittleEndian, uint16(bitknit2Magic))
	binary.Write(&src, binary.LittleEndian, uint16(0))
	src.Write(payload)
	out, err := DecompressBitKnit2(src.Bytes(), len(payload))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, payload) {
		t.Fatalf("got %q", out)
	}
	if _, err := DecompressBitKnit2([]byte{0, 0}, 4); err == nil {
		t.Fatal("expected invalid magic error")
	}
}

func TestHalf(t *testing.T) {
	for h, want := range map[uint16]float32{0x3C00: 1, 0xC000: -2, 0x3800: 0.5, 0x0000: 0, 0x7BFF: 65504} {
		if got := halfToFloat(h); got != want {
			t.Errorf("half %04x = %v want %v", h, got, want)
		}
	}
}

// TestRealFile parses and exports a real file: GR2=path/to/file.model go test ./granny -run RealFile
func TestRealFile(t *testing.T) {
	p := os.Getenv("GR2")
	if p == "" {
		t.Skip("set GR2 to a .gr2/.model file")
	}
	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(b)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	glb, warnings, err := ExportGLB(f, ExportOptions{Skin: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(glb) < 20 || binary.LittleEndian.Uint32(glb) != 0x46546C67 {
		t.Fatal("invalid glb")
	}
	for _, w := range warnings {
		t.Log(w)
	}
	t.Logf("%d meshes, %d bytes", len(objects(f.Root.Get("Meshes"))), len(glb))
}

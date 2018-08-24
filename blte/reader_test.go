package blte

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"io/ioutil"
	"testing"
)

var uncompressed = []byte{'N', 'h', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd'}
var compressed = []byte{'Z', 120, 156, 202, 72, 205, 201, 201, 215, 81, 40, 207,
	47, 202, 73, 225, 2, 4, 0, 0, 255, 255, 33, 231, 4, 147}

func TestOneChunk(t *testing.T) {
	r, err := NewReader(bytes.NewReader(oneChunk()))
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{'h', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd'}
	actual, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(expected, actual) != 0 {
		t.Fatalf("exected:%+v\nactual:%+v", expected, actual)
	}
}

func oneChunk() []byte {
	return concat(
		[]byte{
			/*sig  */ 66, 76, 84, 69,
			/*size */ 0, 0, 0, 0,
		},
		compressed,
	)
}

func TestTwoChunks(t *testing.T) {
	r, err := NewReader(bytes.NewReader(twoChunks()))
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{'h', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd',
		'h', 'e', 'l', 'l', 'o', ',', ' ', 'w', 'o', 'r', 'l', 'd'}
	actual, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(expected, actual) != 0 {
		t.Fatalf("exected:%s\nactual:%s", expected, actual)
	}
}

func twoChunks() []byte {
	hashZ := md5.Sum(compressed)
	hashN := md5.Sum(uncompressed)
	var headerSize = make([]byte, 4)
	binary.BigEndian.PutUint32(headerSize, 12+24*2)
	return concat(
		[]byte{66, 76, 84, 69}, //sig
		headerSize,
		[]byte{
			/*unk  */ 0, 0,
			/*count*/ 0, 2,
			/*csize*/ 0, 0, 0, 25 + 1,
			/*usize*/ 0, 0, 0, 12,
		},
		hashZ[:],
		[]byte{
			/*csize*/ 0, 0, 0, 12 + 1,
			/*usize*/ 0, 0, 0, 12,
		},
		hashN[:],
		compressed,
		uncompressed,
	)
}

func concat(slices ...[]byte) []byte {
	var tmp []byte
	for _, s := range slices {
		tmp = append(tmp, s...)
	}
	return tmp
}

package blte

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestReader(t *testing.T) {
	//Requires valid encoded and decoded version of a file
	return

	e, err := os.Open("test_encoded")
	if err != nil {
		t.Error(err)
	}
	defer e.Close()

	d, err := os.Open("test_decoded")
	if err != nil {
		t.Error(err)
	}
	defer d.Close()

	expected, err := ioutil.ReadAll(d)
	if err != nil {
		t.Error(err)
	}

	w := bytes.NewBuffer([]byte{})
	if err = Decode(e, w); err != nil {
		t.Error(err)
	}

	actual, err := ioutil.ReadAll(w)
	if err != nil {
		t.Error(err)
	}

	if string(expected) != string(actual) {
		t.FailNow()
	}
}

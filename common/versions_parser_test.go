package common

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

func TestParseLocalVersionsWithSharedStorage(t *testing.T) {
	data := bytes.NewBufferString(`Branch!STRING:0|Active!DEC:1|Build Key!HEX:16|CDN Key!HEX:16|Install Key!HEX:16|IM Size!DEC:4|CDN Path!STRING:0|CDN Hosts!STRING:0|CDN Servers!STRING:0|Tags!STRING:0|Armadillo!STRING:0|Last Activated!STRING:0|Version!STRING:0|Product!STRING:0
eu|1|733e8f4a3e8e0feaa44d52b458592651|d0427daa9162695282f0daeffb46b1d1|||||||||1.32.6.15355|w3
eu|1|b5789e1d3f34ffb8a19b9273166d55c0|d0427daa9162695282f0daeffb46b1d1|||||||||1.32.7.15539|w3t
`)
	expected := []Version{
		{Region: "eu", BuildConfigHash: must(hex.DecodeString("733e8f4a3e8e0feaa44d52b458592651")), CDNConfigHash: must(hex.DecodeString("d0427daa9162695282f0daeffb46b1d1")), Name: "1.32.6.15355", ProductCode: "w3"},
		{Region: "eu", BuildConfigHash: must(hex.DecodeString("b5789e1d3f34ffb8a19b9273166d55c0")), CDNConfigHash: must(hex.DecodeString("d0427daa9162695282f0daeffb46b1d1")), Name: "1.32.7.15539", ProductCode: "w3t"},
	}
	actual, err := ParseLocalBuildInfo(data)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}
	assert.Equal(t, len(expected), len(actual))
	for i := range expected {
		if !reflect.DeepEqual(expected[i], actual[i]) {
			t.Errorf("version mismatch %+v %+v", expected[i], actual[i])
		}
	}
}

func TestParseLocalVersions(t *testing.T) {
	data := bytes.NewBufferString(`Branch!STRING:0|Active!DEC:1|Build Key!HEX:16|CDN Key!HEX:16|Install Key!HEX:16|IM Size!DEC:4|CDN Path!STRING:0|CDN Hosts!STRING:0|CDN Servers!STRING:0|Tags!STRING:0|Armadillo!STRING:0|Last Activated!STRING:0|Version!STRING:0
eu|1|17992473d8a335eb5a7fed6699462db8|852ac94d909ed7dcf2d3b76a0e85b16a|||||||||2.6.9.68722
`)

	expected := []Version{
		{Region: "eu", BuildConfigHash: must(hex.DecodeString("17992473d8a335eb5a7fed6699462db8")), CDNConfigHash: must(hex.DecodeString("852ac94d909ed7dcf2d3b76a0e85b16a")), Name: "2.6.9.68722"},
	}
	actual, err := ParseLocalBuildInfo(data)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}
	assert.Equal(t, len(expected), len(actual))
	for i := range expected {
		if !reflect.DeepEqual(expected[i], actual[i]) {
			t.Errorf("version mismatch %+v %+v", expected[i], actual[i])
		}
	}
}

func TestParseOnlineVersions(t *testing.T) {
	data := bytes.NewBufferString(`Region!STRING:0|BuildConfig!HEX:16|CDNConfig!HEX:16|KeyRing!HEX:16|BuildId!DEC:4|VersionsName!String:0|ProductConfig!HEX:16
## seqn = 11111
us|6a9e9d6b2a070a4c6a3b777beeb2b7c0|351c5adcdda3a2553ed1aa3ae5332a38||1|1.1.1.11111|c
eu|66d0476334023bb1eaa241424f9ad178|07b668246e2cb87bfc6aa7a4a825a348||2|2.1.1.11111|f
`)
	expected := []Version{
		{Region: "us", BuildConfigHash: must(hex.DecodeString("6a9e9d6b2a070a4c6a3b777beeb2b7c0")), CDNConfigHash: must(hex.DecodeString("351c5adcdda3a2553ed1aa3ae5332a38")), Name: "1.1.1.11111"},
		{Region: "eu", BuildConfigHash: must(hex.DecodeString("66d0476334023bb1eaa241424f9ad178")), CDNConfigHash: must(hex.DecodeString("07b668246e2cb87bfc6aa7a4a825a348")), Name: "2.1.1.11111"},
	}
	actual, err := ParseOnlineVersions(data)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, len(expected), len(actual))
	for i := range expected {
		if !reflect.DeepEqual(expected[i], actual[i]) {
			t.Errorf("version mismatch %+v %+v", expected[i], actual[i])
		}
	}
}

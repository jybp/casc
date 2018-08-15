package common

import (
	"bytes"
	"reflect"
	"testing"
)

func TestParseVersion(t *testing.T) {
	data := bytes.NewBufferString(`Region!STRING:0|BuildConfig!HEX:16|CDNConfig!HEX:16|KeyRing!HEX:16|BuildId!DEC:4|VersionsName!String:0|ProductConfig!HEX:16
## seqn = 11111
us|6a9e9d6b2a070a4c6a3b777beeb2b7c0|351c5adcdda3a2553ed1aa3ae5332a38||1|1.1.1.11111|c
eu|66d0476334023bb1eaa241424f9ad178|07b668246e2cb87bfc6aa7a4a825a348||2|2.1.1.11111|f
`)

	expected := map[string]Version{
		"us": Version{BuildConfigHash: []byte("6a9e9d6b2a070a4c6a3b777beeb2b7c0"), CDNConfigHash: []byte("351c5adcdda3a2553ed1aa3ae5332a38"), Name: "1.1.1.11111"},
		"eu": Version{BuildConfigHash: []byte("66d0476334023bb1eaa241424f9ad178"), CDNConfigHash: []byte("07b668246e2cb87bfc6aa7a4a825a348"), Name: "2.1.1.11111"},
	}

	vers, err := ParseVersions(data)
	if err != nil {
		t.Error(err)
		return
	}

	for expectedK, expectedVer := range expected {
		ver, ok := vers[expectedK]
		if !ok {
			t.Errorf("%s region not found", expectedK)
		}

		if reflect.DeepEqual(expectedVer, ver) {
			t.Errorf("version mismatch %+v %+v", expectedVer, ver)
		}
	}
}

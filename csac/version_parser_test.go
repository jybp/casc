package csac

import (
	"bytes"
	"testing"
)

func TestParseVersion(t *testing.T) {
	data := bytes.NewBufferString(`Region!STRING:0|BuildConfig!HEX:16|CDNConfig!HEX:16|KeyRing!HEX:16|BuildId!DEC:4|VersionsName!String:0|ProductConfig!HEX:16
## seqn = 11111
us|a|b||1|1.1.1.11111|c
eu|d|e||2|2.1.1.11111|f
kr|g|h||3|3.1.1.11111|i
tw|j|k||4|4.1.1.11111|l
`)

	expected := map[string]Version{
		"us": Version{BuildHash: "a", CDNHash: "b", ID: 1, Name: "1.1.1.11111"},
		"eu": Version{BuildHash: "d", CDNHash: "e", ID: 2, Name: "2.1.1.11111"},
		"kr": Version{BuildHash: "g", CDNHash: "h", ID: 3, Name: "3.1.1.11111"},
		"tw": Version{BuildHash: "j", CDNHash: "k", ID: 4, Name: "4.1.1.11111"},
	}

	vers, err := ParseVersions(data)
	if err != nil {
		t.Error(err)
	}

	for expectedK, expectedVer := range expected {
		ver, ok := vers[expectedK]
		if !ok {
			t.Errorf("%s region not found", expectedK)
		}

		if expectedVer != ver {
			t.Errorf("version mismatch %+v %+v", expectedVer, ver)
		}
	}
}

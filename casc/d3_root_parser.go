package casc

import (
	"encoding/binary"
	"errors"
	"io"
)

//d3 specific
type Root struct {
	Header RootHeader
}

type RootHeader struct {
}

func ParseRoot(r io.Reader) (Root, error) {
	var sig uint32
	if err := binary.Read(r, binary.BigEndian, &sig); err != nil {
		return Root{}, err
	}

	if sig != 0x8007D0C4 /* Diablo III */ {
		return Root{}, errors.New("Unsupported Root file")
	}

	var num uint32
	if err := binary.Read(r, binary.BigEndian, &num); err != nil {
		return Root{}, err
	}

	return Root{}, nil
}

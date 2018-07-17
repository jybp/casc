package casc

import "io"

const arBlockSize uint32 = 1 << 20

type ArchiveIndex struct {
	BlteHeaderHash  [8]uint8
	BlteEncodedSize uint32
	BlteOffset      uint32
}

func ParseArchiveIndex(r io.Reader) (ArchiveIndex, error) {

	size := 4096
	_ = size
	return ArchiveIndex{}, nil
}

package csac

import "io"

const arBlockSize uint32 = 1 << 20

type ArchiveIndex struct {
}

func ParseArchiveIndex(r io.Reader) (ArchiveIndex, error) {

	size := 4096
	_ = size
	return ArchiveIndex{}, nil
}

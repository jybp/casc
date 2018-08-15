package common

import (
	"bufio"
	"encoding/hex"
	"io"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Version struct {
	BuildConfigHash []byte
	CDNConfigHash   []byte
	Name            string // i.e. A.B.C.XXXXX
	ID              int    // last part of Version.Name
}

// ParseVersions returns a map of region:version
func ParseVersions(r io.Reader) (map[string]Version, error) {
	vers := map[string]Version{}
	scanner := bufio.NewScanner(r)
	n := 0
	for scanner.Scan() {
		n++
		if n <= 2 { //TODO parse header
			continue
		}
		line := scanner.Text()
		if !strings.ContainsRune(line, '|') {
			continue
		}
		split := strings.Split(line, "|")

		if len(split) < 6 {
			return nil, errors.WithStack(errors.New("unexpected number of |"))
		}

		id, err := strconv.Atoi(split[4])
		if err != nil {
			return nil, errors.WithStack(err)
		}

		buildHash, err := hex.DecodeString(split[1])
		if err != nil {
			return nil, errors.WithStack(err)
		}
		cdnHash, err := hex.DecodeString(split[2])
		if err != nil {
			return nil, errors.WithStack(err)
		}
		vers[split[0]] = Version{
			BuildConfigHash: buildHash,
			CDNConfigHash:   cdnHash,
			ID:              id,
			Name:            split[5],
		}
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return vers, nil
}

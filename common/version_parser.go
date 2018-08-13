package common

import (
	"bufio"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"
)

type Version struct {
	BuildHash []byte
	CDNHash   []byte
	Name      string // i.e. A.B.C.XXXXX
	ID        int    // last part of Version.Name
}

// ParseVersions returns a map of region:version
func ParseVersions(r io.Reader) (map[string]Version, error) {
	vers := map[string]Version{}
	scanner := bufio.NewScanner(r)
	n := 0
	for scanner.Scan() {
		n++
		if n <= 2 {
			continue
		}
		line := scanner.Text()
		if !strings.ContainsRune(line, '|') {
			continue
		}
		split := strings.Split(line, "|")

		if len(split) < 6 {
			return nil, errors.New("unexpected number of |")
		}

		id, err := strconv.Atoi(split[4])
		if err != nil {
			return nil, err
		}

		buildHash, err := hex.DecodeString(split[1])
		if err != nil {
			return nil, err
		}
		cdnHash, err := hex.DecodeString(split[2])
		if err != nil {
			return nil, err
		}
		vers[split[0]] = Version{
			BuildHash: buildHash,
			CDNHash:   cdnHash,
			ID:        id,
			Name:      split[5],
		}
		if err != nil {
			return nil, err
		}
	}
	return vers, nil
}

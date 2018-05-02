package csac

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

type Version struct {
	BuildHash string // build hash?
	CDNHash   string // cdn hash?
	ID        int    // last part of the displayed version name
	Name      string // displayed version name: A.B.C.XXXXX
}

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
		vers[split[0]] = Version{
			BuildHash: split[1],
			CDNHash:   split[2],
			ID:        id,
			Name:      split[5],
		}
		if err != nil {
			return nil, err
		}
	}
	return vers, nil
}

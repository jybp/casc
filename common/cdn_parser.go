package common

import (
	"bufio"
	"io"
	"strings"

	"github.com/pkg/errors"
)

type Cdn struct {
	Path  string
	Hosts []string
}

func ParseCdn(r io.Reader) (map[string]Cdn, error) {
	cdns := map[string]Cdn{}

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

		if len(split) < 3 {
			return nil, errors.WithStack(errors.New("unexpected number of |"))
		}
		hosts := strings.Split(split[2], " ")
		if len(hosts) == 0 {
			return nil, errors.WithStack(errors.New("unexpected host format"))
		}

		cdns[split[0]] = Cdn{
			Path:  split[1],
			Hosts: hosts,
		}
	}
	return cdns, nil
}

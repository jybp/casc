package common

import (
	"io"
	"strings"
)

type Cdn struct {
	Path  string
	Hosts []string
}

func ParseCdn(r io.Reader) (map[string]Cdn, error) {
	region, path, hosts := "Name", "Path", "Hosts"
	csv, err := ParseCSV(r, region, path, hosts)
	if err != nil {
		return nil, err
	}
	cdns := map[string]Cdn{}
	for _, row := range csv {
		cdns[row[region]] = Cdn{
			Path:  row[path],
			Hosts: strings.Split(row[hosts], " "),
		}
	}
	return cdns, nil
}

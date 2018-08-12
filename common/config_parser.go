package common

import (
	"bufio"
	"io"
	"strings"
)

func parseConfig(r io.Reader) map[string]string {
	cfg := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		i := strings.Index(line, " = ")
		if i <= 0 {
			continue
		}
		cfg[line[0:i]] = line[i+3:]
	}
	return cfg
}

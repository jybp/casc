package common

import (
	"bufio"
	"io"
	"strings"

	"github.com/pkg/errors"
)

//TODO pkg encoding/csv
func parseCSV(r io.Reader) ([]map[string]string, error) {
	columns := []string{}
	csv := []map[string]string{}
	scanner := bufio.NewScanner(r)
	i := -1
	for scanner.Scan() {
		i++
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if i == 0 { //column names
			cols := strings.Split(line, "|")
			for _, col := range cols {
				idx := strings.Index(col, "!")
				if idx < 0 {
					return nil, errors.WithStack(errors.New("invalid csv"))
				}
				columns = append(columns, col[:idx])
			}
			continue
		}
		//rows
		row := map[string]string{}
		values := strings.Split(line, "|")
		if len(columns) != len(values) {
			return nil, errors.WithStack(errors.New("invalid csv"))
		}
		for idx, value := range values {
			row[columns[idx]] = value
		}
		csv = append(csv, row)
	}
	return csv, nil
}

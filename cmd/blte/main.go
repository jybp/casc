/*
blte decode blte files from the command-line.
Usage:
	blte [-suffix <suffix>] [path ...]
Examples
	blte -suffix "_blte_decoded" ./blte_files/*
*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jybp/casc/blte"
)

func main() {
	var suffix string
	flag.StringVar(&suffix, "suffix", "_decoded", "")
	flag.Parse()
	pattern := flag.Arg(0)
	matches, err := filepath.Glob(pattern)
	if err != nil || (pattern == "" && len(matches) == 0) {
		flag.PrintDefaults()
		return
	}
	for _, m := range matches {
		b, err := ioutil.ReadFile(m)
		if err != nil {
			fmt.Printf("cannot read file %s: %+v\n", m, err)
		}
		buf := bytes.NewBuffer([]byte{})
		if err := blte.Decode(bytes.NewBuffer(b), buf); err != nil {
			fmt.Printf("cannot decode file %s: %+v\n", m, err)
			continue
		}
		if err := ioutil.WriteFile(m+suffix, buf.Bytes(), 0700); err != nil {
			fmt.Printf("cannot write decoded file %s: %+v\n", m, err)
			continue
		}
		fmt.Printf("%s\n", m+suffix)
	}
	fmt.Printf("%d files processed\n", len(matches))
}

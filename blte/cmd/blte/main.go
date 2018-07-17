package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jybp/go-d3-auto-parser/blte"
)

func main() {
	flag.Parse()
	pattern := flag.Arg(0)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		flag.PrintDefaults()
		return
	}

	for _, m := range matches {
		b, err := ioutil.ReadFile(m)
		if err != nil {
			fmt.Printf("cannot read file %s: %+v", m, err)
		}
		buf := bytes.NewBuffer([]byte{})
		if err := blte.Decode(bytes.NewBuffer(b), buf); err != nil {
			fmt.Printf("cannot decode file %s: %+v", m, err)
		}
		if err := ioutil.WriteFile(m+"_blte_decoded", buf.Bytes(), 0700); err != nil {
			fmt.Printf("cannot write decoded file %s: %+v", m, err)
		}
	}
	fmt.Printf("%d files processed", len(matches))
}

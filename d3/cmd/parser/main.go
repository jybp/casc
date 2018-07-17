package main

import (
	"log"

	"github.com/jybp/go-d3-auto-parser/d3"
)

func main() {
	if err := d3.Parse(); err != nil {
		log.Printf("%+v", err)
	}
}

package main

import (
	"log"

	"github.com/jybp/go-d3-auto-parser/d3"
)

func main() {
	err := d3.Parse()
	log.Printf("%+v", err)
}

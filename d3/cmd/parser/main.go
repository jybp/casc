package main

import (
	"log"

	"github.com/jybp/go-d3-auto-parser/d3"
)

func main() {
	apps := []string{"d3" /*, "hero", "hsb", "pro", "s1", "s2", "w3", "wow"*/}
	for _, app := range apps {
		if err := d3.Parse(app); err != nil {
			log.Printf("%+v", err)
		}
	}
}

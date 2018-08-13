/*
casclist lists file names from the command-line.
Usage:
	casclist <app> [-region <region>]
Examples
	casclist d3 -region eu
*/
package main

import (
	"flag"
	"fmt"

	"github.com/jybp/casc"
	"github.com/jybp/casc/downloader"
)

func main() {
	var region, app string
	flag.StringVar(&app, "app", "", "program code")
	flag.StringVar(&region, "region", casc.RegionUS, "region code")
	flag.Parse()
	if app == "" {
		flag.Usage()
		return
	}
	fmt.Printf("%s (%s) files:\n", app, region)
	storage := casc.Storage{
		App:        app,
		Region:     region,
		Downloader: downloader.FileCache{CacheDir: "cache"}, //TODO should use downloader.HTTP
	}
	filenames, err := storage.Files()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	for _, filename := range filenames {
		fmt.Printf("%s\n", filename)
	}
}

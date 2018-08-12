package main

import (
	"fmt"

	"github.com/jybp/casc"
	"github.com/jybp/casc/downloader"
)

func main() {
	apps := []string{casc.Diablo3}
	for _, app := range apps {
		fmt.Printf("%s files:\n", app)
		storage := casc.Storage{
			App:        app,
			Region:     casc.RegionUS,
			Downloader: downloader.FileCache{CacheDir: "cache"},
		}
		filenames, err := storage.Files()
		if err != nil {
			fmt.Printf("%+v\n", err)
			continue
		}
		for _, filename := range filenames {
			fmt.Printf("%s\n", filename)
		}
		fmt.Printf("\n")
	}
}

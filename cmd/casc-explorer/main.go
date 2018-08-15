/*
casc-explorer explore CASC files from the command-line.
Usage:
	casc-explorer (-dir <install-dir> | -app <app> [-region <region>] [-cdn <cdn>] [-cache <cache-dir>]) [-v]
Examples
	casc-explorer d3 -region eu -cdn eu
	casc-explorer /Applications/Diablo III/
*/
package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/jybp/casc"
)

// track prints if the cache is missed
type track struct {
	httpcache.Cache
}

func (t track) Get(key string) (responseBytes []byte, ok bool) {
	b, ok := t.Cache.Get(key)
	if !ok {
		fmt.Printf("cache missed %s\n ", key)
	}
	return b, ok
}

func main() {
	var installDir, app, region, cdn, cacheDir string
	var verbose bool
	flag.StringVar(&installDir, "dir", "", "game install directory")
	flag.StringVar(&app, "app", "", "app code")
	flag.StringVar(&region, "region", casc.RegionUS, "app region code")
	flag.StringVar(&cdn, "cdn", casc.RegionUS, "cdn region")
	flag.StringVar(&cacheDir, "cache-dir", "cache", "cache directory")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.Parse()
	if (app == "") == (installDir == "") {
		flag.Usage()
		return
	}

	var explorer *casc.Explorer
	if installDir != "" {
		if verbose {
			fmt.Printf("local with install dir: %s\n", installDir)
		}
		var err error
		explorer, err = casc.NewLocalExplorer(installDir)
		if err != nil {
			fmt.Printf("%+v\n", err)
			return
		}
	} else {
		if verbose {
			fmt.Printf("online with app: %s, region: %s, cdn region: %s, cache dir: %s\n",
				app, region, cdn, cacheDir)
		}
		var cache httpcache.Cache = diskcache.New(cacheDir)
		if verbose {
			cache = track{cache}
		}
		var err error
		explorer, err = casc.NewOnlineExplorer(app, region, cdn,
			&http.Client{Transport: httpcache.NewTransport(cache)})
		if err != nil {
			fmt.Printf("%+v\n", err)
			return
		}
	}

	fmt.Printf("%s (%s) files:\n", app, region)
	filenames, err := explorer.Files()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	for _, filename := range filenames {
		fmt.Printf("%s\n", filename)
	}
}

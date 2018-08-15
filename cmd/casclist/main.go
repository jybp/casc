/*
casclist lists file names from the command-line.
Usage:
	casclist -app <app> [-region <region>] [-cdn <cdn>] [-v]
Examples
	casclist d3 -region eu -cdn eu
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
	var app, region, cdn string
	var verbose bool
	flag.StringVar(&app, "app", "", "app code")
	flag.StringVar(&region, "region", casc.RegionUS, "app region code")
	flag.StringVar(&cdn, "cdn", casc.RegionUS, "cdn region")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.Parse()
	if app == "" {
		flag.Usage()
		return
	}
	var cache httpcache.Cache = diskcache.New("cache")
	if verbose {
		cache = track{cache}
	}
	explorer, err := casc.NewOnlineExplorer(app, region, cdn,
		&http.Client{Transport: httpcache.NewTransport(cache)})
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
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

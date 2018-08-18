/*
casc-explorer explore CASC files from the command-line.
Usage:
	casc-explorer (-dir <install-dir> | -app <app> [-cache <cache-dir>] [-region <region>] [-cdn <cdn>]) [-v]
Examples
	casc-explorer -app d3 -region eu -cdn eu -cache /tmp/casc
	casc-explorer -dir /Applications/Diablo III/
*/
package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/jybp/casc"
)

func main() {
	defer func(start time.Time) { fmt.Printf("%s\n", time.Since(start)) }(time.Now())
	var installDir, app, cacheDir, region, cdn string
	var verbose bool
	flag.StringVar(&installDir, "dir", "", "game install directory")
	flag.StringVar(&app, "app", "", "app code")
	flag.StringVar(&cacheDir, "cache", "/tmp/casc", "cache directory")
	flag.StringVar(&region, "region", casc.RegionUS, "app region code")
	flag.StringVar(&cdn, "cdn", casc.RegionUS, "cdn region")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.Parse()
	if ((app == "") == (installDir == "")) || (app != "" && cacheDir == "") {
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
		transport := http.DefaultTransport
		if verbose {
			transport = logTransport{}
			fmt.Printf("online with app: %s, region: %s, cdn region: %s, cache dir: %s\n",
				app, region, cdn, cacheDir)
		}

		client := &http.Client{Transport: &Transport{
			Dir:       cacheDir,
			Transport: transport,
			Filter:    filter,
		}}
		var err error
		explorer, err = casc.NewOnlineExplorer(app, region, cdn, client)
		if err != nil {
			fmt.Printf("%+v\n", err)
			return
		}
	}

	fmt.Printf("version: %s:\n", explorer.Version())
	filenames, err := explorer.Files()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	for _, filename := range filenames {
		fmt.Printf("%s\n", filename)
	}
	fmt.Printf("%d entries\n", len(filenames))
}

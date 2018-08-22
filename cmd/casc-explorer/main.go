/*
casc-explorer explore CASC files from the command-line.
Usage:
	casc-explorer (-dir <install-dir> | -app <app> [-cache <cache-dir>] [-region <region>] [-cdn <cdn>]) [-v]
Examples:
	casc-explorer -app d3 -region eu -cdn eu -cache /tmp/casc
	casc-explorer -dir /Applications/Diablo III/
*/
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jybp/casc"
	"github.com/jybp/httpcache"
	"github.com/jybp/httpcache/disk"
)

type logTransport struct{}

func (logTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	fmt.Printf("http call (%s) %s\n", r.Method, r.URL)
	return http.DefaultTransport.RoundTrip(r)
}

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
		client := &http.Client{Transport: &httpcache.Transport{
			Transport: transport,
			Filter: func(r *http.Request) bool {
				return strings.Contains(r.URL.String(), "patch.battle.net")
			},
			Cache: disk.Cache{Dir: cacheDir},
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
	filesCount := len(filenames)

	resultDir := "online"
	if installDir != "" {
		resultDir = "local"
	}
	dir := filepath.Join("", resultDir, explorer.App(), explorer.Version())
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0777); err != nil {
			fmt.Printf("cannot create dir %s: %s\n", dir, err.Error())
			return
		}
	}
	extracted := 0
	for _, filename := range filenames {
		fmt.Printf("%d: extracting %s\n", extracted, filename)
		b, err := explorer.Extract(filename)
		if err != nil {
			fmt.Printf("cannot extract %s: %+v\n", filename, err)
			continue
		}
		fullname := filepath.Join(dir, filename)
		if _, err := os.Stat(filepath.Dir(fullname)); err != nil {
			if err := os.MkdirAll(filepath.Dir(fullname), 0777); err != nil {
				fmt.Printf("cannot create dir %s: %+v\n", filepath.Dir(fullname), err)
				return
			}
		}
		if err := ioutil.WriteFile(fullname, b, 0666); err != nil {
			fmt.Printf("cannot write file %s: %+v\n", fullname, err)
			continue
		}
		extracted++
	}
	fmt.Printf("%d extracted from %d files\n", extracted, filesCount)
}

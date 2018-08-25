/*
casc-extract explore CASC files from the command-line.
Usage:
	casc-extract (-dir <install-dir> | -app <app> [-region <region>] [-cache <cache-dir>] [-cdn <cdn>]) [-pattern <pattern>] ([-o <output-dir>] | -list) [-v]
Examples:
	casc-extract -app d3 -region us -cdn us -pattern "enUS/Data_D3/Locale/enUS/Cutscenes/*.ogv"
	casc-extract -dir /Applications/Diablo III/ -pattern "enUS/Data_D3/Locale/enUS/Cutscenes/*.ogv"
*/
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jybp/casc"
	"github.com/jybp/casc/common"
	"github.com/jybp/httpcache"
	"github.com/jybp/httpcache/disk"
)

type logTransport struct{}

func (logTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	fmt.Printf("http call (%s) %s\n", r.Method, r.URL)
	return http.DefaultTransport.RoundTrip(r)
}

func main() {
	// Flags
	var installDir, app, cacheDir, region, cdn, pattern, outputDir string
	var list, verbose bool
	flag.StringVar(&installDir, "dir", "", "game install directory")
	flag.StringVar(&app, "app", "", "app code")
	flag.StringVar(&cacheDir, "cache", "/tmp/casc", "cache directory")
	flag.StringVar(&region, "region", common.RegionEU, "app region code")
	flag.StringVar(&cdn, "cdn", common.RegionEU, "cdn region")
	flag.StringVar(&pattern, "pattern", "", "filenames matching the pattern will be extracted\n"+
		"       https://golang.org/pkg/path/#Match")
	flag.StringVar(&outputDir, "o", "", "output directory for extracted files")
	flag.BoolVar(&list, "list", false, "do not extract files but list them")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.Parse()
	if ((app == "") == (installDir == "")) ||
		(app != "" && cacheDir == "") ||
		(outputDir != "" && list) {
		fmt.Println("casc-extract (-dir <install-dir> | -app <app> [-region <region>]" +
			" [-cache <cache-dir>] [-cdn <cdn>]) [-pattern <pattern>] ([-o <output-dir>] | -list) [-v]")
		flag.Usage()
		return
	}
	if _, err := path.Match(pattern, ""); err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	var errFmt = "%s"
	if verbose {
		common.Wlog = os.Stdout
		errFmt = "%+v"
	}
	defer func(start time.Time) { fmt.Fprintf(common.Wlog, "%s\n", time.Since(start)) }(time.Now())

	// Explorer
	var explorer *casc.Explorer
	if installDir != "" {
		fmt.Fprintf(common.Wlog, "local with install dir: %s\n", installDir)
		var err error
		explorer, err = casc.NewLocalExplorer(installDir)
		if err != nil {
			fmt.Printf(errFmt+"\n", err)
			return
		}
	} else {
		transport := http.DefaultTransport
		if verbose {
			transport = logTransport{}
		}
		fmt.Fprintf(common.Wlog, "online with app: %s, region: %s, cdn region: %s, cache dir: %s\n",
			app, region, cdn, cacheDir)
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
			fmt.Printf(errFmt+"\n", err)
			return
		}
	}

	// Pattern
	fmt.Fprintf(common.Wlog, "version: %s:\n", explorer.Version())
	all, err := explorer.Files()
	if err != nil {
		fmt.Printf(errFmt+"\n", err)
		return
	}
	filtered := all
	if pattern != "" {
		matches := []string{}
		for _, filename := range all {
			if ok, _ := path.Match(pattern, filename); !ok {
				continue
			}
			matches = append(matches, filename)
		}
		fmt.Fprintf(common.Wlog, "%d out of %d files matched pattern %s\n", len(matches), len(all), pattern)
		filtered = matches
	}
	// List
	if list {
		for _, filename := range filtered {
			fmt.Printf("%s\n", filename)
		}
		return
	}
	// Extract
	if outputDir == "" {
		outputDir = filepath.Join(explorer.App(), explorer.Version())
	}
	if _, err := os.Stat(outputDir); err != nil {
		if err := os.MkdirAll(outputDir, 0777); err != nil {
			fmt.Printf("cannot create dir %s: "+errFmt+"\n", outputDir, err.Error())
			return
		}
	}
	extracted := 0
	for i, filename := range filtered {
		fullname := filepath.Join(outputDir, filename)
		fmt.Printf("%d/%d: %s\n", i+1, len(filtered), fullname)
		b, err := explorer.Extract(filename)
		if err != nil {
			fmt.Printf("cannot extract %s: "+errFmt+"\n", filename, err)
			continue
		}
		if _, err := os.Stat(filepath.Dir(fullname)); err != nil {
			if err := os.MkdirAll(filepath.Dir(fullname), 0777); err != nil {
				fmt.Printf("cannot create dir %s: "+errFmt+"\n", filepath.Dir(fullname), err)
				return
			}
		}
		if err := ioutil.WriteFile(fullname, b, 0666); err != nil {
			fmt.Printf("cannot write file %s: "+errFmt+"\n", fullname, err)
			continue
		}
		extracted++
	}
	fmt.Fprintf(common.Wlog, "extracted %d out of %d filtered files (total: %d)\n", extracted, len(filtered), len(all))
}

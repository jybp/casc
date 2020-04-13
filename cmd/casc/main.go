/*
Explore CASC files from the command-line.
Usage:
	casc (-dir <install-dir> | -app <app> [-region <region>] [-cdn <cdn>]) [-o <output-dir>] [-v]
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"github.com/pkg/errors"
	"github.com/jybp/casc"
)

type logTransport struct{}

func (logTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	h := r.Header.Get("Range")
	if len(h) > 0 {
		log.Printf("%s (Range: %s) %s\n", r.Method, h, r.URL)
	} else {
		log.Printf("%s %s\n", r.Method, r.URL)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run() error {
	var installDir, app, region, cdn, outputDir string
	var verbose bool
	flag.StringVar(&installDir, "dir", "", "game install directory")
	flag.StringVar(&app, "app", "", "app code")
	flag.StringVar(&region, "region", casc.RegionUS, "app region code")
	flag.StringVar(&cdn, "cdn", casc.RegionUS, "cdn region")
	flag.StringVar(&outputDir, "o", "", "output directory for extracted files")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.Parse()

	if (len(app) == 0) == (len(installDir) == 0) {
		flag.Usage()
		return nil
	}

	client := http.DefaultClient
	if verbose {
		client.Transport = logTransport{}
	}

	var explorer *casc.Explorer
	var err error
	if len(installDir) > 0 {
		explorer, err = casc.Local(installDir)
	} else {
		explorer, err = casc.Online(app, region, cdn, client)
	}
	if err != nil {
		return err
	}

	stat, _ := os.Stdin.Stat()
	list := (stat.Mode() & os.ModeCharDevice) != 0

	if list {
		all, err := explorer.Files()
		if err != nil {
			return err
		}
		for _, filename := range all {
			fmt.Printf("%s\n", filename)
		}
		return nil
	}

	stdin := bufio.NewReader(os.Stdin)
	for {
		line, _, err := stdin.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.WithStack(err)
		}
		filename := filepath.Base(string(line))
		fullpath := filepath.Join(outputDir, filename)
		b, err := explorer.Extract(string(line))
		if err == casc.ErrNotFound {
			continue
		}
		if err != nil {
			return errors.WithStack(err)
		}
		if err := ioutil.WriteFile(fullpath, b, 0666); err != nil {
			return errors.WithStack(err)
		}
		log.Printf("%s\n", fullpath)
	}
	return nil
}

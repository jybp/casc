package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/jybp/casc"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}

func run() error {
	var app, region, cdn string
	flag.StringVar(&app, "app", "", "app code")
	flag.StringVar(&region, "region", casc.RegionUS, "app region code")
	flag.StringVar(&cdn, "cdn", casc.RegionUS, "cdn region")
	flag.Parse()
	if len(app) == 0 {
		flag.Usage()
		return nil
	}

	storage, err := casc.NewOnlineStorage(app, region, cdn, http.DefaultClient)
	if err != nil {
		return err
	}
	fmt.Printf("%s version %s found", storage.App(), storage.Version())
	return nil
}

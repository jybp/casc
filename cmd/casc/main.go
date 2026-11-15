/*
Explore CASC files from the command-line.
Usage:

	casc (-dir <install-dir> | -app <app> [-region <region>] [-cdn <cdn>])
	     [-l | -x] [-pattern <regexp>] [-preset <name>] [-o <output-dir>] [-j <workers>] [-skip-existing] [-v]

Without -l or -x, files are listed when stdin is a terminal, otherwise the
filenames read from stdin are extracted.
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jybp/casc"
	"github.com/jybp/casc/common"
	"github.com/jybp/casc/d2r"
	"github.com/pkg/errors"
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
	var installDir, app, region, cdn, outputDir, pattern, presetNames string
	var verbose, list, extract, skipExisting, flat, noRaw bool
	var convert string
	var workers int
	flag.StringVar(&installDir, "dir", "", "game install directory")
	flag.StringVar(&app, "app", "", "app code (d3, osi, s1, w3)")
	flag.StringVar(&region, "region", casc.RegionUS, "app region code")
	flag.StringVar(&cdn, "cdn", casc.RegionUS, "cdn region")
	flag.StringVar(&outputDir, "o", "", "output directory for extracted files (default \"extract\" with -x)")
	flag.StringVar(&pattern, "pattern", "", "only keep files matching this regular expression (case insensitive)")
	flag.StringVar(&presetNames, "preset", "", "comma separated list of file groups, see -presets")
	flag.BoolVar(&list, "l", false, "list files (default when stdin is a terminal)")
	flag.BoolVar(&extract, "x", false, "extract every matching file")
	flag.BoolVar(&skipExisting, "skip-existing", false, "do not extract files already present in the output directory")
	flag.BoolVar(&flat, "flat", false, "write files directly inside the output directory instead of keeping the CASC folders")
	flag.StringVar(&convert, "convert", "", "also convert D2R .texture files: dds (lossless, all mipmaps), png (largest mipmap) or both")
	flag.BoolVar(&noRaw, "no-raw", false, "with -convert, do not write the original .texture files")
	flag.IntVar(&workers, "j", 8, "number of files extracted in parallel")
	flag.BoolVar(&verbose, "v", false, "verbose")
	showPresets := flag.Bool("presets", false, "print the available presets")
	flag.Parse()

	if *showPresets {
		printPresets()
		return nil
	}
	if (len(app) == 0) == (len(installDir) == 0) {
		flag.Usage()
		return nil
	}

	filter, err := newFilter(pattern, presetNames)
	if err != nil {
		return err
	}

	client := http.DefaultClient
	if verbose {
		common.Wlog = os.Stdout
		client.Transport = logTransport{}
	}

	var explorer *casc.Explorer
	if len(installDir) > 0 {
		explorer, err = casc.Local(installDir)
	} else {
		explorer, err = casc.Online(app, region, cdn, client)
	}
	if err != nil {
		return err
	}

	if !list && !extract {
		stat, _ := os.Stdin.Stat()
		list = (stat.Mode() & os.ModeCharDevice) != 0
	}

	var filenames []string
	if list || extract {
		all, err := explorer.Files()
		if err != nil {
			return err
		}
		for _, filename := range all {
			if filter(filename) {
				filenames = append(filenames, filename)
			}
		}
	} else {
		stdin := bufio.NewReader(os.Stdin)
		for {
			line, err := stdin.ReadString('\n')
			if line = strings.TrimSpace(line); line != "" && filter(line) {
				filenames = append(filenames, line)
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return errors.WithStack(err)
			}
		}
	}

	if list {
		w := bufio.NewWriter(os.Stdout)
		for _, filename := range filenames {
			fmt.Fprintln(w, filename)
		}
		return w.Flush()
	}

	if outputDir == "" && extract {
		outputDir = "extract"
	}
	opts := extractOptions{
		outputDir:    outputDir,
		workers:      workers,
		skipExisting: skipExisting,
		flat:         flat,
		noRaw:        noRaw,
	}
	switch strings.ToLower(convert) {
	case "":
	case "dds":
		opts.convertDDS = true
	case "png":
		opts.convertPNG = true
	case "both":
		opts.convertDDS, opts.convertPNG = true, true
	default:
		return errors.Errorf("invalid -convert %q: expected dds, png or both", convert)
	}
	return extractAll(explorer, filenames, opts)
}

// localPath converts a CASC filename to a path safe to write on disk.
// The ':' separating nested TVFS directories is invalid on Windows.
func localPath(outputDir, filename string, flat bool) string {
	name := strings.ReplaceAll(filename, ":", "/")
	if flat {
		return filepath.Join(outputDir, filepath.Base(filepath.FromSlash(name)))
	}
	var parts []string
	for _, p := range strings.Split(name, "/") {
		if p == "" || p == "." || p == ".." {
			continue
		}
		parts = append(parts, p)
	}
	return filepath.Join(append([]string{outputDir}, parts...)...)
}

type extractOptions struct {
	outputDir    string
	workers      int
	skipExisting bool
	flat         bool
	convertDDS   bool // write .texture files as .dds too
	convertPNG   bool // write .texture files as .png too
	noRaw        bool // do not write the original file when it was converted
}

// outputs returns the files written for filename.
func (o extractOptions) outputs(filename string) (raw string, dds string, png string) {
	raw = localPath(o.outputDir, filename, o.flat)
	if strings.HasSuffix(strings.ToLower(filename), ".texture") {
		base := strings.TrimSuffix(raw, filepath.Ext(raw))
		if o.convertDDS {
			dds = base + ".dds"
		}
		if o.convertPNG {
			png = base + ".png"
		}
	}
	return
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeFile(path string, write func(io.Writer) error) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return errors.WithStack(err)
	}
	f, err := os.Create(path)
	if err != nil {
		return errors.WithStack(err)
	}
	w := bufio.NewWriterSize(f, 1<<20)
	if err = write(w); err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
	}
	return errors.WithStack(err)
}

func extractAll(explorer *casc.Explorer, filenames []string, opts extractOptions) error {
	if opts.workers < 1 {
		opts.workers = 1
	}
	var done, failed, skipped, notFound, converted, convertFailed int64
	var bytesWritten int64
	total := len(filenames)
	jobs := make(chan string)
	var mu sync.Mutex
	var failures []string
	fail := func(filename string, err error) {
		mu.Lock()
		failures = append(failures, fmt.Sprintf("%s: %v", filename, errors.Cause(err)))
		mu.Unlock()
	}
	var wg sync.WaitGroup
	for i := 0; i < opts.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filename := range jobs {
				raw, dds, png := opts.outputs(filename)
				writeRaw := !opts.noRaw || (dds == "" && png == "")
				if opts.skipExisting &&
					(!writeRaw || exists(raw)) && (dds == "" || exists(dds)) && (png == "" || exists(png)) {
					atomic.AddInt64(&skipped, 1)
					continue
				}
				b, err := explorer.Extract(filename)
				if errors.Cause(err) == casc.ErrNotFound {
					// i.e. a locale or optional content which is not installed.
					atomic.AddInt64(&notFound, 1)
					continue
				}
				if err == nil && writeRaw {
					err = writeFile(raw, func(w io.Writer) error { _, err := w.Write(b); return err })
				}
				if err != nil {
					atomic.AddInt64(&failed, 1)
					fail(filename, err)
					continue
				}
				if dds != "" || png != "" {
					if err := convertTexture(b, filename, dds, png); err != nil {
						atomic.AddInt64(&convertFailed, 1)
						fail(filename, errors.Wrap(err, "conversion"))
						if !writeRaw { // keep the data
							writeFile(raw, func(w io.Writer) error { _, err := w.Write(b); return err })
						}
					} else {
						atomic.AddInt64(&converted, 1)
					}
				}
				atomic.AddInt64(&bytesWritten, int64(len(b)))
				n := atomic.AddInt64(&done, 1)
				log.Printf("[%d/%d] %s\n", n, total, raw)
			}
		}()
	}
	for _, filename := range filenames {
		jobs <- filename
	}
	close(jobs)
	wg.Wait()

	sort.Strings(failures)
	for _, f := range failures {
		log.Printf("failed: %s\n", f)
	}
	log.Printf("%d extracted (%.1f MB), %d skipped, %d not available locally, %d failed\n",
		done, float64(bytesWritten)/(1<<20), skipped, notFound, failed)
	if opts.convertDDS || opts.convertPNG {
		log.Printf("%d textures converted, %d conversions failed\n", converted, convertFailed)
	}
	return nil
}

func convertTexture(b []byte, filename, dds, png string) error {
	if !d2r.IsTexture(b) {
		return errors.New("not a texture")
	}
	tex, err := d2r.ParseTexture(b)
	if err != nil {
		return err
	}
	if dds != "" {
		if err := writeFile(dds, tex.WriteDDS); err != nil {
			return err
		}
	}
	if png != "" {
		normal := d2r.IsNormalMapName(filename)
		if err := writeFile(png, func(w io.Writer) error { return tex.WritePNG(w, normal) }); err != nil {
			return err
		}
	}
	return nil
}

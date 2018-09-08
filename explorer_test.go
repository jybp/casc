package casc

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"strings"
	"testing"

	"github.com/jybp/casc/common"
)

var slow = flag.Bool("slow", false, "run slow tests")
var update = flag.Bool("update", false, "update slow tests")
var app = flag.String("app", "", "apps to test")

func TestExtract(t *testing.T) {
	if !*slow {
		fmt.Println("skip: no slow flag")
		return
	}
	appsToInstallDir := map[string]string{
		common.Diablo3:    "/Applications/Diablo III", //TODO .ogg != .sbk for */SoundBank/*
		common.Starcraft1: "/Applications/StarCraft",
		common.Warcraft3:  "/Applications/Warcraft III",
	}
	if *app != "" {
		installDir, ok := appsToInstallDir[*app]
		if !ok {
			t.Fatalf("app %s not found", *app)
		}
		appsToInstallDir = map[string]string{*app: installDir}
	}
	for appCode, installDir := range appsToInstallDir {
		if *update {
			testUpdate(t, appCode, installDir)
		} else {
			testExtractApp(t, appCode, installDir)
		}
	}
}

func testExtractApp(t *testing.T, app, installDir string) {
	if testing.Verbose() {
		t.Logf("%s: %s\n", app, installDir)
	}
	files, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	cascFilename := ""
	selfFilename := ""
	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "casclib-"+app+"-") {
			cascFilename = file.Name()
		} else if strings.HasPrefix(file.Name(), app+"-") {
			selfFilename = file.Name()
		}
	}
	if cascFilename == "" || selfFilename == "" {
		t.Fatalf("file not found: %s; %s", cascFilename, selfFilename)
	}

	cascLib := testLoadFile(t, fmt.Sprintf("testdata/%s", cascFilename))
	self := testLoadFile(t, fmt.Sprintf("testdata/%s", selfFilename))
	selfmap := map[string]struct{}{} // performance
	for _, s := range self {
		selfmap[s] = struct{}{}
	}
	for i := 0; i < len(cascLib); i++ {
		if strings.TrimSpace(cascLib[i]) == "" {
			continue
		}
		size, err := strconv.Atoi(cascLib[i][strings.LastIndex(cascLib[i], " ")+1:])
		if err != nil {
			t.Fatalf("invalid size %s: %+v", cascLib[i], err)
		}
		if size == 0 || size == 64 {
			continue //ignore cascLib folders
		}
		if testing.Verbose() {
			fmt.Printf("\r%d/%d", i+1, len(cascLib))
		}

		if _, ok := selfmap[cascLib[i]]; ok {
			continue
		}

		// casclib extracts these common Warcraft 3 files.
		commonFiles := []string{}
		if app == common.Warcraft3 {
			commonFiles = []string{"download", "encoding", "install", "root", "index"}
		}
		// casclib extracts these common Diablo 3 files.
		if app == common.Diablo3 {
			commonFiles = []string{"download", "encoding", "install", "root"}
		}
		if len(commonFiles) > 0 {
			found := false
			for _, f := range commonFiles {
				if strings.HasPrefix(cascLib[i], f) {
					found = true
					break
				}
			}
			if found {
				continue
			}
		}

		//TODO casclib doesn't convert sbk extension for Diablo 3?
		if app == common.Diablo3 {
			idxDot := strings.LastIndex(cascLib[i], ".")
			if idxDot > 0 {
				ext := cascLib[i][idxDot+1 : idxDot+4]
				if ext == "sbk" {
					found := false
					for _, s := range self { // slow
						if len(s) < idxDot+4 {
							continue
						}
						if s[:idxDot+1] == cascLib[i][:idxDot+1] &&
							s[idxDot+4:] == cascLib[i][idxDot+4:] {
							found = true
						}
					}
					if found {
						continue
					}
				}
			}
		}
		t.Errorf("\n%s\n", cascLib[i])
	}
	if testing.Verbose() {
		fmt.Print("\n")
	}
}

func testLoadFile(t *testing.T, filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, strings.ToLower(scanner.Text()))
	}
	return lines
}

func testUpdate(t *testing.T, app, installDir string) {
	dir, close := testTempDir(t)
	defer close()
	explorer, err := NewLocalExplorer(installDir)
	if err != nil {
		t.Fatal(err)
	}
	if testing.Verbose() {
		fmt.Printf("updating %s (%s) to %s...\n", app, explorer.Version(), dir)
	}
	files, err := explorer.Files()
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range files {
		if testing.Verbose() {
			fmt.Printf("\r%d/%d", i+1, len(files))
		}
		b, err := explorer.Extract(f)
		if err != nil {
			t.Errorf("error extracting %s: %s", f, err.Error())
			continue
		}
		// force backslashes like casclib
		fo := filepath.Join(dir, strings.Replace(f, "/", "\\", -1))
		if err := ioutil.WriteFile(fo, b, 0777); err != nil {
			t.Errorf("error writing %s: %s", fo, err.Error())
		}
	}
	cmd := exec.Command("bash", "-c", "ls -ls | awk '{print $10,$6}'")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	listfile := filepath.Join("testdata", app+"-"+explorer.Version())
	if err := ioutil.WriteFile(listfile, out, 0777); err != nil {
		t.Fatal(err)
	}
}

func testTempDir(t *testing.T) (string, func()) {
	t.Helper()
	d, err := ioutil.TempDir("", "casc-StarCraft")
	if err != nil {
		t.Fatal(err)
	}
	return d, func() {
		if err := os.RemoveAll(d); err != nil {
			t.Fatal(err)
		}
	}
}

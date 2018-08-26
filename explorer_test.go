package casc

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"strings"
	"testing"

	"github.com/jybp/casc/common"
)

var slow = flag.Bool("slow", false, "run slow tests")
var update = flag.Bool("update", false, "update slow tests")
var app = flag.String("app", "", "apps to test")

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

func TestExtract(t *testing.T) {
	if !*slow {
		fmt.Println("skip: no slow flag")
		return
	}

	appsToInstallDir := map[string]string{
		common.Starcraft1: "/Applications/StarCraft",
		common.Warcraft3:  "/Applications/Warcraft III",
	}
	if *app != "" {
		installDir, ok := appsToInstallDir[*app]
		if !ok {
			t.Fatalf("app %s not found", *app)
		}
		appsToInstallDir = map[string]string{
			*app: installDir,
		}
	}
	for appCode, installDir := range appsToInstallDir {
		if testing.Verbose() {
			fmt.Printf("%s: %s\n", appCode, installDir)
		}
		if *update {
			testUpdate(t, appCode, installDir)
		}
		cascLib := testLoadFile(t, fmt.Sprintf("testdata/casclib-%s", appCode))
		self := testLoadFile(t, fmt.Sprintf("testdata/%s", appCode))
		skipped := 0
		for i := 0; i < len(cascLib); i++ {
			if !strings.Contains(cascLib[i], ".") { //ignore cascLib folders
				skipped++
				continue
			}
			if testing.Verbose() {
				fmt.Printf("\r%d/%d", i+1, len(cascLib))
			}
			if !stringInSlice(cascLib[i], self) {
				t.Errorf("\n%s\n", cascLib[i])
			}
		}
		if testing.Verbose() {
			fmt.Printf("\n%d skipped\n", skipped)
		}
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func testUpdate(t *testing.T, app, installDir string) {
	if testing.Verbose() {
		fmt.Printf("updating %s...\n", app)
	}
	dir, close := testTempDir(t)
	defer close()

	explorer, err := NewLocalExplorer(installDir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := explorer.Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := explorer.Extract(f)
		if err != nil {
			t.Errorf("error extracting %s: %s", f, err.Error())
			continue
		}
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
	listfile := filepath.Join("testdata", app)
	if err := ioutil.WriteFile(listfile, out, 0777); err != nil {
		t.Fatal(err)
	}
}

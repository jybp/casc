package tvfs

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

// TestD2RVfsRoot parses the vfs-root of Diablo II Resurrected 3.4.95032.
// The nested vfs-2 directory is not included (11MB) and is skipped.
func TestD2RVfsRoot(t *testing.T) {
	rootB, err := ioutil.ReadFile("testdata/d2r-vfs-root.bin")
	if err != nil {
		t.Fatal(err)
	}
	rootKey, _ := hex.DecodeString("0d979ceb22075129b7805d98d7499342")
	fetch := func(h []byte) ([]byte, error) {
		if bytes.Equal(h[:9], rootKey[:9]) {
			return rootB, nil
		}
		if p := os.Getenv("TVFS_D2R_VFS2"); p != "" {
			return ioutil.ReadFile(p)
		}
		return nil, os.ErrNotExist
	}
	vfs2, _ := hex.DecodeString("38bb8ed022fb3aff3612f2f30cf2f78d")
	var nested [][]byte
	if os.Getenv("TVFS_D2R_VFS2") != "" {
		nested = [][]byte{rootKey, vfs2}
	}
	r, err := NewRoot(rootKey, nested, fetch)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := r.Files()
	want := map[string]bool{
		"binaries/releaseanticheat/x86_64/d2r.exe":        false,
		"binaries/releaseanticheat/x86_64/d2r_loader.dll": false,
		"binaries/releaseanticheat/x86_64/nvngx_dlss.dll": false,
		"index": false,
	}
	for _, f := range files {
		if _, ok := want[f]; ok {
			want[f] = true
		}
		if strings.Contains(f, "texturesduriel") {
			t.Errorf("missing separator: %s", f)
		}
	}
	for f, found := range want {
		if !found {
			t.Errorf("%s not found in %v", f, files)
		}
	}
	t.Logf("%d files", len(files))
}

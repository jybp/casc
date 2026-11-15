/*
d2rconvert turns an extracted Diablo II Resurrected folder (casc -x -o <dir>) into
reference images and metadata.

	d2rconvert -in extract/d2r -out extract/reference [-only sprites,dc6,tiles,orm] [-j 8]

Outputs (under -out):

	sprites/<chars|monsters|objects>/<token>/<mode>_<weapon>.png   composited animation sheets
	sprites/.../<mode>_<weapon>.json                                frames, directions, hit frame, speed, box
	dc6/<path>.png                                                  UI / item / inventory sprites
	tiles/<act>/<set>/<name>.png (+ .json)                          legacy level tiles
	orm/<path>_{ao,roughness,metal}.png                             HD material channels split
	models/<path>.glb                                               HD meshes (+ skeleton, textures)
*/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jybp/casc/d2r"
	"github.com/jybp/casc/granny"
)

type job struct {
	kind string
	path string // input path, relative to the global or hd folder
}

type converter struct {
	in, out  string
	lods     string // "0", "all"
	textures string // "link", "embed", "none"
	skin     bool
	palettes map[string]*d2r.Palette
	done     int64
	failed   int64
	mu       sync.Mutex
	errs     []string
}

func main() {
	in := flag.String("in", "extract/d2r", "extracted D2R folder (contains data/data/...)")
	out := flag.String("out", "extract/reference", "output folder")
	only := flag.String("only", "sprites,dc6,tiles,orm,models", "comma separated: sprites, dc6, tiles, orm, models")
	lods := flag.String("lod", "0", "models: level of detail to convert (0 = most detailed) or \"all\"")
	skin := flag.Bool("skin", false, "models: export the skeleton and skinning weights (for posing/animation); the meshes are in their bind pose either way")
	textures := flag.String("textures", "link", "models: link (reference the PNGs written by casc -convert png), embed (self-contained .glb, large) or none")
	pattern := flag.String("pattern", "", "only convert inputs whose path contains this text")
	workers := flag.Int("j", 8, "parallel workers")
	flag.Parse()

	c := &converter{in: *in, out: *out, lods: *lods, textures: *textures, skin: *skin, palettes: map[string]*d2r.Palette{}}
	if err := c.loadPalettes(); err != nil {
		log.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, k := range strings.Split(*only, ",") {
		kinds[strings.TrimSpace(k)] = true
	}
	jobs, err := c.collect(kinds, *pattern)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d inputs\n", len(jobs))

	ch := make(chan job)
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				if err := c.run(j); err != nil {
					atomic.AddInt64(&c.failed, 1)
					c.mu.Lock()
					c.errs = append(c.errs, fmt.Sprintf("%s %s: %v", j.kind, j.path, err))
					c.mu.Unlock()
					continue
				}
				if n := atomic.AddInt64(&c.done, 1); n%500 == 0 {
					log.Printf("%d/%d\n", n, len(jobs))
				}
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
	sort.Strings(c.errs)
	for _, e := range c.errs {
		log.Printf("failed: %s\n", e)
	}
	log.Printf("%d converted, %d failed\n", c.done, c.failed)
}

func (c *converter) global(p ...string) string {
	return filepath.Join(append([]string{c.in, "data", "data", "global"}, p...)...)
}

func (c *converter) hd(p ...string) string {
	return filepath.Join(append([]string{c.in, "data", "data", "hd"}, p...)...)
}

func (c *converter) loadPalettes() error {
	dirs, err := ioutil.ReadDir(c.global("palette"))
	if err != nil {
		return err
	}
	for _, d := range dirs {
		b, err := ioutil.ReadFile(c.global("palette", d.Name(), "pal.dat"))
		if err != nil {
			continue
		}
		p, err := d2r.ParsePalette(b)
		if err != nil {
			return err
		}
		c.palettes[d.Name()] = p
	}
	if c.palettes["act1"] == nil {
		return fmt.Errorf("act1 palette not found under %s", c.global("palette"))
	}
	return nil
}

// palette picks the act palette for tiles, act1 otherwise (units are drawn with it).
func (c *converter) palette(path string) *d2r.Palette {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, p := range parts {
		if pal, ok := c.palettes[p]; ok && strings.HasPrefix(p, "act") {
			return pal
		}
	}
	return c.palettes["act1"]
}

func (c *converter) collect(kinds map[string]bool, pattern string) ([]job, error) {
	var jobs []job
	add := func(kind, root string, match func(path string) bool, rel func(string) string) error {
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !match(path) {
				return nil
			}
			r := rel(path)
			if pattern != "" && !strings.Contains(filepath.ToSlash(r), pattern) {
				return nil
			}
			jobs = append(jobs, job{kind: kind, path: r})
			return nil
		})
	}
	relGlobal := func(p string) string { r, _ := filepath.Rel(c.global(), p); return r }
	relHD := func(p string) string { r, _ := filepath.Rel(c.hd(), p); return r }
	ext := func(e string) func(string) bool {
		return func(p string) bool { return strings.EqualFold(filepath.Ext(p), e) }
	}
	if kinds["sprites"] {
		for _, cat := range []string{"chars", "monsters", "objects"} {
			if err := add("sprites", c.global(cat), ext(".cof"), relGlobal); err != nil {
				return nil, err
			}
		}
	}
	if kinds["dc6"] {
		if err := add("dc6", c.global(), ext(".dc6"), relGlobal); err != nil {
			return nil, err
		}
	}
	if kinds["tiles"] {
		if err := add("tiles", c.global("tiles"), ext(".dt1"), relGlobal); err != nil {
			return nil, err
		}
	}
	if kinds["orm"] {
		if err := add("orm", c.hd(), func(p string) bool {
			return strings.HasSuffix(strings.ToLower(p), "_orm.texture")
		}, relHD); err != nil {
			return nil, err
		}
	}
	if kinds["models"] {
		lodRe := func(p string) bool {
			n := strings.ToLower(filepath.Base(p))
			if !strings.HasSuffix(n, ".model") {
				return false
			}
			if c.lods == "all" {
				return true
			}
			i := strings.LastIndex(n, "_lod")
			return i < 0 || strings.TrimSuffix(n[i+4:], ".model") == c.lods
		}
		if err := add("models", c.hd(), lodRe, relHD); err != nil {
			return nil, err
		}
	}
	return jobs, nil
}

func (c *converter) run(j job) error {
	switch j.kind {
	case "sprites":
		return c.sprite(j.path)
	case "dc6":
		return c.dc6(j.path)
	case "tiles":
		return c.tiles(j.path)
	case "orm":
		return c.orm(j.path)
	case "models":
		return c.model(j.path)
	}
	return fmt.Errorf("unknown job %s", j.kind)
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeJSON(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(path, b, 0644)
}

func trimExt(p string) string { return strings.TrimSuffix(p, filepath.Ext(p)) }

// sprite composites <cat>/<token>/cof/<token><mode><wclass>.cof from its DCC layers.
func (c *converter) sprite(rel string) error {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 4 || parts[2] != "cof" {
		return fmt.Errorf("unexpected cof path")
	}
	cat, token := parts[0], parts[1]
	name := trimExt(strings.ToLower(parts[3]))
	if !strings.HasPrefix(name, token) || len(name) < len(token)+5 {
		return fmt.Errorf("unexpected cof name")
	}
	mode := name[len(token) : len(token)+2]
	wclass := name[len(token)+2:]
	b, err := ioutil.ReadFile(c.global(rel))
	if err != nil {
		return err
	}
	cof, err := d2r.ParseCOF(b)
	if err != nil {
		return err
	}
	layers := map[int]*d2r.Sprite{}
	styles := map[string]string{}
	for _, l := range cof.Layers {
		if l.Component >= len(d2r.Components) {
			continue
		}
		comp := d2r.Components[l.Component]
		dir := c.global(cat, token, comp)
		files, _ := ioutil.ReadDir(dir)
		suffix := mode + l.WeaponClass + ".dcc"
		var candidates []string
		for _, f := range files {
			n := strings.ToLower(f.Name())
			if strings.HasPrefix(n, token+comp) && strings.HasSuffix(n, suffix) && len(n) == len(token+comp)+3+len(suffix) {
				candidates = append(candidates, n)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		// Prefer the light ("lit") variant: the base look of the unit.
		sort.Slice(candidates, func(i, j int) bool {
			li, lj := strings.Contains(candidates[i][len(token+comp):], "lit"), strings.Contains(candidates[j][len(token+comp):], "lit")
			if li != lj {
				return li
			}
			return candidates[i] < candidates[j]
		})
		db, err := ioutil.ReadFile(filepath.Join(dir, candidates[0]))
		if err != nil {
			return err
		}
		s, err := d2r.ParseDCC(db)
		if err != nil {
			return fmt.Errorf("%s: %v", candidates[0], err)
		}
		if len(s.Directions) != cof.Directions {
			continue
		}
		layers[l.Component] = s
		styles[comp] = candidates[0][len(token+comp) : len(token+comp)+3]
	}
	if len(layers) == 0 {
		return fmt.Errorf("no layer found")
	}
	s, err := cof.Composite(layers)
	if err != nil {
		return err
	}
	base := filepath.Join(c.out, "sprites", cat, token, mode+"_"+wclass)
	if err := writePNG(base+".png", s.Sheet(c.palette(rel))); err != nil {
		return err
	}
	bounds := s.Bounds()
	meta := map[string]interface{}{
		"category":      cat,
		"token":         token,
		"mode":          mode,
		"weaponClass":   wclass,
		"directions":    cof.Directions,
		"framesPerDir":  cof.FramesPerDir,
		"hitFrame":      cof.HitFrame(),
		"events":        cof.Events,
		"animSpeed":     cof.AnimationSpeed,
		"cellWidth":     bounds.Dx(),
		"cellHeight":    bounds.Dy(),
		"anchorX":       -bounds.Min.X,
		"anchorY":       -bounds.Min.Y,
		"layers":        len(layers),
		"layerStyles":   styles,
		"opaqueHeights": opaqueHeights(s),
	}
	return writeJSON(base+".json", meta)
}

// opaqueHeights returns, per direction, the height in pixels of the visible
// figure above the anchor in the first frame (used to compare unit sizes).
func opaqueHeights(s *d2r.Sprite) []int {
	var hs []int
	for _, dir := range s.Directions {
		if len(dir) == 0 {
			hs = append(hs, 0)
			continue
		}
		f := dir[0]
		top := -1
		for y := 0; y < f.Height && top < 0; y++ {
			for x := 0; x < f.Width; x++ {
				if f.Pix[y*f.Width+x] != 0 {
					top = y
					break
				}
			}
		}
		if top < 0 {
			hs = append(hs, 0)
			continue
		}
		hs = append(hs, -(f.OffsetY + top))
	}
	return hs
}

func (c *converter) dc6(rel string) error {
	b, err := ioutil.ReadFile(c.global(rel))
	if err != nil {
		return err
	}
	s, err := d2r.ParseDC6(b)
	if err != nil {
		return err
	}
	return writePNG(filepath.Join(c.out, "dc6", trimExt(rel)+".png"), s.Sheet(c.palette(rel)))
}

func (c *converter) tiles(rel string) error {
	b, err := ioutil.ReadFile(c.global(rel))
	if err != nil {
		return err
	}
	tiles, err := d2r.ParseDT1(b)
	if err != nil {
		return err
	}
	if len(tiles) == 0 {
		return nil
	}
	base := filepath.Join(c.out, trimExt(rel))
	if err := writePNG(base+".png", d2r.TileSheet(tiles, c.palette(rel), 10)); err != nil {
		return err
	}
	type info struct {
		Orientation, Main, Sub, Rarity, Width, Height int
	}
	var infos []info
	for _, t := range tiles {
		infos = append(infos, info{t.Orientation, t.Main, t.Sub, t.Rarity, t.Width, t.Height})
	}
	return writeJSON(base+".json", infos)
}

// orm splits an occlusion/roughness/metalness texture into 3 grayscale images.
func (c *converter) orm(rel string) error {
	b, err := ioutil.ReadFile(c.hd(rel))
	if err != nil {
		return err
	}
	tex, err := d2r.ParseTexture(b)
	if err != nil {
		return err
	}
	img, err := tex.Image(false)
	if err != nil {
		return err
	}
	base := filepath.Join(c.out, "orm", strings.TrimSuffix(trimExt(rel), "_orm"))
	for ch, suffix := range []string{"_ao", "_roughness", "_metal"} {
		g := image.NewGray(img.Rect)
		for i := 0; i < len(g.Pix); i++ {
			g.Pix[i] = img.Pix[i*4+ch]
		}
		if err := writePNG(base+suffix+".png", g); err != nil {
			return err
		}
	}
	return nil
}

// model converts a Granny .model to a binary glTF with its skeleton and textures.
func (c *converter) model(rel string) error {
	b, err := ioutil.ReadFile(c.hd(rel))
	if err != nil {
		return err
	}
	f, err := granny.Parse(b)
	if err != nil {
		return err
	}
	path := filepath.Join(c.out, "models", trimExt(rel)+".glb")
	opts := granny.ExportOptions{Skin: c.skin}
	switch c.textures {
	case "embed":
		opts.Textures = c.texturePNG
	case "link":
		opts.Textures = func(name string) ([]byte, string, error) { return c.textureLink(name, filepath.Dir(path)) }
	}
	glb, _, err := granny.ExportGLB(f, opts)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(path, glb, 0644)
}

func (c *converter) texturePath(name string) string {
	return filepath.Join(c.in, "data", filepath.FromSlash(strings.ToLower(name)))
}

// texturePNG loads "data/hd/.../x_alb.texture" from the extraction and encodes it as PNG.
func (c *converter) texturePNG(name string) ([]byte, string, error) {
	b, err := ioutil.ReadFile(c.texturePath(name))
	if os.IsNotExist(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	tex, err := d2r.ParseTexture(b)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	if err := tex.WritePNG(&buf, d2r.IsNormalMapName(name)); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "", nil
}

// textureLink returns the PNG written by casc -convert png, relative to the .glb folder.
func (c *converter) textureLink(name, glbDir string) ([]byte, string, error) {
	png := strings.TrimSuffix(c.texturePath(name), filepath.Ext(name)) + ".png"
	if _, err := os.Stat(png); err != nil {
		return nil, "", nil
	}
	absPNG, err1 := filepath.Abs(png)
	absDir, err2 := filepath.Abs(glbDir)
	if err1 != nil || err2 != nil {
		return nil, "", nil
	}
	rel, err := filepath.Rel(absDir, absPNG)
	if err != nil {
		return nil, filepath.ToSlash(absPNG), nil
	}
	return nil, filepath.ToSlash(rel), nil
}

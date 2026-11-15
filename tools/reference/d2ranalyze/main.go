/*
d2ranalyze measures the style of an extracted Diablo II Resurrected folder and writes
numbers that can be turned into a neutral style guide.

	d2ranalyze -in extract/d2r -sprites extract/reference/sprites -out extract/reference/analysis

Outputs (under -out):

	palettes.json, palettes/<group>.png   value/saturation percentiles, hue histogram, dominant colours
	materials.json                        occlusion / roughness / metalness distributions
	proportions.csv                       unit heights in pixels and relative to the playable classes
	animation.csv, animation_modes.json   frames, playback rate and hit timing per animation and mode
	report.md                             everything above as tables
*/
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/jybp/casc/d2r"
)

func main() {
	in := flag.String("in", "extract/d2r", "extracted D2R folder (contains data/data/...)")
	sprites := flag.String("sprites", "extract/reference/sprites", "sprites folder written by d2rconvert (optional)")
	out := flag.String("out", "extract/reference/analysis", "output folder")
	mip := flag.Int("mip", 128, "analyse textures at this resolution (smallest mip at least this size)")
	flag.Parse()

	if err := os.MkdirAll(filepath.Join(*out, "palettes"), 0755); err != nil {
		log.Fatal(err)
	}
	hd := filepath.Join(*in, "data", "data", "hd")
	global := filepath.Join(*in, "data", "data", "global")
	r := &report{}

	log.Println("palettes")
	pal, err := analysePalettes(hd, *mip, filepath.Join(*out, "palettes"))
	if err != nil {
		log.Fatal(err)
	}
	r.palettes = pal
	writeJSON(filepath.Join(*out, "palettes.json"), pal)

	log.Println("materials")
	mat, err := analyseMaterials(hd, *mip)
	if err != nil {
		log.Fatal(err)
	}
	r.materials = mat
	writeJSON(filepath.Join(*out, "materials.json"), mat)

	names := monsterNames(filepath.Join(global, "excel", "monstats.txt"))
	charTokens := dirNames(filepath.Join(global, "chars"))
	monTokens := dirNames(filepath.Join(global, "monsters"))

	log.Println("proportions")
	r.proportions = analyseProportions(*sprites, names)
	writeProportions(filepath.Join(*out, "proportions.csv"), r.proportions)

	log.Println("animation")
	anims, modes, err := analyseAnimation(filepath.Join(global, "animdata.d2"), charTokens, monTokens)
	if err != nil {
		log.Fatal(err)
	}
	r.modes = modes
	writeAnimations(filepath.Join(*out, "animation.csv"), anims, names)
	writeJSON(filepath.Join(*out, "animation_modes.json"), modes)

	if err := ioutil.WriteFile(filepath.Join(*out, "report.md"), []byte(r.markdown()), 0644); err != nil {
		log.Fatal(err)
	}
	log.Printf("written to %s\n", *out)
}

//
// Palettes
//

type hsv struct{ h, s, v float64 } // h in degrees, s and v in percent

func toHSV(r, g, b uint8) hsv {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	d := max - min
	var h float64
	switch {
	case d == 0:
		h = 0
	case max == rf:
		h = 60 * math.Mod((gf-bf)/d, 6)
	case max == gf:
		h = 60 * ((bf-rf)/d + 2)
	default:
		h = 60 * ((rf-gf)/d + 4)
	}
	if h < 0 {
		h += 360
	}
	s := 0.0
	if max > 0 {
		s = d / max
	}
	return hsv{h, s * 100, max * 100}
}

type swatch struct {
	Hex   string  `json:"hex"`
	Share float64 `json:"share"`
	H     float64 `json:"h"`
	S     float64 `json:"s"`
	V     float64 `json:"v"`
}

type paletteStats struct {
	Group      string             `json:"group"`
	Textures   int                `json:"textures"`
	Pixels     int                `json:"pixels"`
	Value      map[string]float64 `json:"valuePercentiles"`
	Saturation map[string]float64 `json:"saturationPercentiles"`
	// HueShare is the share of saturated pixels (s >= 15%) per 30 degree bin, starting at 0 (red).
	HueShare []float64 `json:"hueShare"`
	Dominant []swatch  `json:"dominant"`
}

// groupOf returns the analysis groups of a hd texture path (forward slashes, relative to hd).
func groupsOf(rel string) []string {
	p := strings.Split(rel, "/")
	var g []string
	switch {
	case len(p) > 3 && p[0] == "env" && p[1] == "texture":
		g = append(g, "env/"+p[2])
	case len(p) > 3 && p[0] == "env":
		g = append(g, "env/"+p[1])
	case len(p) > 3 && p[0] == "character":
		g = append(g, "character/"+p[1], "character/"+p[1]+"/"+p[2])
	case len(p) > 2 && (p[0] == "objects" || p[0] == "items" || p[0] == "missiles"):
		g = append(g, p[0])
	}
	return g
}

func walkTextures(hd, suffix string, fn func(rel string, b []byte)) error {
	var paths []string
	err := filepath.Walk(hd, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		n := strings.ToLower(info.Name())
		if strings.HasSuffix(n, ".texture") && strings.Contains(n, suffix) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	ch := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range ch {
				b, err := ioutil.ReadFile(path)
				if err != nil {
					continue
				}
				rel, _ := filepath.Rel(hd, path)
				fn(filepath.ToSlash(rel), b)
			}
		}()
	}
	for _, p := range paths {
		ch <- p
	}
	close(ch)
	wg.Wait()
	return nil
}

func sampleTexture(b []byte, mip int) []color.NRGBA {
	tex, err := d2r.ParseTexture(b)
	if err != nil {
		return nil
	}
	img, err := tex.MipImage(tex.SmallestMipAtLeast(mip), false)
	if err != nil {
		return nil
	}
	var px []color.NRGBA
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i+3] < 128 {
			continue
		}
		px = append(px, color.NRGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255})
	}
	// Balance textures: at most 4096 pixels each.
	if len(px) > 4096 {
		step := float64(len(px)) / 4096
		s := make([]color.NRGBA, 0, 4096)
		for f := 0.0; int(f) < len(px); f += step {
			s = append(s, px[int(f)])
		}
		px = s
	}
	return px
}

func analysePalettes(hd string, mip int, swatchDir string) ([]paletteStats, error) {
	var mu sync.Mutex
	pixels := map[string][]color.NRGBA{}
	counts := map[string]int{}
	err := walkTextures(hd, "_alb", func(rel string, b []byte) {
		groups := groupsOf(rel)
		if len(groups) == 0 {
			return
		}
		px := sampleTexture(b, mip)
		mu.Lock()
		for _, g := range groups {
			pixels[g] = append(pixels[g], px...)
			counts[g]++
		}
		mu.Unlock()
	})
	if err != nil {
		return nil, err
	}
	var groups []string
	for g := range pixels {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	var out []paletteStats
	for _, g := range groups {
		px := pixels[g]
		if len(px) == 0 {
			continue
		}
		st := paletteStats{Group: g, Textures: counts[g], Pixels: len(px), HueShare: make([]float64, 12)}
		vs := make([]float64, len(px))
		ss := make([]float64, len(px))
		saturated := 0
		for i, c := range px {
			h := toHSV(c.R, c.G, c.B)
			vs[i], ss[i] = h.v, h.s
			if h.s >= 15 && h.v >= 5 {
				st.HueShare[int(h.h/30)%12]++
				saturated++
			}
		}
		for i := range st.HueShare {
			if saturated > 0 {
				st.HueShare[i] = round(st.HueShare[i]/float64(saturated), 3)
			}
		}
		st.Value = percentiles(vs)
		st.Saturation = percentiles(ss)
		// Dominant colours only for the main groups (unit groups have 3 levels).
		if strings.Count(g, "/") < 2 {
			st.Dominant = kmeans(px, 8)
			writePNG(filepath.Join(swatchDir, strings.ReplaceAll(g, "/", "_")+".png"), swatchImage(st.Dominant))
		}
		out = append(out, st)
	}
	return out, nil
}

func percentiles(v []float64) map[string]float64 {
	sort.Float64s(v)
	m := map[string]float64{}
	for _, p := range []int{10, 25, 50, 75, 90} {
		m[fmt.Sprintf("p%d", p)] = round(v[int(float64(len(v)-1)*float64(p)/100)], 1)
	}
	return m
}

func round(v float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(v*p) / p
}

// kmeans clusters colours (deterministic initialisation spread along luminance).
func kmeans(px []color.NRGBA, k int) []swatch {
	if len(px) > 60000 {
		step := len(px) / 60000
		s := make([]color.NRGBA, 0, 60000)
		for i := 0; i < len(px); i += step {
			s = append(s, px[i])
		}
		px = s
	}
	type c3 struct{ r, g, b float64 }
	pts := make([]c3, len(px))
	for i, p := range px {
		pts[i] = c3{float64(p.R), float64(p.G), float64(p.B)}
	}
	sorted := make([]c3, len(pts))
	copy(sorted, pts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].r+sorted[i].g+sorted[i].b < sorted[j].r+sorted[j].g+sorted[j].b
	})
	centers := make([]c3, k)
	for i := range centers {
		centers[i] = sorted[(2*i+1)*len(sorted)/(2*k)]
	}
	assign := make([]int, len(pts))
	for iter := 0; iter < 15; iter++ {
		var sums = make([]c3, k)
		var n = make([]int, k)
		for i, p := range pts {
			best, bd := 0, math.MaxFloat64
			for j, c := range centers {
				d := (p.r-c.r)*(p.r-c.r) + (p.g-c.g)*(p.g-c.g) + (p.b-c.b)*(p.b-c.b)
				if d < bd {
					best, bd = j, d
				}
			}
			assign[i] = best
			sums[best].r += p.r
			sums[best].g += p.g
			sums[best].b += p.b
			n[best]++
		}
		for j := range centers {
			if n[j] > 0 {
				centers[j] = c3{sums[j].r / float64(n[j]), sums[j].g / float64(n[j]), sums[j].b / float64(n[j])}
			}
		}
	}
	n := make([]int, k)
	for _, a := range assign {
		n[a]++
	}
	var out []swatch
	for j, c := range centers {
		if n[j] == 0 {
			continue
		}
		r, g, b := uint8(c.r+0.5), uint8(c.g+0.5), uint8(c.b+0.5)
		h := toHSV(r, g, b)
		out = append(out, swatch{
			Hex:   fmt.Sprintf("#%02x%02x%02x", r, g, b),
			Share: round(float64(n[j])/float64(len(pts)), 3),
			H:     round(h.h, 0), S: round(h.s, 0), V: round(h.v, 0),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Share > out[j].Share })
	return out
}

func swatchImage(sw []swatch) image.Image {
	const w, h = 800, 100
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	x := 0
	for i, s := range sw {
		var r, g, b uint8
		fmt.Sscanf(s.Hex, "#%02x%02x%02x", &r, &g, &b)
		width := int(math.Round(s.Share * w))
		if i == len(sw)-1 {
			width = w - x
		}
		for xx := x; xx < x+width && xx < w; xx++ {
			for y := 0; y < h; y++ {
				img.SetNRGBA(xx, y, color.NRGBA{r, g, b, 255})
			}
		}
		x += width
	}
	return img
}

//
// Materials
//

type materialStats struct {
	Group     string             `json:"group"`
	Textures  int                `json:"textures"`
	Occlusion map[string]float64 `json:"occlusionPercentiles"` // 0..255
	Roughness map[string]float64 `json:"roughnessPercentiles"` // 0..255
	Metalness map[string]float64 `json:"metalnessPercentiles"` // 0..255
	// MetalShare is the share of pixels with metalness >= 128.
	MetalShare float64 `json:"metalShare"`
}

func analyseMaterials(hd string, mip int) ([]materialStats, error) {
	var mu sync.Mutex
	ch := map[string][3][]float64{}
	counts := map[string]int{}
	err := walkTextures(hd, "_orm", func(rel string, b []byte) {
		groups := groupsOf(rel)
		if len(groups) == 0 {
			return
		}
		px := sampleTexture(b, mip)
		mu.Lock()
		for _, g := range groups {
			if strings.Count(g, "/") >= 2 {
				continue
			}
			v := ch[g]
			for _, p := range px {
				v[0] = append(v[0], float64(p.R))
				v[1] = append(v[1], float64(p.G))
				v[2] = append(v[2], float64(p.B))
			}
			ch[g] = v
			counts[g]++
		}
		mu.Unlock()
	})
	if err != nil {
		return nil, err
	}
	var out []materialStats
	for g, v := range ch {
		if len(v[0]) == 0 {
			continue
		}
		metal := 0
		for _, m := range v[2] {
			if m >= 128 {
				metal++
			}
		}
		out = append(out, materialStats{
			Group: g, Textures: counts[g],
			Occlusion: percentiles(v[0]), Roughness: percentiles(v[1]), Metalness: percentiles(v[2]),
			MetalShare: round(float64(metal)/float64(len(v[2])), 3),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Group < out[j].Group })
	return out, nil
}

//
// Proportions
//

type proportion struct {
	Category, Token, Name string
	Height                int
	Ratio                 float64
}

func analyseProportions(spritesDir string, names map[string]string) []proportion {
	var out []proportion
	var heroes []int
	for _, cat := range []string{"chars", "monsters"} {
		tokens, _ := ioutil.ReadDir(filepath.Join(spritesDir, cat))
		for _, t := range tokens {
			// Neutral pose; characters use the unarmed one.
			var meta struct{ OpaqueHeights []int }
			found := false
			for _, n := range []string{"nu_hth.json", "tn_hth.json"} {
				b, err := ioutil.ReadFile(filepath.Join(spritesDir, cat, t.Name(), n))
				if err == nil && json.Unmarshal(b, &meta) == nil {
					found = true
					break
				}
			}
			if !found {
				matches, _ := filepath.Glob(filepath.Join(spritesDir, cat, t.Name(), "nu_*.json"))
				if len(matches) == 0 {
					continue
				}
				b, _ := ioutil.ReadFile(matches[0])
				if json.Unmarshal(b, &meta) != nil {
					continue
				}
			}
			h := 0
			for _, v := range meta.OpaqueHeights {
				h = maxInt(h, v)
			}
			if h <= 0 {
				continue
			}
			name := names[strings.ToLower(t.Name())]
			out = append(out, proportion{Category: cat, Token: t.Name(), Name: name, Height: h})
			if cat == "chars" {
				heroes = append(heroes, h)
			}
		}
	}
	if len(heroes) > 0 {
		sort.Ints(heroes)
		ref := float64(heroes[len(heroes)/2])
		for i := range out {
			out[i].Ratio = round(float64(out[i].Height)/ref, 2)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Height < out[j].Height })
	return out
}

func writeProportions(path string, ps []proportion) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"category", "token", "name", "heightPx", "ratioToHero"})
	for _, p := range ps {
		w.Write([]string{p.Category, p.Token, p.Name, fmt.Sprint(p.Height), fmt.Sprint(p.Ratio)})
	}
	w.Flush()
}

//
// Animation
//

var modeNames = map[string]string{
	"NU": "neutral", "WL": "walk", "RN": "run", "GH": "get hit", "A1": "attack 1", "A2": "attack 2",
	"BL": "block", "SC": "cast", "TH": "throw", "KK": "kick", "S1": "skill 1", "S2": "skill 2",
	"S3": "skill 3", "S4": "skill 4", "DT": "death", "DD": "dead", "TN": "town neutral",
	"TW": "town walk", "SQ": "sequence", "KB": "knockback", "OP": "opening", "ON": "on", "OF": "off",
}

type modeStats struct {
	Category     string  `json:"category"`
	Mode         string  `json:"mode"`
	Name         string  `json:"name"`
	Count        int     `json:"count"`
	FramesMin    int     `json:"framesMin"`
	FramesMedian int     `json:"framesMedian"`
	FramesMax    int     `json:"framesMax"`
	FPSMedian    float64 `json:"fpsMedian"`
	// Durations in seconds at the native rate.
	DurationMedian float64 `json:"durationMedian"`
	HitFrameMedian float64 `json:"hitFrameMedian"`
	HitRatio       float64 `json:"hitRatioMedian"` // hit frame / frame count
	HitTimeMedian  float64 `json:"hitTimeMedian"`
}

func analyseAnimation(path string, chars, mons map[string]bool) ([]d2r.AnimData, []modeStats, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	anims, err := d2r.ParseAnimData(b)
	if err != nil {
		return nil, nil, err
	}
	type key struct{ cat, mode string }
	groups := map[key][]d2r.AnimData{}
	for _, a := range anims {
		if len(a.Name) < 4 || a.Speed <= 0 || a.FramesPerDir <= 0 {
			continue
		}
		tok := strings.ToLower(a.Name[:2])
		cat := "objects"
		if chars[tok] {
			cat = "chars"
		} else if mons[tok] {
			cat = "monsters"
		}
		k := key{cat, a.Name[2:4]}
		groups[k] = append(groups[k], a)
	}
	var out []modeStats
	for k, list := range groups {
		var frames []int
		var fps, dur, hit, ratio, hitT []float64
		for _, a := range list {
			frames = append(frames, a.FramesPerDir)
			fps = append(fps, a.FPS())
			dur = append(dur, float64(a.FramesPerDir)/a.FPS())
			if h := a.HitFrame(); h >= 0 {
				hit = append(hit, float64(h))
				ratio = append(ratio, float64(h)/float64(a.FramesPerDir))
				hitT = append(hitT, float64(h)/a.FPS())
			}
		}
		sort.Ints(frames)
		out = append(out, modeStats{
			Category: k.cat, Mode: k.mode, Name: modeNames[k.mode], Count: len(list),
			FramesMin: frames[0], FramesMedian: frames[len(frames)/2], FramesMax: frames[len(frames)-1],
			FPSMedian: round(median(fps), 1), DurationMedian: round(median(dur), 2),
			HitFrameMedian: round(median(hit), 1), HitRatio: round(median(ratio), 2), HitTimeMedian: round(median(hitT), 2),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Count > out[j].Count
	})
	return anims, out, nil
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return -1
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

func writeAnimations(path string, anims []d2r.AnimData, names map[string]string) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"name", "unit", "mode", "weaponClass", "frames", "fps", "duration", "hitFrame"})
	sort.Slice(anims, func(i, j int) bool { return anims[i].Name < anims[j].Name })
	for _, a := range anims {
		if len(a.Name) < 4 || a.Speed <= 0 {
			continue
		}
		unit := names[strings.ToLower(a.Name[:2])]
		w.Write([]string{a.Name, unit, a.Name[2:4], a.Name[4:], fmt.Sprint(a.FramesPerDir),
			fmt.Sprint(round(a.FPS(), 2)), fmt.Sprint(round(float64(a.FramesPerDir)/a.FPS(), 2)), fmt.Sprint(a.HitFrame())})
	}
	w.Flush()
}

//
// Helpers
//

func monsterNames(path string) map[string]string {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n")
	names := map[string]string{}
	if len(lines) == 0 {
		return names
	}
	cols := strings.Split(lines[0], "\t")
	idCol, codeCol := -1, -1
	for i, c := range cols {
		switch c {
		case "Id":
			idCol = i
		case "Code":
			codeCol = i
		}
	}
	if idCol < 0 || codeCol < 0 {
		return names
	}
	for _, l := range lines[1:] {
		f := strings.Split(l, "\t")
		if len(f) <= codeCol || len(f) <= idCol {
			continue
		}
		code := strings.ToLower(f[codeCol])
		if code != "" && names[code] == "" {
			names[code] = f[idCol]
		}
	}
	return names
}

func dirNames(path string) map[string]bool {
	m := map[string]bool{}
	entries, _ := ioutil.ReadDir(path)
	for _, e := range entries {
		if e.IsDir() {
			m[strings.ToLower(e.Name())] = true
		}
	}
	return m
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func writeJSON(path string, v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		log.Fatal(err)
	}
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

//
// Report
//

type report struct {
	palettes    []paletteStats
	materials   []materialStats
	proportions []proportion
	modes       []modeStats
}

func (r *report) markdown() string {
	var b strings.Builder
	b.WriteString("# Reference measurements\n\n")
	b.WriteString("Generated by d2ranalyze. Private reference: numbers only, describe them in neutral terms before using them elsewhere.\n\n")

	b.WriteString("## Albedo palettes\n\nValue and saturation in percent (HSV, sRGB albedo, before lighting).\n\n")
	b.WriteString("| Group | Textures | Value p10 / p50 / p90 | Saturation p10 / p50 / p90 | Main hues | Dominant colours |\n|---|---|---|---|---|---|\n")
	for _, p := range r.palettes {
		if strings.Count(p.Group, "/") >= 2 {
			continue
		}
		var dom []string
		for _, s := range p.Dominant {
			dom = append(dom, fmt.Sprintf("`%s` %.0f%%", s.Hex, s.Share*100))
		}
		fmt.Fprintf(&b, "| %s | %d | %.0f / %.0f / %.0f | %.0f / %.0f / %.0f | %s | %s |\n", p.Group, p.Textures,
			p.Value["p10"], p.Value["p50"], p.Value["p90"], p.Saturation["p10"], p.Saturation["p50"], p.Saturation["p90"],
			mainHues(p.HueShare), strings.Join(dom, ", "))
	}

	b.WriteString("\n## Materials\n\nChannels of the occlusion/roughness/metalness textures, 0-255.\n\n")
	b.WriteString("| Group | Textures | Occlusion p50 | Roughness p10 / p50 / p90 | Metal share |\n|---|---|---|---|---|\n")
	for _, m := range r.materials {
		fmt.Fprintf(&b, "| %s | %d | %.0f | %.0f / %.0f / %.0f | %.0f%% |\n", m.Group, m.Textures, m.Occlusion["p50"],
			m.Roughness["p10"], m.Roughness["p50"], m.Roughness["p90"], m.MetalShare*100)
	}

	if len(r.proportions) > 0 {
		b.WriteString("\n## Unit heights\n\nNeutral pose, pixels above the feet in the original 2D sprites, relative to the median playable class.\n\n")
		buckets := []struct {
			name     string
			min, max float64
		}{{"tiny (< 0.5)", 0, 0.5}, {"small (0.5 - 0.8)", 0.5, 0.8}, {"human (0.8 - 1.2)", 0.8, 1.2},
			{"large (1.2 - 1.8)", 1.2, 1.8}, {"huge (1.8 - 2.5)", 1.8, 2.5}, {"giant (>= 2.5)", 2.5, 1e9}}
		b.WriteString("| Size class | Units | Examples |\n|---|---|---|\n")
		for _, bk := range buckets {
			var ex []string
			n := 0
			for _, p := range r.proportions {
				if p.Category != "monsters" || p.Ratio < bk.min || p.Ratio >= bk.max {
					continue
				}
				n++
				if len(ex) < 6 && p.Name != "" {
					ex = append(ex, fmt.Sprintf("%s %.2f", p.Name, p.Ratio))
				}
			}
			fmt.Fprintf(&b, "| %s | %d | %s |\n", bk.name, n, strings.Join(ex, ", "))
		}
	}

	b.WriteString("\n## Animation timing\n\nNative playback; durations and hit times in seconds.\n\n")
	b.WriteString("| Units | Mode | Animations | Frames min / median / max | FPS | Duration | Hit frame | Hit at % | Hit time |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, m := range r.modes {
		if m.Count < 5 || m.Category == "objects" {
			continue
		}
		hit, ratio, ht := "-", "-", "-"
		if m.HitFrameMedian >= 0 {
			hit = fmt.Sprint(m.HitFrameMedian)
			ratio = fmt.Sprintf("%.0f%%", m.HitRatio*100)
			ht = fmt.Sprint(m.HitTimeMedian)
		}
		fmt.Fprintf(&b, "| %s | %s %s | %d | %d / %d / %d | %.1f | %.2f | %s | %s | %s |\n", m.Category, m.Mode, m.Name, m.Count,
			m.FramesMin, m.FramesMedian, m.FramesMax, m.FPSMedian, m.DurationMedian, hit, ratio, ht)
	}
	return b.String()
}

func mainHues(share []float64) string {
	names := []string{"red", "orange", "yellow", "yellow-green", "green", "green-cyan", "cyan", "azure", "blue", "violet", "magenta", "rose"}
	type hs struct {
		name  string
		share float64
	}
	var l []hs
	for i, s := range share {
		l = append(l, hs{names[i], s})
	}
	sort.Slice(l, func(i, j int) bool { return l[i].share > l[j].share })
	var parts []string
	for _, h := range l[:3] {
		if h.share >= 0.05 {
			parts = append(parts, fmt.Sprintf("%s %.0f%%", h.name, h.share*100))
		}
	}
	return strings.Join(parts, ", ")
}

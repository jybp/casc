package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

type preset struct {
	description string
	patterns    []string
}

// presets are groups of files, mostly for Diablo II Resurrected.
// "hd" folders contain the remastered 3D assets (.model, .texture, .json, ...),
// "global" folders contain the original 2D assets (.dcc, .dc6, .dt1, .ds1, .cof, ...).
var presets = map[string]preset{
	"characters": {"D2R playable classes: HD models/textures/animations and legacy sprites", []string{
		`^data:data/hd/character/(player|common)/`,
		`^data:data/global/chars/`,
	}},
	"enemies": {"D2R monsters and bosses: HD models/textures/animations and legacy sprites", []string{
		`^data:data/hd/character/enemy/`,
		`^data:data/global/monsters/`,
	}},
	"npcs": {"D2R town NPCs and hirelings", []string{
		`^data:data/hd/character/npc/`,
	}},
	"environments": {"D2R levels/dungeons: HD env models, textures, presets, foliage, room tiles and legacy tiles (.dt1/.ds1)", []string{
		`^data:data/hd/env/`,
		`^data:data/hd/roomtiles/`,
		`^data:data/hd/bvt/`,
		`^data:data/global/tiles/`,
	}},
	"objects": {"D2R world objects (chests, shrines, doors, ...)", []string{
		`^data:data/hd/objects/`,
		`^data:data/global/objects/`,
	}},
	"items": {"D2R items", []string{
		`^data:data/hd/items/`,
		`^data:data/global/items/`,
	}},
	"vfx": {"D2R visual effects, missiles and overlays", []string{
		`^data:data/hd/(vfx|missiles|overlays)/`,
		`^data:data/global/(missiles|overlays)/`,
	}},
	"ui": {"D2R user interface, fonts and sprites", []string{
		`^data:data/hd/global/ui/`,
		`^data:data/global/ui/`,
		`^data:data/local/(ui|font)/`,
		`\.sprite$`,
	}},
	"audio": {"D2R music, sounds and voices (all locales)", []string{
		`^data:data/hd/global/(music|sfx)/`,
		`^data:data/global/(sfx|music)/`,
		`^data:data/local/sfx/`,
		`^data:locales/audio/`,
		`\.flac$`,
	}},
	"data": {"D2R game data tables and strings (.txt, .json, .tbl, .bin)", []string{
		`^data:data/global/excel/`,
		`^data:data/global/dataguide/`,
		`^data:data/local/lng/`,
		`^data:locales/data/`,
	}},
	"video": {"Cinematics", []string{
		`\.webm$`,
	}},
}

func printPresets() {
	var names []string
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("%-13s %s\n", name, presets[name].description)
		for _, p := range presets[name].patterns {
			fmt.Printf("%-13s   %s\n", "", p)
		}
	}
}

// newFilter returns a func reporting if a file must be kept.
// A file is kept if it matches pattern (when set) and one of the presets (when set).
func newFilter(pattern, presetNames string) (func(string) bool, error) {
	var patternRe *regexp.Regexp
	if pattern != "" {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		patternRe = re
	}
	var presetRes []*regexp.Regexp
	for _, name := range strings.Split(presetNames, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		p, ok := presets[name]
		if !ok {
			return nil, errors.Errorf("unknown preset %q, see -presets", name)
		}
		for _, s := range p.patterns {
			presetRes = append(presetRes, regexp.MustCompile("(?i)"+s))
		}
	}
	return func(filename string) bool {
		if patternRe != nil && !patternRe.MatchString(filename) {
			return false
		}
		if len(presetRes) == 0 {
			return true
		}
		for _, re := range presetRes {
			if re.MatchString(filename) {
				return true
			}
		}
		return false
	}, nil
}

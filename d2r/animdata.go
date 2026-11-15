package d2r

import (
	"encoding/binary"
	"strings"

	"github.com/pkg/errors"
)

// AnimData is an entry of animdata.d2: the timing of one animation (unit, mode, weapon class).
type AnimData struct {
	Name         string // i.e. "DUA1HTH": token, mode, weapon class
	FramesPerDir int
	Speed        int    // 256 = one frame per game tick (25 per second)
	Events       []byte // per frame: 0 none, 1 attack, 2 missile, 3 sound, 4 skill
}

// FPS returns the playback rate in frames per second.
func (a AnimData) FPS() float64 {
	return 25 * float64(a.Speed) / 256
}

// HitFrame returns the first attack/missile/skill frame, or -1.
func (a AnimData) HitFrame() int {
	for i, e := range a.Events {
		if e == 1 || e == 2 || e == 4 {
			return i
		}
	}
	return -1
}

// ParseAnimData parses animdata.d2 (256 hash buckets of 160 bytes records).
func ParseAnimData(b []byte) ([]AnimData, error) {
	le := binary.LittleEndian
	var out []AnimData
	o := 0
	for bucket := 0; bucket < 256; bucket++ {
		if o+4 > len(b) {
			return nil, errors.New("truncated animdata")
		}
		n := int(le.Uint32(b[o:]))
		o += 4
		if n < 0 || o+n*160 > len(b) {
			return nil, errors.New("invalid animdata bucket")
		}
		for i := 0; i < n; i++ {
			r := b[o : o+160]
			name := string(r[:8])
			if k := strings.IndexByte(name, 0); k >= 0 {
				name = name[:k]
			}
			frames := int(le.Uint32(r[8:]))
			if frames > 144 {
				frames = 144
			}
			ev := make([]byte, frames)
			copy(ev, r[16:16+frames])
			out = append(out, AnimData{
				Name:         name,
				FramesPerDir: int(le.Uint32(r[8:])),
				Speed:        int(le.Uint32(r[12:])),
				Events:       ev,
			})
			o += 160
		}
	}
	return out, nil
}

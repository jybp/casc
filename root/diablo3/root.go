package diablo3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"

	"github.com/pkg/errors"
)

type NamedEntry struct {
	ContentHash [0x10]uint8
	Filename    string
}

type AssetEntry struct {
	ContentHash [0x10]uint8
	SNOID       uint32
}

type AssetIdxEntry struct {
	ContentHash [0x10]uint8
	SNOID       uint32
	FileIndex   uint32
}

type SnoExtension struct {
	Name      string
	Extension string
}

type SnoInfo struct {
	SnoExtension
	GroupID uint32
}

var SnoExtensions = []SnoExtension{
	SnoExtension{"", ""},             //0
	SnoExtension{"Actor", "acr"},     //1
	SnoExtension{"Adventure", "adv"}, //2 ...
	SnoExtension{"AiBehavior", ""},
	SnoExtension{"AiState", ""},
	SnoExtension{"AmbientSound", "ams"},
	SnoExtension{"Anim", "ani"},
	SnoExtension{"Animation2D", "an2"},
	SnoExtension{"AnimSet", "ans"},
	SnoExtension{"Appearance", "app"},
	SnoExtension{"Hero", ""},
	SnoExtension{"Cloth", "clt"},
	SnoExtension{"Conversation", "cnv"},
	SnoExtension{"ConversationList", ""},
	SnoExtension{"EffectGroup", "efg"},
	SnoExtension{"Encounter", "enc"},
	SnoExtension{"Explosion", "xpl"},
	SnoExtension{"FlagSet", ""},
	SnoExtension{"Font", "fnt"},
	SnoExtension{"GameBalance", "gam"},
	SnoExtension{"Globals", "glo"},
	SnoExtension{"LevelArea", "lvl"},
	SnoExtension{"Light", "lit"},
	SnoExtension{"MarkerSet", "mrk"},
	SnoExtension{"Monster", "mon"},
	SnoExtension{"Observer", "obs"},
	SnoExtension{"Particle", "prt"},
	SnoExtension{"Physics", "phy"},
	SnoExtension{"Power", "pow"},
	SnoExtension{"Quest", "qst"},
	SnoExtension{"Rope", "rop"},
	SnoExtension{"Scene", "scn"},
	SnoExtension{"SceneGroup", "scg"},
	SnoExtension{"Script", ""},
	SnoExtension{"ShaderMap", "shm"},
	SnoExtension{"Shaders", "shd"},
	SnoExtension{"Shakes", "shk"},
	SnoExtension{"SkillKit", "skl"},
	SnoExtension{"Sound", "snd"},
	SnoExtension{"SoundBank", "sbk"},
	SnoExtension{"StringList", "stl"},
	SnoExtension{"Surface", "srf"},
	SnoExtension{"Textures", "tex"},
	SnoExtension{"Trail", "trl"},
	SnoExtension{"UI", "ui"},
	SnoExtension{"Weather", "wth"},
	SnoExtension{"Worlds", "wrl"},
	SnoExtension{"Recipe", "rcp"},
	SnoExtension{"Condition", "cnd"},
	SnoExtension{"TreasureClass", ""},
	SnoExtension{"Account", ""},
	SnoExtension{"Conductor", ""},
	SnoExtension{"TimedEvent", ""},
	SnoExtension{"Act", "act"},
	SnoExtension{"Material", "mat"},
	SnoExtension{"QuestRange", "qsr"},
	SnoExtension{"Lore", "lor"},
	SnoExtension{"Reverb", "rev"},
	SnoExtension{"PhysMesh", "phm"},
	SnoExtension{"Music", "mus"},
	SnoExtension{"Tutorial", "tut"},
	SnoExtension{"BossEncounter", "bos"},
	SnoExtension{"ControlScheme", ""},
	SnoExtension{"Accolade", "aco"},
	SnoExtension{"AnimTree", "ant"},
	SnoExtension{"Vibration", ""},
	SnoExtension{"DungeonFinder", ""},
}

const snoGroupSize = 70

type CoreTocHeader struct {
	EntryCounts  [snoGroupSize]uint32
	EntryOffsets [snoGroupSize]uint32
	Unks         [snoGroupSize]uint32
	Unk          uint32
}

type Root struct {
	assetsEntries   map[string][]AssetEntry
	assetIdxEntries map[string][]AssetIdxEntry
	namedEntries    map[string][]NamedEntry
}

func (r *Root) Files() ([]string, error) {
	var names []string
	for subdir, namedEntries := range r.namedEntries {
		for _, namedEntry := range namedEntries {
			names = append(names, subdir+"\\"+namedEntry.Filename)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (r *Root) ContentHash(filename string) ([]byte, error) {
	return nil, errors.New("not implemented")
	// contentHash, ok := r.lookup[filename]
	// if !ok {
	// 	return nil, errors.WithStack(fmt.Errorf("%s file name not found", filename))
	// }
	// return contentHash, nil
}

func NewRoot(rootHash []byte, dataFromContentHashFn func(contentHash []byte) ([]byte, error)) (*Root, error) {
	rootB, err := dataFromContentHashFn(rootHash)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(rootB)
	var sig uint32
	if err := binary.Read(r, binary.LittleEndian, &sig); err != nil {
		return nil, errors.WithStack(err)
	}
	if sig != 0x8007D0C4 /* Diablo III */ {
		return nil, errors.WithStack(fmt.Errorf("invalid Diablo III root signature %x", sig))
	}

	var namedEntriesCount uint32
	if err := binary.Read(r, binary.LittleEndian, &namedEntriesCount); err != nil {
		return nil, errors.WithStack(err)
	}

	readAsciizFn := func(r io.Reader, dest *string) error {
		buf := bytes.NewBufferString("")
		for {
			var c byte
			if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
				return errors.WithStack(err)
			}
			if c == 0 { //ASCIIZ
				break
			}
			buf.WriteByte(c)
		}
		*dest = buf.String()
		return nil
	}

	assetsEntries := map[string][]AssetEntry{}
	assetIdxEntries := map[string][]AssetIdxEntry{}
	namedEntries := map[string][]NamedEntry{}
	for i := uint32(0); i < namedEntriesCount; i++ {
		dirEntry := NamedEntry{}
		if err := binary.Read(r, binary.LittleEndian, &dirEntry.ContentHash); err != nil {
			return nil, errors.WithStack(err)
		}
		if err := readAsciizFn(r, &dirEntry.Filename); err != nil {
			return nil, errors.WithStack(err)
		}

		dirB, err := dataFromContentHashFn(dirEntry.ContentHash[:])
		if err != nil {
			// We fail silently because 'Mac' and 'Windows' dirEntry cannot be fetch.
			// Also each language has a dirEntry that won't be referenced in the .idx if not installed.
			fmt.Printf("failed fetching %s (%x)\n", dirEntry.Filename, dirEntry.ContentHash)
			continue
		}
		dirR := bytes.NewReader(dirB)

		// sig uint32
		// number of AssetEntry uint32
		// []AssetEntry
		// number of AssetIdxEntry uint32
		// []AssetIdxEntry
		// number of NamedEntry uint32
		// []NamedEntry

		var sig uint32
		if err := binary.Read(dirR, binary.LittleEndian, &sig); err != nil {
			return nil, errors.WithStack(err)
		}
		if sig != 0xeaf1fe87 {
			return nil, errors.WithStack(errors.New("unexpected subdir signature"))
		}

		var assetCount uint32
		if err := binary.Read(dirR, binary.LittleEndian, &assetCount); err != nil {
			return nil, errors.WithStack(err)
		}
		for i := uint32(0); i < assetCount; i++ {
			assetEntry := AssetEntry{}
			if err := binary.Read(dirR, binary.LittleEndian, &assetEntry.ContentHash); err != nil {
				return nil, errors.WithStack(err)
			}
			if err := binary.Read(dirR, binary.LittleEndian, &assetEntry.SNOID); err != nil {
				return nil, errors.WithStack(err)
			}
		}

		var assetIdxCount uint32
		if err := binary.Read(dirR, binary.LittleEndian, &assetIdxCount); err != nil {
			return nil, errors.WithStack(err)
		}
		for i := uint32(0); i < assetIdxCount; i++ {
			assetIdxEntry := AssetIdxEntry{}
			if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.ContentHash); err != nil {
				return nil, errors.WithStack(err)
			}
			if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.SNOID); err != nil {
				return nil, errors.WithStack(err)
			}
			if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.FileIndex); err != nil {
				return nil, errors.WithStack(err)
			}
		}

		var namedCount uint32
		if err := binary.Read(dirR, binary.LittleEndian, &namedCount); err != nil {
			return nil, errors.WithStack(err)
		}
		for i := uint32(0); i < namedCount; i++ {
			namedEntry := NamedEntry{}
			if err := binary.Read(dirR, binary.LittleEndian, &namedEntry.ContentHash); err != nil {
				return nil, errors.WithStack(err)
			}
			if err := readAsciizFn(dirR, &namedEntry.Filename); err != nil {
				return nil, errors.WithStack(err)
			}
			namedEntries[dirEntry.Filename] = append(namedEntries[dirEntry.Filename], namedEntry)
		}
	}

	// CoreTOC.dat

	baseNamedEntries, ok := namedEntries["Base"]
	if !ok {
		return nil, errors.WithStack(errors.New("Base not found"))
	}
	coreTocEntry := NamedEntry{}
	for _, namedEntry := range baseNamedEntries {
		if namedEntry.Filename == "CoreTOC.dat" {
			coreTocEntry = namedEntry
		}
	}
	if coreTocEntry == (NamedEntry{}) {
		return nil, errors.WithStack(errors.New("CoreTOC.dat not found"))
	}
	coreTocB, err := dataFromContentHashFn(coreTocEntry.ContentHash[:])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	coreTocR := bytes.NewReader(coreTocB)
	coreTocHeader := CoreTocHeader{}
	if err := binary.Read(coreTocR, binary.LittleEndian, &coreTocHeader); err != nil {
		return nil, errors.WithStack(err)
	}

	fmt.Printf("coreTocHeader: %+v\n", coreTocHeader)

	snoInfos := map[uint32]SnoInfo{}
	for i := uint32(0); i < snoGroupSize; i++ {
		if coreTocHeader.EntryCounts[i] == 0 {
			continue
		}
		coreTocHeaderSize := int64(70*4*3 + 1)
		if _, err := coreTocR.Seek(int64(coreTocHeader.EntryOffsets[i])+coreTocHeaderSize,
			io.SeekStart); err != nil {
			return nil, errors.WithStack(err)
		}

		fmt.Printf("coreTocHeader.EntryCounts %d: %+v\n", i, coreTocHeader.EntryCounts[i])

		for j := uint32(0); j < coreTocHeader.EntryCounts[i]; j++ {
			var snoGroupID uint32 //index of SnoExtensions
			if err := binary.Read(coreTocR, binary.LittleEndian, &snoGroupID); err != nil {
				return nil, errors.WithStack(err)
			}
			var snoID uint32
			if err := binary.Read(coreTocR, binary.LittleEndian, &snoID); err != nil {
				return nil, errors.WithStack(err)
			}
			var namePos uint32
			if err := binary.Read(coreTocR, binary.LittleEndian, &namePos); err != nil {
				return nil, errors.WithStack(err)
			}

			//TODO retrieve name
			name := "TODO"
			// unk := make([]byte, namePos)
			// if err := binary.Read(coreTocR, binary.LittleEndian, &unk); err != nil {
			// 	return nil, errors.WithStack(err)
			// }
			// var name string
			// if err := readAsciizFn(coreTocR, &name); err != nil {
			// 	return nil, err
			// }
			if snoGroupID < 0 || snoGroupID >= uint32(len(SnoExtensions)) {
				fmt.Printf("snoGroupID %d for id %d (%s) outside SnoExtensions(%d)\n", snoGroupID, snoID, name, len(SnoExtensions))
				continue
			}

			snoInfos[snoID] = SnoInfo{
				GroupID:      snoGroupID, //TODO needed?
				SnoExtension: SnoExtensions[snoGroupID],
			}
		}
	}

	fmt.Printf("CoreTOC.dat:\n")
	for id, snoInfo := range snoInfos {
		fmt.Printf("%d %s (%s):\n", id, snoInfo.Name, snoInfo.Extension)
	}

	return &Root{
		assetsEntries,
		assetIdxEntries,
		namedEntries,
	}, nil
}

package diablo3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unsafe"

	"github.com/jybp/casc/common"
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
	SnoGroupID uint32
	Filename   string
}

// SnoExtensions relates arbitrary friendly names => extension
var SnoExtensions = []SnoExtension{
	SnoExtension{"", ""},                 // 0x00 Index matters
	SnoExtension{"Actor", "acr"},         // 0x01
	SnoExtension{"Adventure", "adv"},     // 0x02
	SnoExtension{"", ""},                 // 0x03
	SnoExtension{"", ""},                 // 0x04
	SnoExtension{"AmbientSound", "ams"},  // 0x05
	SnoExtension{"Anim", "ani"},          // 0x06
	SnoExtension{"Anim2D", "an2"},        // 0x07
	SnoExtension{"AnimSet", "ans"},       // 0x08
	SnoExtension{"Appearance", "app"},    // 0x09
	SnoExtension{"", ""},                 // 0x0A
	SnoExtension{"Cloth", "clt"},         // 0x0B
	SnoExtension{"Conversation", "cnv"},  // 0x0C
	SnoExtension{"", ""},                 // 0x0D
	SnoExtension{"EffectGroup", "efg"},   // 0x0E
	SnoExtension{"Encounter", "enc"},     // 0x0F
	SnoExtension{"", ""},                 // 0x10
	SnoExtension{"Explosion", "xpl"},     // 0x11
	SnoExtension{"", ""},                 // 0x12
	SnoExtension{"Font", "fnt"},          // 0x13
	SnoExtension{"GameBalance", "gam"},   // 0x14
	SnoExtension{"Globals", "glo"},       // 0x15
	SnoExtension{"LevelArea", "lvl"},     // 0x16
	SnoExtension{"Light", "lit"},         // 0x17
	SnoExtension{"MarkerSet", "mrk"},     // 0x18
	SnoExtension{"Monster", "mon"},       // 0x19
	SnoExtension{"Observer", "obs"},      // 0x1A
	SnoExtension{"Particle", "prt"},      // 0x1B
	SnoExtension{"Physics", "phy"},       // 0x1C
	SnoExtension{"Power", "pow"},         // 0x1D
	SnoExtension{"", ""},                 // 0x1E
	SnoExtension{"Quest", "qst"},         // 0x1F
	SnoExtension{"Rope", "rop"},          // 0x20
	SnoExtension{"Scene", "scn"},         // 0x21
	SnoExtension{"SceneGroup", "scg"},    // 0x22
	SnoExtension{"", ""},                 // 0x23
	SnoExtension{"ShaderMap", "shm"},     // 0x24
	SnoExtension{"Shaders", "shd"},       // 0x25
	SnoExtension{"Shakes", "shk"},        // 0x26
	SnoExtension{"SkillKit", "skl"},      // 0x27
	SnoExtension{"Sound", "snd"},         // 0x28
	SnoExtension{"SoundBank", "sbk"},     // 0x29
	SnoExtension{"StringList", "stl"},    // 0x2A
	SnoExtension{"Surface", "srf"},       // 0x2B
	SnoExtension{"Textures", "tex"},      // 0x2C
	SnoExtension{"Trail", "trl"},         // 0x2D
	SnoExtension{"UI", "ui"},             // 0x2E
	SnoExtension{"Weather", "wth"},       // 0x2F
	SnoExtension{"Worlds", "wrl"},        // 0x30
	SnoExtension{"Recipe", "rcp"},        // 0x31
	SnoExtension{"", ""},                 // 0x32
	SnoExtension{"Condition", "cnd"},     // 0x33
	SnoExtension{"", ""},                 // 0x34
	SnoExtension{"", ""},                 // 0x35
	SnoExtension{"", ""},                 // 0x36
	SnoExtension{"", ""},                 // 0x37
	SnoExtension{"Act", "act"},           // 0x38
	SnoExtension{"Material", "mat"},      // 0x39
	SnoExtension{"QuestRange", "qsr"},    // 0x3A
	SnoExtension{"Lore", "lor"},          // 0x3B
	SnoExtension{"Reverb", "rev"},        // 0x3C
	SnoExtension{"PhysMesh", "phm"},      // 0x3D
	SnoExtension{"Music", "mus"},         // 0x3E
	SnoExtension{"Tutorial", "tut"},      // 0x3F
	SnoExtension{"BossEncounter", "bos"}, // 0x40
	SnoExtension{"", ""},                 // 0x41
	SnoExtension{"Accolade", "aco"},      // 0x42
}

const snoGroupSize = 70

type CoreTocHeader struct {
	EntryCounts  [snoGroupSize]uint32
	EntryOffsets [snoGroupSize]uint32
	Unks         [snoGroupSize]uint32
	Unk          uint32
}

type Root struct {
	nameToContentHash map[string][0x10]byte
}

func (r *Root) Files() ([]string, error) {
	names := []string{}
	for name := range r.nameToContentHash {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (r *Root) ContentHash(filename string) ([]byte, error) {
	contentHash, ok := r.nameToContentHash[filename]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("%s file name not found", filename))
	}
	return contentHash[:], nil
}

func NewRoot(root []byte, fetchFn func(contentHash []byte) ([]byte, error)) (*Root, error) {
	dirEntries, err := parseRoot(bytes.NewReader(root))
	if err != nil {
		return nil, err
	}
	assetEntriesByDir := map[string][]AssetEntry{}
	assetIdxEntriesByDir := map[string][]AssetIdxEntry{}
	namedEntriesByDir := map[string][]NamedEntry{}
	for _, dirEntry := range dirEntries {
		dirB, err := fetchFn(dirEntry.ContentHash[:])
		if err != nil {
			// 'Mac' and 'Windows' dirEntry cannot be fetch. //TODO can it now with fix?
			// Each language has a dirEntry that won't be referenced in the .idx if not installed.
			continue
		}
		assets, assetIdxs, nameds, err := parseRootDirectory(bytes.NewReader(dirB))
		if err != nil {
			return nil, err
		}
		assetEntriesByDir[dirEntry.Filename] = assets
		assetIdxEntriesByDir[dirEntry.Filename] = assetIdxs
		namedEntriesByDir[dirEntry.Filename] = nameds
	}

	//
	// CoreTOC.dat
	//
	baseNamedEntries, ok := namedEntriesByDir["Base"]
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
	coreTocB, err := fetchFn(coreTocEntry.ContentHash[:])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	snoInfos, err := parseCoreToc(bytes.NewReader(coreTocB))
	if err != nil {
		return nil, err
	}

	//
	// Packages.dat
	//
	packagesEntry := NamedEntry{}
	for _, namedEntry := range baseNamedEntries {
		if namedEntry.Filename == "Data_D3\\PC\\Misc\\Packages.dat" {
			packagesEntry = namedEntry
		}
	}
	if packagesEntry == (NamedEntry{}) {
		return nil, errors.WithStack(errors.New("Data_D3\\PC\\Misc\\Packages.dat not found"))
	}
	packagesB, err := fetchFn(packagesEntry.ContentHash[:])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	nameToExt, err := parsePackages(bytes.NewReader(packagesB))
	if err != nil {
		return nil, err
	}

	//
	// Compure names to hash
	//
	namePartsFn := func(snoID uint32) (filename, extension, extensionName string) {
		snoInfo, ok := snoInfos[snoID]
		if !ok {
			return
		}
		filename = snoInfo.Filename
		if snoInfo.SnoGroupID >= 0 && int(snoInfo.SnoGroupID) < len(SnoExtensions) {
			extension = SnoExtensions[snoInfo.SnoGroupID].Extension
			extensionName = SnoExtensions[snoInfo.SnoGroupID].Name
			//TODO Necromancer skillkit missing
			if strings.Contains(extensionName, "SkillKit") {
				fmt.Println(filename)
			}
		} else {
			//extension not found in snoInfo, generate a random one
			ext := []rune{
				rune('0') + rune(snoInfo.SnoGroupID/10),
				rune('0') + rune(snoInfo.SnoGroupID%10),
			}
			extension = fmt.Sprintf("a%s", string(ext))
			extensionName = fmt.Sprintf("Asset%s", string(ext))
		}
		return
	}
	nameToContentHash := map[string][0x10]byte{}
	for subdir, assetEntries := range assetEntriesByDir {
		for _, assetEntry := range assetEntries {
			filename, extension, extensionName := namePartsFn(assetEntry.SNOID)
			if filename == "" {
				continue
			}
			name := fmt.Sprintf("%s\\%s\\%s.%s", subdir, extensionName, filename, extension)
			nameToContentHash[common.CleanPath(name)] = assetEntry.ContentHash
		}
	}
	for subdir, assetIdxEntries := range assetIdxEntriesByDir {
		for _, assetIdxEntry := range assetIdxEntries {
			filename, extension, extensionName := namePartsFn(assetIdxEntry.SNOID)
			if filename == "" {
				continue
			}
			//Packages.dat might contain the real extension of assetIdxEntry.
			namewithoutDirAndExt := fmt.Sprintf("%s\\%s\\%04d", extensionName, filename, assetIdxEntry.FileIndex)
			realExtension, ok := nameToExt[namewithoutDirAndExt]
			if ok {
				extension = strings.TrimLeft(realExtension, ".")
			}
			name := fmt.Sprintf("%s\\%s.%s", subdir, namewithoutDirAndExt, extension)
			nameToContentHash[common.CleanPath(name)] = assetIdxEntry.ContentHash
		}
	}
	for subdir, namedEntries := range namedEntriesByDir {
		for _, namedEntry := range namedEntries {
			name := fmt.Sprintf("%s\\%s", subdir, namedEntry.Filename)
			nameToContentHash[common.CleanPath(name)] = namedEntry.ContentHash
		}
	}
	return &Root{nameToContentHash}, nil
}

func readAsciiz(r io.Reader, dest *string) error {
	buf := bytes.NewBufferString("")
	for {
		var c byte
		if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
			return errors.WithStack(err)
		}
		if c == 0 {
			break
		}
		buf.WriteByte(c)
	}
	*dest = buf.String()
	return nil
}

func parseRoot(r io.Reader) ([]NamedEntry, error) {
	var rootSig uint32
	if err := binary.Read(r, binary.LittleEndian, &rootSig); err != nil {
		return nil, errors.WithStack(err)
	}
	if rootSig != 0x8007D0C4 /* Diablo III */ {
		return nil, errors.WithStack(fmt.Errorf("invalid Diablo III root signature %x", rootSig))
	}
	var namedEntriesCount uint32
	if err := binary.Read(r, binary.LittleEndian, &namedEntriesCount); err != nil {
		return nil, errors.WithStack(err)
	}
	namedEntries := []NamedEntry{}
	for i := uint32(0); i < namedEntriesCount; i++ {
		namedEntry := NamedEntry{}
		if err := binary.Read(r, binary.LittleEndian, &namedEntry.ContentHash); err != nil {
			return nil, errors.WithStack(err)
		}
		if err := readAsciiz(r, &namedEntry.Filename); err != nil {
			return nil, errors.WithStack(err)
		}
		namedEntries = append(namedEntries, namedEntry)
	}
	return namedEntries, nil
}

func parseRootDirectory(dirR io.Reader) (
	[]AssetEntry,
	[]AssetIdxEntry,
	[]NamedEntry,
	error,
) {
	var sig uint32
	if err := binary.Read(dirR, binary.LittleEndian, &sig); err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	if sig != 0xeaf1fe87 {
		return nil, nil, nil, errors.WithStack(errors.New("unexpected dir signature"))
	}
	assetEntries := []AssetEntry{}
	var assetCount uint32
	if err := binary.Read(dirR, binary.LittleEndian, &assetCount); err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	for i := uint32(0); i < assetCount; i++ {
		assetEntry := AssetEntry{}
		if err := binary.Read(dirR, binary.LittleEndian, &assetEntry.ContentHash); err != nil {
			return nil, nil, nil, errors.WithStack(err)
		}
		if err := binary.Read(dirR, binary.LittleEndian, &assetEntry.SNOID); err != nil {
			return nil, nil, nil, errors.WithStack(err)
		}
		assetEntries = append(assetEntries, assetEntry)
	}
	assetIdxEntries := []AssetIdxEntry{}
	var assetIdxCount uint32
	if err := binary.Read(dirR, binary.LittleEndian, &assetIdxCount); err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	for i := uint32(0); i < assetIdxCount; i++ {
		assetIdxEntry := AssetIdxEntry{}
		if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.ContentHash); err != nil {
			return nil, nil, nil, errors.WithStack(err)
		}
		if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.SNOID); err != nil {
			return nil, nil, nil, errors.WithStack(err)
		}
		if err := binary.Read(dirR, binary.LittleEndian, &assetIdxEntry.FileIndex); err != nil {
			return nil, nil, nil, errors.WithStack(err)
		}
		assetIdxEntries = append(assetIdxEntries, assetIdxEntry)
	}
	namedEntries := []NamedEntry{}
	var namedCount uint32
	if err := binary.Read(dirR, binary.LittleEndian, &namedCount); err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	for i := uint32(0); i < namedCount; i++ {
		namedEntry := NamedEntry{}
		if err := binary.Read(dirR, binary.LittleEndian, &namedEntry.ContentHash); err != nil {
			return nil, nil, nil, errors.WithStack(err)
		}
		if err := readAsciiz(dirR, &namedEntry.Filename); err != nil {
			return nil, nil, nil, errors.WithStack(err)
		}
		namedEntries = append(namedEntries, namedEntry)
	}
	return assetEntries, assetIdxEntries, namedEntries, nil
}

func parseCoreToc(coreTocR io.ReadSeeker) (map[uint32]SnoInfo, error) {
	coreTocHeader := CoreTocHeader{}
	if err := binary.Read(coreTocR, binary.LittleEndian, &coreTocHeader); err != nil {
		return nil, errors.WithStack(err)
	}
	coreTocHeaderSize := uint32(unsafe.Sizeof(CoreTocHeader{}))
	snoInfos := map[uint32]SnoInfo{}
	for i := uint32(0); i < snoGroupSize; i++ {
		if coreTocHeader.EntryCounts[i] == 0 {
			continue
		}
		if _, err := coreTocR.Seek(int64(coreTocHeaderSize+coreTocHeader.EntryOffsets[i]),
			io.SeekStart); err != nil {
			return nil, errors.WithStack(err)
		}
		for j := uint32(0); j < coreTocHeader.EntryCounts[i]; j++ {
			var snoGroupID uint32 //index of SnoExtensions
			if err := binary.Read(coreTocR, binary.LittleEndian, &snoGroupID); err != nil {
				return nil, errors.WithStack(err)
			}
			var snoID uint32
			if err := binary.Read(coreTocR, binary.LittleEndian, &snoID); err != nil {
				return nil, errors.WithStack(err)
			}
			var nameOffset uint32
			if err := binary.Read(coreTocR, binary.LittleEndian, &nameOffset); err != nil {
				return nil, errors.WithStack(err)
			}
			//names are stored after all entries
			currentPos := coreTocHeaderSize + coreTocHeader.EntryOffsets[i] + 4*3*j
			nameOffset = coreTocHeaderSize + coreTocHeader.EntryOffsets[i] + 4*3*coreTocHeader.EntryCounts[i] + nameOffset
			if _, err := coreTocR.Seek(int64(nameOffset), io.SeekStart); err != nil {
				return nil, errors.WithStack(err)
			}
			var name string
			if err := readAsciiz(coreTocR, &name); err != nil {
				return nil, err
			}
			if _, err := coreTocR.Seek(int64(currentPos), io.SeekStart); err != nil {
				return nil, errors.WithStack(err)
			}
			snoInfos[snoID] = SnoInfo{
				Filename:   name,
				SnoGroupID: snoGroupID,
			}
		}
	}
	return snoInfos, nil
}

func parsePackages(packagesR io.Reader) (map[string]string, error) {
	var sig uint32
	if err := binary.Read(packagesR, binary.LittleEndian, &sig); err != nil {
		return nil, errors.WithStack(err)
	}
	if sig != 0xAABB0002 {
		return nil, errors.WithStack(fmt.Errorf("invalid Data_D3\\PC\\Misc\\Packages.dat signature %x", sig))
	}
	nameToExt := map[string]string{}
	var namesCount uint32
	if err := binary.Read(packagesR, binary.LittleEndian, &namesCount); err != nil {
		return nil, errors.WithStack(err)
	}
	for i := uint32(0); i < namesCount; i++ {
		var name string
		if err := readAsciiz(packagesR, &name); err != nil {
			return nil, errors.WithStack(err)
		}
		if len(name) < 4 {
			return nil, errors.WithStack(errors.New("invalid name length"))
		}
		nameToExt[name[:len(name)-4]] = path.Ext(name)
	}
	return nameToExt, nil
}

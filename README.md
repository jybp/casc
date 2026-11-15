# casc

## Library

Library to extract files from the CASC file system used by Blizzard games. 
Files can be extracted locally from an installed game or online from Blizzard's CDN.  
Full documentation available at: https://godoc.org/github.com/jybp/casc

### Example

```
package example

import (
    "github.com/jybp/casc"
    "net/http"
)

func example() {
    explorer, err = casc.Online(casc.Warcraft3, casc.RegionUS, casc.RegionUS, http.DefaultClient)
    // Or fetch files locally using:
    // explorer, err = casc.Local("/Applications/Warcraft III")
    // explorer, err = casc.Local("C:\Program Files\Warcraft III") 
    if err != nil {
        // Handle error
    }
    for _, filename := range explorer.Files() {
        data, err := explorer.Extract(filename)
        if err == casc.ErrNotFound {
            continue
        }
        if err != nil {
            // Handle error
        }
        // Do something with data
    }
}
```

## cmd/casc

A command line program to extract files from a local installation or from Blizzard's CDN.  
You can download the latest release here: https://github.com/jybp/casc/releases

### Usage
```
  -app string
        app code (d3, osi, s1, w3)
  -cdn string
        cdn region (default "us")
  -convert string
        also convert D2R .texture files: dds (lossless, all mipmaps), png (largest mipmap) or both
  -dir string
        game install directory
  -flat
        write files directly inside the output directory instead of keeping the CASC folders
  -j int
        number of files extracted in parallel (default 8)
  -l    list files (default when stdin is a terminal)
  -no-raw
        with -convert, do not write the original .texture files
  -o string
        output directory for extracted files (default "extract" with -x)
  -pattern string
        only keep files matching this regular expression (case insensitive)
  -preset string
        comma separated list of file groups, see -presets
  -presets
        print the available presets
  -region string
        app region code (default "us")
  -skip-existing
        do not extract files already present in the output directory
  -v    verbose
  -x    extract every matching file
```

Without `-l` or `-x`, files are listed when stdin is a terminal, otherwise the filenames read from stdin are extracted.
Extracted files keep their CASC folders (`:` is replaced by `/`), unless `-flat` is used.

### Examples

List all Warcraft III files :
```
$ casc.exe -dir "C:\Program Files\Warcraft III"
$ casc -dir "/Applications/Warcraft III"
$ casc -app w3
```

Extract all Warcraft III files that are inside the 'War3.w3mod:Movies' folder from Blizzard's CDN into the current directory:
```
$ ./casc -app w3 | grep '^War3.w3mod:Movies/' | ./casc -app w3
```

### Diablo II Resurrected

The install folder must be named `Diablo II Resurrected`. Files are named like `data:data/hd/character/enemy/duriel/torso_lod0.model`.

List the presets (characters, enemies, npcs, environments, objects, items, vfx, ui, audio, data, video):
```
casc.exe -presets
```

Extract the playable characters, the monsters and the levels, converting textures to PNG and DDS:
```
casc.exe -dir "C:\Program Files (x86)\Diablo II Resurrected" -x -o d2r -preset characters,enemies,environments -convert both
```

Extract everything (~30GB):
```
casc.exe -dir "C:\Program Files (x86)\Diablo II Resurrected" -x -o d2r
```

Extract only Andariel:
```
casc.exe -dir "C:\Program Files (x86)\Diablo II Resurrected" -x -o d2r -pattern "/andariel/" -convert png
```

D2R file formats:

| Extension | Content | Notes |
| --- | --- | --- |
| `.texture` | HD textures | Converted with `-convert` (BC1/BC3/BC4/BC5/RGBA8). `_alb` albedo, `_nrm` normal map (DXT5nm, rebuilt as a regular normal map in PNG), `_orm` occlusion/roughness/metalness. |
| `.model` | HD meshes | Granny 3D (`.gr2`, version 7, 64-bit). Open with Granny compatible tools. |
| `.skeleton`, `.animations` | HD rigs and animations | Granny 3D. |
| `.json` | HD materials, variants, presets | Plain text. |
| `.dcc`, `.dc6`, `.cof` | Original 2D sprites and their layer composition | `data:data/global/...` |
| `.dt1`, `.ds1` | Original 2D tiles and level presets | `data:data/global/tiles/...` |
| `.txt`, `.tbl`, `.bin` | Game data tables and strings | |
| `.flac`, `.webm` | Audio and videos | |

## Support

| App | Code | Status |
| --- | --- | --- |
| Diablo II Resurrected | osi | done |
| Diablo III | d3 | done |
| StarCraft | s1 | done |
| Warcraft III | w3 | done |

## Thanks

- [ladislav-zezula](https://github.com/ladislav-zezula) for [CascLib](https://github.com/ladislav-zezula/CascLib)
- [TOM_RUS](https://github.com/tomrus88) for [CASCExplorer](https://github.com/WoW-Tools/CASCExplorer)
- [WoWDev Wiki](https://wowdev.wiki/CASC) contributors

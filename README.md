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
        app code
  -cdn string
        cdn region (default "us")
  -dir string
        game install directory
  -o string
        output directory for extracted files
  -region string
        app region code (default "us")
  -v    verbose
```

### Examples

List all Warcraft III files :
```
$ casc.exe -dir "C:\Program Files\Warcraft III"
$ casc -dir "/Applications/Warcraft III"
$ casc -app w3
```

Extract all Warcraft III files that are inside the 'War3.w3mod:Movies' folder from Blizzard's CDN into the current directoy:
```
$ ./casc -app w3 | grep '^War3.w3mod:Movies/' | ./casc -app w3
```

## Support

| App | Code | Status |
| --- | --- | --- |
| Diablo III | d3 | done |
| StarCraft | s1 | done |
| Warcraft III | w3 | done |

## Thanks

- [ladislav-zezula](https://github.com/ladislav-zezula) for [CascLib](https://github.com/ladislav-zezula/CascLib)
- [TOM_RUS](https://github.com/tomrus88) for [CASCExplorer](https://github.com/WoW-Tools/CASCExplorer)
- [WoWDev Wiki](https://wowdev.wiki/CASC) contributors

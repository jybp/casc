# casc

Library to extract files from the CASC file system used by Blizzard games. 
Files can be extracted locally from an installed game or online from Blizzard's CDN. 

## Getting Started

```
go get -u github.com/jybp/casc
```

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

## Support

| App | Status |
| --- | --- |
| Diablo III | done |
| StarCraft | done |
| Warcraft III | done |


## Documentation

https://godoc.org/github.com/jybp/casc

## Examples

Aside from the examples available in the documentation, a binary package is provided as an example of how the library can be used. Please refer to [cmd/casc-extract](cmd/casc-extract/).


## Thanks

- [ladislav-zezula](https://github.com/ladislav-zezula) for [CascLib](https://github.com/ladislav-zezula/CascLib)
- [TOM_RUS](https://github.com/tomrus88) for [CASCExplorer](https://github.com/WoW-Tools/CASCExplorer)
- [WoWDev Wiki](https://wowdev.wiki/CASC) contributors
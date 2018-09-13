# casc-explorer

## Installation
```
$ go get -u github.com/jybp/casc/cmd/casc-extract
```	

## Examples

Extract all Warcraft III files from a local installation:
```
$ casc-extract -dir "/Applications/Warcraft III"
```
```
$ casc-extract -dir "C:\Program Files\Warcraft III"
```

Extract a Warcraft III file from Blizzard's CDN:
```
$ casc-extract -app w3 -pattern "War3.mpq:Movies/HumanEd.avi"
```

Please refer to the documentation for more examples.

## Documentation

https://godoc.org/github.com/jybp/casc/cmd/casc-extract
package common

import (
	"bytes"
	"encoding/hex"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
)

// Version describes the local and online version of a product.
// A product colum with the product code will be present inside the
// local .build.info file if "shared storage" is enabled. (https://wowdev.wiki/TACT#Shared_storage)
// See more details in the issue:
// https://github.com/jybp/casc/issues/1
type Version struct {
	Region          string
	BuildConfigHash []byte
	CDNConfigHash   []byte
	Name            string // i.e. A.B.C.XXXXX

	ProductCode string // Optional
}

// ParseLocalBuildInfo parses the .build.info file
func ParseLocalBuildInfo(r io.Reader) ([]Version, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	csv, err := ParseCSV(bytes.NewReader(b), "Branch", "Build Key", "CDN Key", "Version", "Product")
	if err != nil {
		csv, err = ParseCSV(bytes.NewReader(b), "Branch", "Build Key", "CDN Key", "Version")
		if err != nil {
			return nil, err
		}
	}
	versions := []Version{}
	for _, row := range csv {
		builConfigHash, err := hex.DecodeString(row["Build Key"])
		if err != nil {
			return nil, errors.WithStack(errors.New("invalid versions"))
		}
		cdnConfigHash, err := hex.DecodeString(row["CDN Key"])
		if err != nil {
			return nil, errors.WithStack(errors.New("invalid versions"))
		}
		product, _ := row["Product"]
		versions = append(versions, Version{
			Region:          row["Branch"],
			BuildConfigHash: builConfigHash,
			CDNConfigHash:   cdnConfigHash,
			Name:            row["Version"],
			ProductCode:     product,
		})
	}
	return versions, nil
}

// ParseOnlineVersions parses the file located at:
// http://(Region).patch.battle.net:1119/(ProgramCode)/versions
func ParseOnlineVersions(r io.Reader) ([]Version, error) {
	csv, err := ParseCSV(r, "Region", "BuildConfig", "CDNConfig", "VersionsName")
	if err != nil {
		return nil, err
	}
	versions := []Version{}
	for _, row := range csv {
		builConfigHash, err := hex.DecodeString(row["BuildConfig"])
		if err != nil {
			return nil, errors.WithStack(errors.New("invalid versions"))
		}
		cdnConfigHash, err := hex.DecodeString(row["CDNConfig"])
		if err != nil {
			return nil, errors.WithStack(errors.New("invalid versions"))
		}
		versions = append(versions, Version{
			Region:          row["Region"],
			BuildConfigHash: builConfigHash,
			CDNConfigHash:   cdnConfigHash,
			Name:            row["VersionsName"],
		})
	}
	return versions, nil
}

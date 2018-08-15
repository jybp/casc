package common

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

type Version struct {
	BuildConfigHash []byte
	CDNConfigHash   []byte
	Name            string // i.e. A.B.C.XXXXX
}

// ParseVersions tries to parse using ParseBuildInfo and ParseOnlineVersions
func ParseVersions(r io.Reader) (map[string]Version, error) {
	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)
	versions, err := ParseBuildInfo(tee)
	if err != nil {
		versions, err = ParseOnlineVersions(&buf)
		if err != nil {
			return nil, err
		}
	}
	return versions, nil
}

// ParseBuildInfo parses the .build.info file
func ParseBuildInfo(r io.Reader) (map[string]Version, error) {
	return parseVersions(r, "Branch", "Build Key", "CDN Key", "Version")
}

// ParseOnlineVersions parses the file located at:
// http://(Region).patch.battle.net:1119/(ProgramCode)/versions
func ParseOnlineVersions(r io.Reader) (map[string]Version, error) {
	return parseVersions(r, "Region", "BuildConfig", "CDNConfig", "VersionsName")
}

func parseVersions(r io.Reader, region, build, cdn, version string) (map[string]Version, error) {
	csv, err := parseCSV(r)
	if err != nil {
		return nil, err
	}
	versions := map[string]Version{}
	for _, row := range csv {
		region, ok := row[region]
		if !ok {
			return nil, errors.WithStack(fmt.Errorf("invalid version: %+v", row))
		}
		builConfigStr, ok := row[build]
		if !ok {
			return nil, errors.WithStack(fmt.Errorf("invalid version: %+v", row))
		}
		builConfigHash, err := hex.DecodeString(builConfigStr)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		cdnConfigStr, ok := row[cdn]
		if !ok {
			return nil, errors.WithStack(fmt.Errorf("invalid version: %+v", row))
		}
		cdnConfigHash, err := hex.DecodeString(cdnConfigStr)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		versionName, ok := row[version]
		if !ok {
			return nil, errors.WithStack(fmt.Errorf("invalid version: %+v", row))
		}
		versions[region] = Version{
			BuildConfigHash: builConfigHash,
			CDNConfigHash:   cdnConfigHash,
			Name:            versionName,
		}
	}
	return versions, nil
}

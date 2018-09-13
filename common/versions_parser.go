package common

import (
	"encoding/hex"
	"io"

	"github.com/pkg/errors"
)

type Version struct {
	BuildConfigHash []byte
	CDNConfigHash   []byte
	Name            string // i.e. A.B.C.XXXXX
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
	csv, err := ParseCSV(r, region, build, cdn, version)
	if err != nil {
		return nil, err
	}
	versions := map[string]Version{}
	for _, row := range csv {
		builConfigHash, err := hex.DecodeString(row[build])
		if err != nil {
			return nil, errors.WithStack(errors.New("invalid versions"))
		}
		cdnConfigHash, err := hex.DecodeString(row[cdn])
		if err != nil {
			return nil, errors.WithStack(errors.New("invalid versions"))
		}
		versions[row[region]] = Version{
			BuildConfigHash: builConfigHash,
			CDNConfigHash:   cdnConfigHash,
			Name:            row[version],
		}
	}
	return versions, nil
}

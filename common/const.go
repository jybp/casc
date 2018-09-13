package common

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func ngdpHostURL(region string) string {
	return fmt.Sprintf("http://%s.patch.battle.net:1119", region)
}

func NGDPVersionsURL(app, region string) string {
	return fmt.Sprintf("%s/%s/versions", ngdpHostURL(region), app)
}

func NGDPCdnsURL(app, region string) string {
	return fmt.Sprintf("%s/%s/cdns", ngdpHostURL(region), app)
}

const (
	PathTypeConfig = "config"
	PathTypeData   = "data"
)

func Url(cdnHost, cdnPath string, pathType string, hash []byte, index bool) (string, error) {
	h := hex.EncodeToString(hash)
	if len(h) < 4 {
		return "", errors.WithStack(errors.New("invalid hash len"))
	}
	url := "http://" + cdnHost + "/" + cdnPath + "/" + string(pathType) + "/" + string(h[0:2]) + "/" + string(h[2:4]) + "/" + h
	if !index {
		return url, nil
	}
	return url + ".index", nil
}

func CleanPath(path string) string {
	return filepath.Clean(strings.Replace(path, "\\", "/", -1))
}

package common

import "fmt"

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
	//PathTypePatch  = "patch"
)

func Url(cdnHost, cdnPath string, pathType string, hash string, index bool) string {
	//TODO potential Panic
	url := "http://" + cdnHost + "/" + cdnPath + "/" + string(pathType) + "/" + string(hash[0:2]) + "/" + string(hash[2:4]) + "/" + hash
	if !index {
		return url
	}
	return url + ".index"
}

package csac

import (
	"fmt"
)

const (
	RegionUS = "us"
	RegionEU = "eu"
	RegionKR = "kr"
	RegionTW = "tw"
	RegionCN = "cn"
)

func HostURL(region string) string {
	return fmt.Sprintf("http://%s.patch.battle.net:1119", region)
}

func VersionsURL(hostURL, app string) string {
	return fmt.Sprintf("%s/%s/versions", hostURL, app)
}

func CdnsURL(hostURL, app string) string {
	return fmt.Sprintf("%s/%s/cdns", hostURL, app)
}

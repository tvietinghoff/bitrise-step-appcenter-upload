package main

import (
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Conf struct {
	ApkDistribution string `env:"dist_group_list,required"`
	ApkPathList     string `env:"apk_path_list,required"`
	AppId           string `env:"app_id,required"`
	ApiToken        string `env:"api_token,required"`
}

func fail(message string, args ...interface{}) {
	log.Errorf(message, args...)
	os.Exit(1)
}
func splitElements(list []string, sep string) (s []string) {
	var temp []string

	for _, e := range list {
		temp = append(temp, strings.Split(e, sep)...)
	}

	for _, item := range temp {
		item = strings.TrimSpace(item)
		if len(item) > 0 {
			s = append(s, item)
		}
	}
	return
}
func parseAppList(list string) (apps []string) {
	list = strings.TrimSpace(list)
	if len(list) == 0 {
		return nil
	}

	apps = parseList(list)

	return
}

func parseList(list string) []string {
	s := []string{list}
	for _, sep := range []string{"\n", `\n`, "|"} {
		s = splitElements(s, sep)
	}
	return s
}

func main() {

	var conf Conf

	if err := stepconf.Parse(&conf); err != nil {
		fail("step config failed: %v\n", err)
	}

	stepconf.Print(conf)

	buildArtifactPaths := parseAppList(conf.ApkPathList)
	distributionSpecs := make(map[string]string)

	apkDistributionSpecs := parseList(conf.ApkDistribution)
	distributionDefault := ""

	for _, spec := range apkDistributionSpecs {
		spec = strings.TrimSpace(spec)
		if len(spec) == 0 {
			continue
		}
		parts := strings.Split(spec, "=")

		if len(parts) < 2 {
			distributionDefault = spec
			log.Infof("Default distribution group set to '%s'", spec)
		} else {
			distributionSpecs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	distributionCount := 0

APK:

	_, err := exec.LookPath("appcenter")
	if err != nil {
		log.Infof("appcenter cli not found, installing...")
		err := exec.Command("npm", "install", "-g", "appcenter-cli").Run()
		if err != nil {
			fail("Failed to install appcenter cli %v\n", err)
		}
	}

	for _, apk := range buildArtifactPaths {
		extLen := len(filepath.Ext(apk))
		base := filepath.Base(apk)
		apkName := base[:len(base)-extLen]

		// find distribution group configured for variant
		for key, groupName := range distributionSpecs {
			if strings.Contains(apkName, key) {
				log.Infof("Distributing %s to AppCenter distribution group %s\n", apkName, groupName)
				if appcenterUpload(apk, groupName, conf.AppId, conf.ApiToken) {
					distributionCount++
					continue APK
				}
			}
		}
		if distributionDefault != "" {
			// no match found, use default distribution if present
			log.Infof("Distributing %s to default AppCenter distribution group %s\n", apkName, distributionDefault)
			if appcenterUpload(apk, distributionDefault, conf.AppId, conf.ApiToken) {
				distributionCount++
				continue APK
			}
		}
	}

	log.Infof("%d APKs successfully distributed", distributionCount)
	os.Exit(0)
}

func appcenterUpload(apk string, distributionGroup string, appId string, apiToken string) bool {
	// appcenter distribute release -g "$dist_group" -f "$apk_path" -a $app_id --token $api_token
	//# waiting for AppCenter to support uploading symbols for Android
	//# see: https://github.com/microsoft/appcenter-cli/issues/551
	//#
	//# appcenter crashes upload-symbols -s "$mapping_path" -a $app_id --token $api_token

	err := exec.Command("appcenter", "distribute", "release", "-g", distributionGroup, "-f", apk, "-a", appId, "--token", apiToken).Run()
	if err != nil {
		log.Errorf("Failed to distribute '%s' to '%s'\n%v", apk, distributionGroup, err)
		return false
	}
	return true
}

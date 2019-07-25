package main

import (
	"bytes"
	"encoding/json"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Conf struct {
	ApkDistribution string `env:"dist_group_list,required"`
	ApkPathList     string `env:"apk_path_list,required"`
	AppId           string `env:"app_id,required"`
	ApiToken        string `env:"api_token,required"`
	Mappings        int    `env:"upload_mappings"`
}

type OutputInfo []struct {
	ApkData struct {
		VersionCode int    `json:"versionCode"`
		VersionName string `json:"versionName"`
	} `json:"apkData"`
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

	_, err := exec.LookPath("appcenter")
	if err != nil {
		log.Infof("appcenter cli not found, installing...")
		err := exec.Command("npm", "install", "-g", "appcenter-cli").Run()
		if err != nil {
			fail("Failed to install appcenter cli %v\n", err)
		}
	}
	command := exec.Command("appcenter", "--version")
	var outBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &outBuf
	err = command.Run()
	if outBuf.Len() > 0 {
		println(outBuf.String())
	}

APK:
	for _, apk := range buildArtifactPaths {
		extLen := len(filepath.Ext(apk))
		base := filepath.Base(apk)
		apkName := base[:len(base)-extLen]
		mapping := ""
		versionCode := 0
		versionName := ""

		if conf.Mappings > 0 {

			// build mapping file name, assume path is like for apk with last "apk" in path replaced by "mapping"
			parts := strings.Split(filepath.Dir(apk), string(os.PathSeparator))
			for i := len(parts) - 1; i >= 0; i-- {
				if parts[i] == "apk" {
					parts[i] = "mapping"
					break
				}
			}

			mapping = filepath.Join(strings.Join(parts, string(os.PathSeparator)), "mapping.txt")
			var outputInfo OutputInfo

			jsonFileName := filepath.Join(filepath.Dir(apk), "output.json")
			outputJson, err := os.Open(jsonFileName)
			if err == nil {
				err = json.NewDecoder(outputJson).Decode(&outputInfo)
			} else {
				log.Warnf("Failed to read %s: %v\n", jsonFileName, err)
			}
			if err == nil {
				for _, oi := range outputInfo {
					versionCode = oi.ApkData.VersionCode
					versionName = oi.ApkData.VersionName
					if versionCode != 0 && versionName != "" {
						break
					}
				}
			} else {
				log.Warnf("Failed to parse %s: %v\n", jsonFileName, err)
			}
		}

		// find distribution group configured for variant
		for key, groupName := range distributionSpecs {
			if strings.Contains(apkName, key) {
				log.Infof("Distributing %s to AppCenter distribution group %s\n", apkName, groupName)
				if appcenterUpload(apk, groupName, conf.AppId, conf.ApiToken, mapping, versionName, versionCode) {
					distributionCount++
					continue APK
				}
			}
		}
		if distributionDefault != "" {
			// no match found, use default distribution if present
			log.Infof("Distributing %s to default AppCenter distribution group %s\n", apkName, distributionDefault)
			if appcenterUpload(apk, distributionDefault, conf.AppId, conf.ApiToken, mapping, versionName, versionCode) {
				distributionCount++
				continue APK
			}
		}
	}

	log.Infof("%d APKs successfully distributed", distributionCount)
	exitCode := 0
	if distributionCount == 0 {
		exitCode = 1
	}
	os.Exit(exitCode)
}

func appcenterUpload(apk string, distributionGroup string, appId string, apiToken string, mapping string,
	versionName string, versionCode int) bool {
	// appcenter distribute release -g "$dist_group" -f "$apk_path" -a $app_id --token $api_token
	//# waiting for AppCenter to support uploading symbols for Android
	//# see: https://github.com/microsoft/appcenter-cli/issues/551
	var outBuf bytes.Buffer

	command := exec.Command("appcenter", "distribute", "release", "-g", distributionGroup, "-f", apk, "-a", appId,
		"--token", apiToken)
	command.Stdout = &outBuf
	command.Stderr = &outBuf

	err := command.Run()
	if outBuf.Len() > 0 {
		println(outBuf.String())
	}

	if err != nil {
		log.Errorf("Failed to distribute '%s' to '%s'\n%v", apk, distributionGroup, err)
		return false
	}
	if mapping != "" {

		_, err = os.Stat(mapping)
		if err != nil {
			log.Errorf("Error locating mapping file '%s'\n%v", mapping, err)
			return true
		}

		command := exec.Command("appcenter", "crashes", "upload-mappings", "--mapping", mapping, "--version-name",
			versionName, "--version-code", strconv.Itoa(versionCode), "-a", appId, "--token", apiToken)
		outBuf.Reset()
		command.Stdout = &outBuf
		command.Stderr = &outBuf

		err = command.Run()
		if outBuf.Len() > 0 {
			println(outBuf.String())
		}

		if err != nil {
			// mapping upload failure is non-fatal
			log.Errorf("Failed to upload mapping file '%s'\n%v", mapping, err)
		}
	}
	return true
}

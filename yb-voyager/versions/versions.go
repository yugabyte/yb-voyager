/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package versions

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// =============================== YB Versions ===============================

//go:embed yb-versions.json
var ybVersionsJSON []byte

// YBVersionsData represents the structure of yb-versions.json
type YBVersionsData struct {
	Version      []string `json:"version"`
	LatestStable string   `json:"latest_stable"`
}

// LoadYBVersions loads YB versions from the embedded yb-versions.json file
func LoadYBVersions() YBVersionsData {
	var versions YBVersionsData
	err := json.Unmarshal(ybVersionsJSON, &versions)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse yb-versions.json: %v", err))
	}

	if len(versions.Version) == 0 {
		panic("No versions found in yb-versions.json")
	}

	return versions
}

// GetVoyagerSupportedYBVersions returns just the version array from yb-versions.json
func GetVoyagerSupportedYBVersions() []string {
	versions := LoadYBVersions()
	return versions.Version
}

// GetLatestStableYBVersion returns the latest stable version from yb-versions.json
func GetLatestStableYBVersion() string {
	versions := LoadYBVersions()
	if versions.LatestStable == "" {
		panic("No latest_stable version found in yb-versions.json")
	}
	return versions.LatestStable
}

func GetLatestStableYBVersionWithoutBuildNumber() string {
	latestVersion := GetLatestStableYBVersion()
	return strings.Split(latestVersion, "-")[0]
}

// =============================== CI Config ===============================

//go:embed ci-config.json
var ciConfigJSON []byte

// CIConfigData represents the structure of ci-config.json
type CIConfigData struct {
	Versions struct {
		Go          string `json:"go"`
		Java        string `json:"java"`
		Staticcheck string `json:"staticcheck"`
	} `json:"versions"`
	Runner struct {
		Ubuntu string `json:"ubuntu"`
	} `json:"runner"`
}

// LoadCIConfig loads CI configuration from the embedded ci-config.json file
func LoadCIConfig() CIConfigData {
	var config CIConfigData
	err := json.Unmarshal(ciConfigJSON, &config)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse ci-config.json: %v", err))
	}
	return config
}

// GetGoVersion returns the Go version from ci-config.json
func GetGoVersion() string {
	config := LoadCIConfig()
	return config.Versions.Go
}

// GetJavaVersion returns the Java version from ci-config.json
func GetJavaVersion() string {
	config := LoadCIConfig()
	return config.Versions.Java
}

// GetStaticcheckVersion returns the Staticcheck version from ci-config.json
func GetStaticcheckVersion() string {
	config := LoadCIConfig()
	return config.Versions.Staticcheck
}

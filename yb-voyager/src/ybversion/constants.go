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
package ybversion

import "github.com/yugabyte/yb-voyager/yb-voyager/versions"

const (
	SERIES_2_14   = "2.14"
	SERIES_2_18   = "2.18"
	SERIES_2_20   = "2.20"
	SERIES_2024_1 = "2024.1"
	SERIES_2024_2 = "2024.2"
	SERIES_2025_1 = "2025.1"
	SERIES_2025_2 = "2025.2"
	SERIES_2_21   = "2.21"
	SERIES_2_23   = "2.23"
	SERIES_2_25   = "2.25"
)

var LatestStable *YBVersion

var V2024_1_0_0 *YBVersion
var V2024_1_1_0 *YBVersion
var V2024_1_3_1 *YBVersion
var V2024_2_0_0 *YBVersion
var V2024_2_1_0 *YBVersion
var V2024_2_2_2 *YBVersion
var V2024_2_2_3 *YBVersion
var V2024_2_3_0 *YBVersion
var V2024_2_3_1 *YBVersion
var V2024_2_4_0 *YBVersion
var V2025_1_0_0 *YBVersion
var V2025_2_0_0 *YBVersion
var V2_23_0_0 *YBVersion
var V2025_1_1_1 *YBVersion
var V2_25_0_0 *YBVersion
var V2_25_1_0 *YBVersion

func init() {
	var err error
	V2024_1_0_0, err = NewYBVersion("2024.1.0.0")
	if err != nil {
		panic("could not create version 2024.1.0.0")
	}
	V2024_1_1_0, err = NewYBVersion("2024.1.1.0")
	if err != nil {
		panic("could not create version 2024.1.1.0")
	}
	V2024_1_3_1, err = NewYBVersion("2024.1.3.1")
	if err != nil {
		panic("could not create version 2024.1.3.1")
	}
	V2024_2_0_0, err = NewYBVersion("2024.2.0.0")
	if err != nil {
		panic("could not create version 2024.2.0.0")
	}

	V2024_2_1_0, err = NewYBVersion("2024.2.1.0")
	if err != nil {
		panic("could not create version 2024.2.1.0")
	}

	V2024_2_2_2, err = NewYBVersion("2024.2.2.2")
	if err != nil {
		panic("could not create version 2024.2.2.2")
	}

	V2024_2_2_3, err = NewYBVersion("2024.2.2.3")
	if err != nil {
		panic("could not create version 2024.2.2.3")
	}

	V2024_2_3_0, err = NewYBVersion("2024.2.3.0")
	if err != nil {
		panic("could not create version 2024.2.3.0")
	}

	V2024_2_3_1, err = NewYBVersion("2024.2.3.1")
	if err != nil {
		panic("could not create version 2024.2.3.1")
	}

	V2024_2_4_0, err = NewYBVersion("2024.2.4.0")
	if err != nil {
		panic("could not create version 2024.2.4.0")
	}

	V2_23_0_0, err = NewYBVersion("2.23.0.0")
	if err != nil {
		panic("could not create version 2.23.0.0")
	}
	V2_25_0_0, err = NewYBVersion("2.25.0.0")
	if err != nil {
		panic("could not create version 2.25.0.0")
	}

	V2_25_1_0, err = NewYBVersion("2.25.1.0")
	if err != nil {
		panic("could not create version 2.25.1.0")
	}

	V2025_1_0_0, err = NewYBVersion("2025.1.0.0")
	if err != nil {
		panic("could not create version 2025.1.0.0")
	}

	V2025_2_0_0, err = NewYBVersion("2025.2.0.0")
	if err != nil {
		panic("could not create version 2025.2.0.0")
	}

	// Whenever latest_stable version is updated in yb-versions.json, it will be reflected here as well.
	latestStableVersionString := versions.GetLatestStableYBVersionWithoutBuildNumber()
	LatestStable, err = NewYBVersion(latestStableVersionString)
	if err != nil {
		panic("could not create version " + latestStableVersionString)
	}
}

//go:build yb_version_latest_stable

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

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLatestStable(t *testing.T) {
	type Release struct {
		Name string `json:"name"`
	}

	url := "https://api.github.com/repos/yugabyte/yugabyte-db/releases"
	response, err := http.Get(url)
	assert.NoErrorf(t, err, "could not access URL:%q", url)
	defer response.Body.Close()
	if response.StatusCode == 403 {
		t.Skip("skipping test; rate limit exceeded")
	}
	assert.Equal(t, 200, response.StatusCode)

	body, err := io.ReadAll(response.Body)
	assert.NoError(t, err, "could not read contents of response %q")
	var releases []Release
	err = json.Unmarshal(body, &releases)
	assert.NoErrorf(t, err, "could not unmarshal response %q", string(body))
	assert.NotEmpty(t, releases, "no releases found")
	for _, r := range releases {
		// sample -  v2.20.7.1 (Released October 16, 2024)
		releaseName := r.Name
		releaseName = strings.Split(releaseName, " ")[0]
		if releaseName[0] == 'v' {
			releaseName = releaseName[1:]
		}
		releaseName = strings.Trim(releaseName, " ")
		releaseSegs := strings.Split(releaseName, ".")
		assert.Equal(t, 4, len(releaseSegs), "invalid release version: %s. It has %d segments. Version should have exactly 4 segments (A.B.C.D).")
		//Changing the minor version segment to 0 to be able to compare only the major version i.e. aaaa.b.b 
		//e.g. 2024.2.3.1 -> 2024.2.3.0
		releaseSegs[3] = "0"
		rVersion, err := NewYBVersion(strings.Join(releaseSegs, "."))
		assert.NoErrorf(t, err, "could not create version %q", releaseName)
		if rVersion.ReleaseType() == STABLE {
			assert.True(t, LatestStable.GreaterThanOrEqual(rVersion), "%s is not greater than %s", LatestStable, rVersion)
		}
	}
}

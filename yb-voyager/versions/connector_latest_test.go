//go:build connector_latest_stable

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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logicalConnectorTagRe matches logical connector tags of the form:
//
//	dz.<DZMAJOR>.<DZMINOR>.<DZPATCH>.yb.<YBYEAR>.<YBTRACK>[.<COUNTER>]
//
// The trailing counter is optional: the first (GA) release of a YB series omits it
// (e.g. dz.2.5.2.yb.2026.1), while subsequent patches carry one (e.g. dz.2.5.2.yb.2025.2.3).
// It intentionally rejects gRPC tags (which contain ".yb.grpc.") and pre-release tags such
// as ".SNAPSHOT.N" or "-CF-DRAFT" (their suffix is non-numeric, so the counter group fails to match).
var logicalConnectorTagRe = regexp.MustCompile(`^dz\.\d+\.\d+\.\d+\.yb\.(\d+)\.(\d+)(?:\.(\d+))?$`)

// parseLogicalConnectorTag parses a logical connector tag.
// Returns ok=false for any tag that does not match the logical-connector format.
func parseLogicalConnectorTag(tag string) (ok bool, ybYear, ybTrack, counter int) {
	m := logicalConnectorTagRe.FindStringSubmatch(tag)
	if m == nil {
		return false, 0, 0, 0
	}
	// m[1]=ybYear, m[2]=ybTrack, m[3]=counter — strconv.Atoi cannot fail on \d+ groups.
	ybYear, _ = strconv.Atoi(m[1])
	ybTrack, _ = strconv.Atoi(m[2])
	// m[3] is empty for a series-GA tag (no counter); treat that as counter 0 — the
	// series baseline, which sorts below any .1, .2, ... patch of the same series.
	if m[3] != "" {
		counter, _ = strconv.Atoi(m[3])
	}
	return true, ybYear, ybTrack, counter
}

// isNewer reports whether the candidate (ybYear2, ybTrack2, counter2) is strictly newer
// than the reference (ybYear1, ybTrack1, counter1).
//
// Comparison order:
//  1. YB series (ybYear, ybTrack) — the most significant dimension; a connector for a newer
//     YB series is always considered newer regardless of counter.
//  2. counter — within the same YB series, a higher counter is a newer patch release.
func isNewer(ybYear1, ybTrack1, counter1, ybYear2, ybTrack2, counter2 int) bool {
	if ybYear2 != ybYear1 {
		return ybYear2 > ybYear1
	}
	if ybTrack2 != ybTrack1 {
		return ybTrack2 > ybTrack1
	}
	return counter2 > counter1
}

func TestConnectorLatestStable(t *testing.T) {
	type Release struct {
		TagName string `json:"tag_name"`
	}

	// -- 1. Read bundled tag from the versions package accessors --
	bundledTag := GetLogicalConnectorTag()
	bundledOk, bYear, bTrack, bCounter := parseLogicalConnectorTag(bundledTag)
	if !bundledOk {
		t.Fatalf("bundled logical connector tag %q does not match expected format — "+
			"check yb-voyager/versions/connector-versions.json", bundledTag)
	}

	// -- 2. Fetch releases from GitHub --
	url := "https://api.github.com/repos/yugabyte/debezium/releases"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoErrorf(t, err, "could not build request for %q", url)

	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoErrorf(t, err, "could not access URL %q", url)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		t.Skipf("skipping; GitHub API rate limit exceeded (status %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code from %q: %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err, "could not read response body")

	var releases []Release
	err = json.Unmarshal(body, &releases)
	assert.NoErrorf(t, err, "could not unmarshal releases response: %s", string(body))
	assert.NotEmpty(t, releases, "no releases returned from GitHub API")

	// -- 3. Find the latest logical connector release across all tags --
	// Comparison is series-first (ybYear, ybTrack), then counter within the same series.
	foundAny := false
	latestTag := ""
	lYear, lTrack, lCounter := 0, 0, 0

	for _, r := range releases {
		ok, rYear, rTrack, rCounter := parseLogicalConnectorTag(r.TagName)
		if !ok {
			continue // skip gRPC tags, pre-release tags, or any non-logical format
		}
		foundAny = true
		if latestTag == "" || isNewer(lYear, lTrack, lCounter, rYear, rTrack, rCounter) {
			latestTag = r.TagName
			lYear, lTrack, lCounter = rYear, rTrack, rCounter
		}
	}

	if !foundAny {
		t.Fatalf("no logical connector tags found in GitHub releases for yugabyte/debezium — " +
			"check the API response or the tag format regexp")
	}

	// -- 4. Assert bundled tag is not behind the latest available release --
	if isNewer(bYear, bTrack, bCounter, lYear, lTrack, lCounter) {
		t.Errorf(
			"bundled logical connector is behind the latest available release.\n"+
				"  bundled : %s\n"+
				"  latest  : %s\n"+
				"Remediation: bump yb-voyager/versions/connector-versions.json to the latest connector release "+
				"and follow the connector-version section of the update-yb-latest-stable skill.",
			bundledTag,
			fmt.Sprintf("%s (series %d.%d, counter %d)", latestTag, lYear, lTrack, lCounter),
		)
	}
}

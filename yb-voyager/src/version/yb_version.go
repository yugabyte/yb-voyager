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

package version

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/go-version"
	"golang.org/x/exp/slices"
)

var supportedYBVersionStableSeriesOld = []string{"2.20"}
var supportedYBVersionStableSeries = []string{"2024.1"}
var supportedYBVersionPreviewSeries = []string{"2.21"}

var allSupportedYBVersionSeries = append(append(supportedYBVersionStableSeries, supportedYBVersionPreviewSeries...), supportedYBVersionStableSeriesOld...)

const (
	STABLE     = "stable"
	PREVIEW    = "preview"
	STABLE_OLD = "stable_old"
)

type YBVersion struct {
	V *version.Version
}

func NewYBVersion(v string) (*YBVersion, error) {
	v1, err := version.NewVersion(v)
	if err != nil {
		return nil, err
	}

	ybv := &YBVersion{v1}
	origSegLen := ybv.originalSegmentsLen()
	if origSegLen < 2 || origSegLen > 4 {
		return nil, fmt.Errorf("Invalid YB version: %s. Version should have between min 2 and max 4 segments (A.B.C.D). Version %s has ybv.originalSegmentsLen()", v, v)
	}

	if !slices.Contains(allSupportedYBVersionSeries, ybv.Series()) {
		return nil, fmt.Errorf("Unsupported YB version series: %s. Supported YB version series = %v", ybv.Series(), allSupportedYBVersionSeries)
	}
	return ybv, nil
}

func (ybv *YBVersion) Series() string {
	return joinIntsWith(ybv.V.Segments()[:2], ".")
}

func (ybv *YBVersion) ReleaseType() string {
	series := ybv.Series()
	if slices.Contains(supportedYBVersionStableSeries, series) {
		return STABLE
	} else if slices.Contains(supportedYBVersionPreviewSeries, series) {
		return PREVIEW
	} else if slices.Contains(supportedYBVersionStableSeriesOld, series) {
		return STABLE_OLD
	} else {
		panic("unknown release type for series: " + series)
	}
}

func (ybv *YBVersion) originalSegmentsLen() int {
	orig := ybv.V.Original()
	segments := strings.Split(orig, ".")
	return len(segments)
}

func (ybv *YBVersion) ComparePrefix(other *YBVersion) int {
	myOriginalSegLen := ybv.originalSegmentsLen()
	otherOriginalSegLen := other.originalSegmentsLen()
	minSegLen := min(myOriginalSegLen, otherOriginalSegLen)

	ybvMin, err := version.NewVersion(joinIntsWith(ybv.V.Segments()[:minSegLen], "."))
	if err != nil {
		panic(err)
	}
	otherMin, err := version.NewVersion(joinIntsWith(other.V.Segments()[:minSegLen], "."))
	if err != nil {
		panic(err)
	}
	return ybvMin.Compare(otherMin)
}

func (ybv *YBVersion) PrefixGreaterThanOrEqual(other *YBVersion) bool {
	return ybv.ComparePrefix(other) >= 0
}

func (ybv *YBVersion) PrefixLessThan(other *YBVersion) bool {
	return ybv.ComparePrefix(other) < 0
}

func (ybv *YBVersion) String() string {
	return ybv.V.Original()
}

func joinIntsWith(ints []int, delimiter string) string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strings.Join(strs, delimiter)
}

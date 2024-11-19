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

/*
YBVersion is a wrapper around hashicorp/go-version.Version that adds some Yugabyte-specific
functionality.
 1. It only supports versions with 2, 3, or 4 segments (A.B, A.B.C, A.B.C.D).
 2. It only accepts one of supported Yugabyte version series.
 3. It provides a method to compare versions based on common prefix of segments. This is useful in cases
    where the version is not fully specified (e.g. 2.20 vs 2.20.7.0).
    If the user specifies 2.20, a reasonable assumption is that they would be on the latest 2.20.x.y version.
    Therefore, we want only want to compare the prefix 2.20 between versions and ignore the rest.
*/
type YBVersion struct {
	*version.Version
}

func NewYBVersion(v string) (*YBVersion, error) {
	v1, err := version.NewVersion(v)
	if err != nil {
		return nil, err
	}

	ybv := &YBVersion{v1}
	origSegLen := ybv.OriginalSegmentsLen()
	if origSegLen < 2 || origSegLen > 4 {
		return nil, fmt.Errorf("invalid YB version: %s. Version should have between min 2 and max 4 segments (A.B.C.D). Version %s has ybv.originalSegmentsLen()", v, v)
	}

	if !slices.Contains(allSupportedYBVersionSeries, ybv.Series()) {
		return nil, fmt.Errorf("unsupported YB version series: %s. Supported YB version series = %v", ybv.Series(), allSupportedYBVersionSeries)
	}
	return ybv, nil
}

func (ybv *YBVersion) Series() string {
	return joinIntsWith(ybv.Segments()[:2], ".")
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

func (ybv *YBVersion) OriginalSegmentsLen() int {
	orig := ybv.Original()
	segments := strings.Split(orig, ".")
	return len(segments)
}

/*
Compare the common prefix of segments between two versions.
Similar to method go-version.Version.Compare, but only compares the common prefix of segments.
This is useful in cases where the version is not fully specified (e.g. 2.20 vs 2.20.7.0).
If the user wants to compare 2.20 and 2.20.7.0, a reasonable assumption is that they would be on the latest 2.20.x.y version.
Therefore, we want only want to compare the prefix 2.20 between versions and ignore the rest.

This returns -1, 0, or 1 if this version is smaller, equal,
or larger than the other version, respectively.
*/
func (ybv *YBVersion) CompareCommonPrefix(other *YBVersion) (int, error) {
	if ybv.Series() != other.Series() {
		return 0, fmt.Errorf("cannot compare versions with different series: %s and %s", ybv.Series(), other.Series())
	}
	myOriginalSegLen := ybv.OriginalSegmentsLen()
	otherOriginalSegLen := other.OriginalSegmentsLen()
	minSegLen := min(myOriginalSegLen, otherOriginalSegLen)

	ybvMin, err := version.NewVersion(joinIntsWith(ybv.Segments()[:minSegLen], "."))
	if err != nil {
		panic(err)
	}
	otherMin, err := version.NewVersion(joinIntsWith(other.Segments()[:minSegLen], "."))
	if err != nil {
		panic(err)
	}
	return ybvMin.Compare(otherMin), nil
}

func (ybv *YBVersion) CommonPrefixGreaterThanOrEqual(other *YBVersion) (bool, error) {
	res, err := ybv.CompareCommonPrefix(other)
	if err != nil {
		return false, err
	}
	return res >= 0, nil
}

func (ybv *YBVersion) CommonPrefixLessThan(other *YBVersion) (bool, error) {
	res, err := ybv.CompareCommonPrefix(other)
	if err != nil {
		return false, err
	}
	return res < 0, nil
}

func (ybv *YBVersion) String() string {
	return ybv.Original()
}

// override the UnmarshalText method of Version.
// UnmarshalText implements encoding.TextUnmarshaler interface.
func (ybv *YBVersion) UnmarshalText(b []byte) error {
	temp, err := NewYBVersion(string(b))
	if err != nil {
		return err
	}

	*ybv = *temp
	return nil
}

func joinIntsWith(ints []int, delimiter string) string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strings.Join(strs, delimiter)
}

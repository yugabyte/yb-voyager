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
	"strconv"
	"strings"

	goerrors "github.com/go-errors/errors"
	"github.com/hashicorp/go-version"
	"github.com/samber/lo"
	"golang.org/x/exp/slices"
)

// Reference - https://docs.yugabyte.com/preview/releases/ybdb-releases/
var supportedYBVersionStableSeriesOld = []string{SERIES_2_14, SERIES_2_18, SERIES_2_20}
var supportedYBVersionStableSeries = []string{SERIES_2024_1, SERIES_2024_2, SERIES_2025_1, SERIES_2025_2}
var supportedYBVersionPreviewSeries = []string{SERIES_2_21, SERIES_2_23, SERIES_2_25}

var allSupportedYBVersionSeries = lo.Flatten([][]string{supportedYBVersionStableSeries, supportedYBVersionPreviewSeries, supportedYBVersionStableSeriesOld})
var ErrUnsupportedSeries = goerrors.Errorf("unsupported YB version series. Supported YB version series = %v", allSupportedYBVersionSeries)

const (
	STABLE     = "stable"
	PREVIEW    = "preview"
	STABLE_OLD = "stable_old"
)

/*
YBVersion is a wrapper around hashicorp/go-version.Version that adds some Yugabyte-specific
functionality.
 1. It only supports versions with 4 segments (A.B.C.D). No build number is allowed.
 2. It only accepts one of supported Yugabyte version series.
*/
type YBVersion struct {
	*version.Version
}

func NewYBVersion(v string) (*YBVersion, error) {
	v1, err := version.NewVersion(v)
	if err != nil {
		return nil, err
	}

	// Do not allow build number in the version. for example, 2024.1.1.0-b123
	if v1.Prerelease() != "" {
		return nil, goerrors.Errorf("invalid YB version: %s. Build number is not supported. Version should be of format (A.B.C.D) ", v)
	}

	ybv := &YBVersion{v1}
	origSegLen := ybv.OriginalSegmentsLen()
	if origSegLen != 4 {
		return nil, goerrors.Errorf("invalid YB version: %s. It has %d segments. Version should have exactly 4 segments (A.B.C.D).", v, origSegLen)
	}

	if !slices.Contains(allSupportedYBVersionSeries, ybv.Series()) {
		return nil, ErrUnsupportedSeries
	}
	return ybv, nil
}

// The first two segments essentially represent the release series
// as per https://docs.yugabyte.com/preview/releases/ybdb-releases/
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

// This returns the len of the segments in the original
// input. For instance if input is 2024.1,
// go-version.Version.Segments() will return [2024, 1, 0, 0]
// original = 2024.1
// originalSegmentsLen  = 2 ([2024,1])
func (ybv *YBVersion) OriginalSegmentsLen() int {
	orig := ybv.Original()
	segments := strings.Split(orig, ".")
	return len(segments)
}

func (ybv *YBVersion) GreaterThanOrEqual(other *YBVersion) bool {
	//TODO: should fail in case the ybv and other version is of different release type
	return ybv.Version.GreaterThanOrEqual(other.Version)
}

func (ybv *YBVersion) Equal(other *YBVersion) bool {
	return ybv.Version.Equal(other.Version)
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

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
package compareperf

import (
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

type QueryPerformanceComparator struct {
	SourceQueryStats []QueryStats
	TargetQueryStats []QueryStats
}

func NewQueryPerformanceComparator(assessmentDBPath string, targetDB *tgtdb.TargetYugabyteDB) (*QueryPerformanceComparator, error) {
	adb, err := migassessment.NewAssessmentDB("")
	if err != nil {
		return nil, fmt.Errorf("failed to open assessment database: %w", err)
	}

	sourceQueryStats, err := adb.GetSourceQueryStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get source query stats: %w", err)
	}

	targetQueryStats, err := targetDB.CollectPgStatStatements()
	if err != nil {
		return nil, fmt.Errorf("failed to get target query stats: %w", err)
	}

	return &QueryPerformanceComparator{
		SourceQueryStats: convertPgssToQueryStats(sourceQueryStats),
		TargetQueryStats: convertPgssToQueryStats(targetQueryStats),
	}, nil
}

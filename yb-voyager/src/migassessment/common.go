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
package migassessment

import (
	"net/http"
)

var AssessmentMetadataDir string

type Record map[string]any

var SizingReport *SizingAssessmentReport

type SizingAssessmentReport struct {
	ColocatedTables                 []string
	ColocatedReasoning              string
	ShardedTables                   []string
	NumNodes                        float64
	VCPUsPerInstance                float64
	MemoryPerInstance               float64
	OptimalSelectConnectionsPerNode int64
	OptimalInsertConnectionsPerNode int64
	MigrationTimeTakenInMin         float64
	ParallelVoyagerThreadsSharded   int64
	ParallelVoyagerThreadsColocated int64
	FailureReasoning                string
}

func checkInternetAccess() (ok bool) {
	_, err := http.Get("http://clients3.google.com/generate_204")
	return err == nil
}

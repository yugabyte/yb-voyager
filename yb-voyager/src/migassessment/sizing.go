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

type SizingRecommendation struct {
	NodeCount        int
	VcpuCountPerNode int
	MemGBPerNode     int
	StorageGBPerNode int
}

type SizingParams struct {
	// add any sizing specific parameters required from user here
	// dummy params for now
	UseNvme           bool `toml:"use_nvme"`
	MultiAzDeployment bool `toml:"multi_az_deployment"`
}

func SizingAssessment() error {
	// Can read user input from `params.SizingParams`
	// If sizing algorithm requires output of sharding,
	// it can directly read the input from
	// the `report.ShardingReport`

	// Load CSV data files.
	// Run/call sizer.
	// Populate `FinalReport.SizingReport`

	return nil
}

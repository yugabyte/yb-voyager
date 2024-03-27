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
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"path/filepath"
)

type SizingReport struct {
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

type SizingTableCountRecord struct {
	Count string `json:"count"`
}

func SizingAssessment() error {
	// load the assessment data
	assessmentDataDir := filepath.Join(ExportDir, "assessment", "data")
	tableCountFile := filepath.Join(assessmentDataDir, "sizing__num-tables-count.csv")

	log.Infof("loading metadata files for sharding assessment")
	tableCount, err := loadCSVDataFile[SizingTableCountRecord](tableCountFile)
	if err != nil {
		log.Errorf("failed to load table count file: %v", err)
		return fmt.Errorf("failed to load table count file: %w", err)
	}

	utils.PrintAndLog("performing sizing assessment...")
	for _, tableC := range tableCount {
		log.Infof("count: %+v", tableC)
	}
	// core assessment logic goes here - maybe call some external package APIs to perform the assessment
	FinalReport.SizingReport = &SizingReport{
		// replace with actual assessment results
		NodeCount:        3,
		VcpuCountPerNode: 8,
		MemGBPerNode:     16,
		StorageGBPerNode: 250,
	}
	return nil
}

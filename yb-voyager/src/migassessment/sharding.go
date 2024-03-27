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
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type ShardingReport struct {
	ColocatedTables []string
	ShardedTables   []string
}

type ShardingTableSizesRecord struct {
	SchemaName string `json:"schema_name"`
	TableName  string `json:"table_name"`
	TableSize  int64  `json:"table_size,string"` // specified string tag option to handle conversion(string -> int64) during unmarshalling
}

type ShardingTableIOPSRecord struct {
	SchemaName string `json:"schema_name"`
	TableName  string `json:"table_name"`
	SeqReads   int64  `json:"seq_reads,string"`
	RowWrites  int64  `json:"row_writes,string"`
}

type ShardingParams struct {
	// add any sharding specific parameters required from user here
	NumNodes        int `toml:"num_nodes"`
	NumCoresPerNode int `toml:"num_cores_per_node"`
}

func ShardingAssessment() error {
	// load the assessment data
	tableSizesFpath := filepath.Join(AssessmentDataDir, "sharding__table-sizes.csv")
	tableIOPSFpath := filepath.Join(AssessmentDataDir, "sharding__table-iops.csv")

	log.Infof("loading metadata files for sharding assessment")
	tableSizes, err := loadCSVDataFile[ShardingTableSizesRecord](tableSizesFpath)
	if err != nil {
		log.Errorf("failed to load table sizes file: %v", err)
		return fmt.Errorf("failed to load table size file: %w", err)
	}

	tableIOPS, err := loadCSVDataFile[ShardingTableIOPSRecord](tableIOPSFpath)
	if err != nil {
		log.Errorf("failed to load table IOPS file: %v", err)
		return fmt.Errorf("failed to load table IOPS file: %w", err)
	}

	for _, tableSize := range tableSizes {
		log.Infof("table size: %+v", tableSize)
	}
	for _, tableIOPS := range tableIOPS {
		log.Infof("table IOPS: %+v", tableIOPS)
	}

	utils.PrintAndLog("performing sharding assessment...")

	// core assessment logic goes here - maybe call some external package APIs to perform the assessment

	FinalReport.ShardingReport = &ShardingReport{
		// replace with actual assessment results
		ColocatedTables: []string{"table1", "table2"},
		ShardedTables:   []string{"table3", "table4"},
	}

	return nil
}

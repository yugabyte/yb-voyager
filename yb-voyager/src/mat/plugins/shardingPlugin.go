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
package mat

import (
	"fmt"
	"sort"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type ShardingPlugin struct{}

func newShardingPlugin() *ShardingPlugin {
	return &ShardingPlugin{}
}

/*
   NUM_IDEAL_OBJ_TABLETS_PER_TSERVER = 'num_ideal_obj_tablets_per_tserver'
   COLOCATED_MAX_SIZE_BYTES = 'colocated_max_size_bytes'
   COLOCATED_MAX_ROW_COUNT = 'colocated_max_row_count'
   COLOCATED_MAX_IOPS = 'colocated_max_iops'
   MIN_OBJECTS_FOR_COLOCATION = 'min_objects_for_colocation'
*/

// TODO: sample code, needs to fetch this info from KnowledgeBase
func (sp *ShardingPlugin) GetConfig() map[string]any {
	return map[string]any{
		"num_ideal_obj_tablets_per_tserver":  int64(100),
		"colocated_max_size_bytes":           int64(1000000000), // 1GB
		"colocated_max_row_count":            int64(1000000),
		"colocated_max_iops":                 int64(1000),
		"min_objects_for_colocation":         int64(3),
		"min_objects_for_sharding":           int64(3),
		"min_objects_for_sharding_per_table": int64(3),
	}
}

func (sp *ShardingPlugin) RunAssessment(queryResults map[string]QueryResult, userInput map[string]any) (any, error) {
	result := map[string][]string{
		"colocated": {},
		"sharded":   {},
	}

	log.Infof("Running sharding plugin")

	thresholds_for_colocation := sp.GetConfig()

	// TODO: if number of sql objects are less than the MINIMUM_TABLES_TO_COLOCATE then NO COLOCATION

	table_sizes_info := queryResults["table-sizes"]
	// fmt.Printf("before sorting table_sizes_info: %v\n\n", table_sizes_info)
	// sort this slice of maps by table size
	sort.Slice(table_sizes_info, func(i, j int) bool {
		a, err := strconv.ParseInt(table_sizes_info[i]["table_size"].(string), 10, 64)
		if err != nil {
			log.Errorf("error parsing table size: %v", err)
			return false
		}
		b, err := strconv.ParseInt(table_sizes_info[j]["table_size"].(string), 10, 64)
		if err != nil {
			log.Errorf("error parsing table size: %v", err)
			return false
		}
		return a > b
	})
	// fmt.Printf("after sorting table_sizes_info: %v\n\n", table_sizes_info)

	// similarly sort table_rows_info
	table_rows_info := queryResults["table-row-counts"]
	// fmt.Printf("before sorting table_rows_info: %v\n\n", table_rows_info)
	sort.Slice(table_rows_info, func(i, j int) bool {
		a, err := strconv.ParseInt(table_rows_info[i]["row_count"].(string), 10, 64)
		if err != nil {
			log.Errorf("error parsing table rows: %v", err)
			return false
		}
		b, err := strconv.ParseInt(table_rows_info[j]["row_count"].(string), 10, 64)
		if err != nil {
			log.Errorf("error parsing table rows: %v", err)
			return false
		}
		return a > b
	})
	// fmt.Printf("after sorting table_rows_info: %v\n", table_rows_info)

	table_iops_info := queryResults["table-iops"]
	// fmt.Printf("table_iops_info: %v\n\n", table_iops_info)
	var total_num_colocated_tables, total_colocated_tablet_size, total_colocated_num_rows int64
	if len(table_sizes_info) > 0 {
		for _, table_size_info := range table_sizes_info {
			// fmt.Printf("table_size_info: %v\n", table_size_info)
			schema := table_size_info["schema_name"].(string)
			table := table_size_info["table_name"].(string)

			if total_num_colocated_tables <= thresholds_for_colocation["num_ideal_obj_tablets_per_tserver"].(int64) {
				if total_colocated_tablet_size <= thresholds_for_colocation["colocated_max_size_bytes"].(int64) &&
					total_colocated_num_rows <= thresholds_for_colocation["colocated_max_row_count"].(int64) {

					read_write_iops := getReadWriteIopsForTable(schema, table, table_iops_info)
					if read_write_iops <= thresholds_for_colocation["colocated_max_iops"].(int64) {
						total_num_colocated_tables++
						table_size, _ := strconv.ParseInt(table_size_info["table_size"].(string), 10, 64)
						total_colocated_tablet_size += table_size
						total_colocated_num_rows += getRowCountForTable(schema, table, table_rows_info)

						result["colocated"] = append(result["colocated"], fmt.Sprintf("%s.%s", schema, table))
					} else {
						log.Infof("table %s.%s has more read/write iops than the threshold", schema, table)
						result["sharded"] = append(result["sharded"], fmt.Sprintf("%s.%s", schema, table))
					}
				} else {
					log.Infof("table %s.%s has more size or row count than the threshold", schema, table)
					result["sharded"] = append(result["sharded"], fmt.Sprintf("%s.%s", schema, table))
				}
			} else {
				log.Infof("table %s.%s has more number of colocated tables than the threshold", schema, table)
				result["sharded"] = append(result["sharded"], fmt.Sprintf("%s.%s", schema, table))
			}
		}
	} else if len(table_rows_info) > 0 {
		// same logic as above but using table_rows_info
	} else {
		log.Errorf("no table size or row count info found")
		return nil, fmt.Errorf("no table size or row count info found")
	}

	return result, nil
}

func (sp *ShardingPlugin) GetHtmlTemplate() string {
	return ""
}

func (sp *ShardingPlugin) ModifySchema(report map[string]any) error {
	return nil
}

func (sp *ShardingPlugin) GetName() string {
	return "sharding"
}

func getReadWriteIopsForTable(schema string, table string, tables_iops_info QueryResult) int64 {
	for _, table_iops_info := range tables_iops_info {
		if table_iops_info["table_name"].(string) == table && table_iops_info["schema_name"].(string) == schema {
			a, _ := strconv.ParseInt(table_iops_info["seq_reads"].(string), 10, 64)
			b, _ := strconv.ParseInt(table_iops_info["row_writes"].(string), 10, 64)
			return a + b
		}
	}
	return 0
}

func getRowCountForTable(schema string, table string, tables_row_count_info QueryResult) int64 {
	for _, table_row_count_info := range tables_row_count_info {
		if table_row_count_info["table_name"].(string) == table && table_row_count_info["schema_name"].(string) == schema {
			a, _ := strconv.ParseInt(table_row_count_info["row_count"].(string), 10, 64)
			return a
		}
	}
	return 0
}

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

	// TODO: use Knowledge Base for fetching thresholds based on user input
	thresholds_for_colocation := sp.GetConfig()

	// TODO: if number of sql objects are less than the MINIMUM_TABLES_TO_COLOCATE then NO COLOCATION

	table_sizes_info := queryResults["table-sizes"]
	table_rows_info := queryResults["table-row-counts"]
	table_iops_info := queryResults["table-iops"]

	sortQueryResult(&table_sizes_info, "table_size")
	sortQueryResult(&table_rows_info, "row_count")

	// fmt.Printf("after sorting table_sizes_info: %v\n\n", table_sizes_info)
	// fmt.Printf("after sorting table_rows_info: %v\n", table_rows_info)
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

					read_write_iops := getReadWriteIopsForTable(schema, table, &table_iops_info)
					if read_write_iops <= thresholds_for_colocation["colocated_max_iops"].(int64) {
						total_num_colocated_tables++
						table_size, _ := strconv.ParseInt(table_size_info["table_size"].(string), 10, 64)
						total_colocated_tablet_size += table_size
						total_colocated_num_rows += getRowCountForTable(schema, table, &table_rows_info)

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
		// TODO: similar logic as above if table_sizes_info is empty
	} else {
		log.Errorf("no table size or row count info found")
		return nil, fmt.Errorf("no table size or row count info found")
	}

	return result, nil
}

func (sp *ShardingPlugin) GetHtmlTemplate() string {
	htmlString := `
    <!DOCTYPE html>
    <html>
    <head>
        <title>Sharding Assessment Report</title>
        <style>
            body {
                font-family: Arial, sans-serif;
                text-align: center;
            }

            .section {
                margin-bottom: 20px;
            }

            .section h2 {
                font-size: 1.2em;
                margin-bottom: 5px;
                text-align: left; /* Align headings to the left */
            }

            .table-list {
                list-style: none;
                padding: 0;
                text-align: right; /* Align table content to the right */
                margin: auto;
                max-width: 600px;		
            }

            .table-list li {
                margin-bottom: 5px;
                text-align: left; /* Align table content to the left */
                padding-left: 50px; /* Adjust left padding for schema and table */
            }
        </style>
    </head>
    <body>
        <h1 style="text-align: left;">Sharding Assessment Report</h1>
        <div class="section">
            <h2 style="text-align: left;">Colocated Tables</h2>
            <ul class="table-list">
                <li><b>Schema</b>&emsp;&emsp;<b>Table</b></li>
                {{range .colocated}}
                {{ $parts := split . "." }}
                <li>{{index $parts 0}}&emsp;&emsp;{{index $parts 1}}</li>
                {{end}}
            </ul>
        </div>
        <div class="section">
            <h2 style="text-align: left;">Sharded Tables</h2>
            <ul class="table-list">
                <li><b>Schema</b>&emsp;&emsp;<b>Table</b></li>
                {{range .sharded}}
                {{ $parts := split . "." }}
                <li>{{index $parts 0}}&emsp;&emsp;{{index $parts 1}}</li>
                {{end}}
            </ul>
        </div>
    </body>
    </html>
    `
	return htmlString
}

func (sp *ShardingPlugin) ModifySchema(report map[string]any) error {
	return nil
}

func (sp *ShardingPlugin) GetName() string {
	return "sharding"
}

func getReadWriteIopsForTable(schema string, table string, tables_iops_info *QueryResult) int64 {
	for _, table_iops_info := range *tables_iops_info {
		if table_iops_info["table_name"].(string) == table && table_iops_info["schema_name"].(string) == schema {
			a, _ := strconv.ParseInt(table_iops_info["seq_reads"].(string), 10, 64)
			b, _ := strconv.ParseInt(table_iops_info["row_writes"].(string), 10, 64)
			return a + b
		}
	}
	return 0
}

func getRowCountForTable(schema string, table string, tables_row_count_info *QueryResult) int64 {
	for _, table_row_count_info := range *tables_row_count_info {
		if table_row_count_info["table_name"].(string) == table && table_row_count_info["schema_name"].(string) == schema {
			a, _ := strconv.ParseInt(table_row_count_info["row_count"].(string), 10, 64)
			return a
		}
	}
	return 0
}

func sortQueryResult(queryResult *QueryResult, key string) {
	sort.Slice(*queryResult, func(i, j int) bool {
		a, err := strconv.ParseInt((*queryResult)[i][key].(string), 10, 64)
		if err != nil {
			log.Errorf("error parsing %s: %v", key, err)
			return false
		}
		b, err := strconv.ParseInt((*queryResult)[j][key].(string), 10, 64)
		if err != nil {
			log.Errorf("error parsing %s: %v", key, err)
			return false
		}
		return a > b
	})
}

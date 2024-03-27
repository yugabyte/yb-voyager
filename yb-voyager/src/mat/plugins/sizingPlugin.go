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
	log "github.com/sirupsen/logrus"
)

type SizingPlugin struct{}

func newSizingPlugin() *SizingPlugin {
	return &SizingPlugin{}
}

// TODO: sample code, needs to fetch this info from KnowledgeBase
func (sp *SizingPlugin) GetConfig() map[string]any {
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

func (sp *SizingPlugin) RunAssessment(queryResults map[string]QueryResult, userInput map[string]any) (Report, error) {
	result := map[string][]string{
		"configuration": {"4 cores * 6 nodes"},
		"tablets":       {"1 tablet per tables"},
	}

	log.Infof("Running sizing plugin")

	// TODO: use Knowledge Base for fetching thresholds based on user input
	return result, nil
}

func (sp *SizingPlugin) GetHtmlTemplate() string {
	htmlString := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sizing Assessment Report</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            text-align: center;
            margin: 0;
            padding: 0;
            background-color: #f4f4f4;
        }

        .container {
            max-width: 800px;
            margin: 20px auto;
            padding: 20px;
            background-color: #fff;
            border-radius: 5px;
            box-shadow: 0 0 10px rgba(0, 0, 0, 0.1);
        }

        h1 {
            text-align: center;
            color: #333;
            margin-bottom: 20px;
        }

        .section {
            margin-bottom: 30px;
            text-align: left;
        }

        .section h2 {
            font-size: 1.5em;
            margin-bottom: 10px;
            color: #007bff;
        }

        .table-list {
            list-style: none;
            padding: 0;
            margin: 0;
        }

        .table-list li {
            margin-bottom: 10px;
            display: flex;
            justify-content: space-between;
        }

        b {
            font-weight: bold;
        }

        .schema-column {
            flex: 1;
            margin-right: 10px;
        }

        .table-column {
            flex: 1;
            margin-left: 10px;
        }

        a {
            text-decoration: none;
            color: #007bff;
        }
    </style>
</head>
<body>
<div class="container">
    <h1>Sizing Assessment Report</h1>
    <div class="section">
        <h2>Colocated Tables</h2>
        <ul class="table-list">
            <li>
                <span class="schema-column"><b>Schema</b></span>
                <span class="table-column"><b>Table</b></span>
            </li>
            {{range .colocated}}
            {{ $parts := split . "." }}
            <li>
                <span class="schema-column">{{index $parts 0}}</span>
                <span class="table-column">{{index $parts 1}}</span>
            </li>
            {{end}}
        </ul>
    </div>
    <div class="section">
        <h2>Sharded Tables</h2>
        <ul class="table-list">
            <li>
                <span class="schema-column"><b>Schema</b></span>
                <span class="table-column"><b>Table</b></span>
            </li>
            {{range .sharded}}
            {{ $parts := split . "." }}
            <li>
                <span class="schema-column">{{index $parts 0}}</span>
                <span class="table-column">{{index $parts 1}}</span>
            </li>
            {{end}}
        </ul>
    </div>
</div>
</body>
</html>
`
	return htmlString
}

func (sp *SizingPlugin) ModifySchema(report Report) error {
	return nil
}

func (sp *SizingPlugin) GetName() string {
	return "sizing"
}

/*func getReadWriteIopsForTable(schema string, table string, tables_iops_info *QueryResult) int64 {
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

func getSizeForTable(schema string, table string, tables_size_info *QueryResult) int64 {
	for _, table_size_info := range *tables_size_info {
		if table_size_info["table_name"].(string) == table && table_size_info["schema_name"].(string) == schema {
			a, _ := strconv.ParseInt(table_size_info["table_size"].(string), 10, 64)
			return a
		}
	}
	return 0
}

func sortQueryResult(queryResult *QueryResult, key string) {
	log.Infof("sorting query result by key: %s", key)
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
}*/

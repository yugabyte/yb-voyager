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
package dbzm

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func IsDebeziumForDataExport(exportDir string) bool {
	exportStatusFilePath := filepath.Join(exportDir, "data", "export_status.json")
	return utils.FileOrFolderExists(exportStatusFilePath)
}

func StringSplitWithEscape(s string, separator string, escapeChar byte) ([]string, error) {
	var result []string
	stringSplits := strings.Split(s, separator)
	for i := 0; i < len(stringSplits); i++ {
		if stringSplits[i][len(stringSplits[i])-1] != escapeChar {
			result = append(result, stringSplits[i])
			continue
		}
		var j int
		for j = i + 1; j < len(stringSplits); j++ {
			if stringSplits[j][len(stringSplits[j])-1] != escapeChar {
				strCombined := strings.Join(stringSplits[i:j+1], separator)
				result = append(result, strCombined)
				break
			}
			if (j == (len(stringSplits) - 1)) && (stringSplits[j][len(stringSplits[j])-1] == escapeChar) {
				return nil, fmt.Errorf("Invalid string as last split ends with escape character. string=%s, escapeChar=%c", s, escapeChar)
			}
		}
		i = j
	}
	return result, nil
}

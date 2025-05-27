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

package queryissue

import (
	"fmt"

	"github.com/samber/lo"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
)

const (
	COLUMN_NAME                  = "Column Name"
	EXISTING_INDEX_SQL_STATEMENT = "Existing Index SQL Statement"
	CARDINALITY                  = "Cardinality"
	FREQUENCY_OF_NULLS           = "Frequency of Nulls"
	VALUE                        = "Value"
	FREQUENCY_OF_VALUE           = "Frequency of the value"
)

var hotspotsOnDateIndexes = issue.Issue{
	Type:        HOTSPOTS_ON_DATE_INDEX,
	Name:        HOTSPOTS_ON_DATE_INDEX_ISSUE,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: HOTSPOTS_ON_RANGE_SHARDED_INDEX_ISSUE_DESCRIPTION,
	GH:          "",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#hotspots-with-range-sharded-timestamp-date-indexes",
}

func NewHotspotOnDateIndexIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := hotspotsOnDateIndexes
	details := map[string]interface{}{
		COLUMN_NAME: colName,
	}

	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var hotspotsOnTimestampIndexes = issue.Issue{
	Type:        HOTSPOTS_ON_TIMESTAMP_INDEX,
	Name:        HOTSPOTS_ON_TIMESTAMP_INDEX_ISSUE,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: HOTSPOTS_ON_RANGE_SHARDED_INDEX_ISSUE_DESCRIPTION,
	GH:          "",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#hotspots-with-range-sharded-timestamp-date-indexes",
}

func NewHotspotOnTimestampIndexIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := hotspotsOnTimestampIndexes
	details := map[string]interface{}{
		COLUMN_NAME: colName,
	}

	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var redundantIndexesIssue = issue.Issue{
	Name:        REDUNDANT_INDEXES_ISSUE_NAME,
	Type:        REDUNDANT_INDEXES,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: REDUNDANT_INDEXES_DESCRIPTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#redundant-indexes",
}

func NewRedundantIndexIssue(objectType string, objectName string, sqlStatement string, existingDDL string) QueryIssue {
	issue := redundantIndexesIssue
	details := map[string]interface{}{
		EXISTING_INDEX_SQL_STATEMENT: existingDDL,
	}

	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var lowCardinalityIndexIssue = issue.Issue{
	Name:     LOW_CARDINALITY_INDEX_ISSUE_NAME,
	Type:     LOW_CARDINALITY_INDEXES,
	Impact:   constants.IMPACT_LEVEL_1,
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-low-cardinality-column",
}

func NewLowCardinalityIndexesIssue(objectType string, objectName string, sqlStatement string, isSingleColumnIndex bool, cardinality int64, columnName string) QueryIssue {
	issue := lowCardinalityIndexIssue
	if isSingleColumnIndex {
		issue.Description = fmt.Sprintf("%s %s", LOW_CARDINALITY_DESCRIPTION, LOW_CARDINALITY_DESCRIPTION_SINGLE_COLUMN)
	} else {
		issue.Description = fmt.Sprintf("%s %s", LOW_CARDINALITY_DESCRIPTION, LOW_CARDINALITY_DESCRIPTION_MULTI_COLUMN)
	}

	details := map[string]interface{}{
		COLUMN_NAME: columnName,
		CARDINALITY: cardinality,
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var nullValueIndexes = issue.Issue{
	Name:     NULL_VALUE_INDEXES_ISSUE_NAME,
	Type:     NULL_VALUE_INDEXES,
	Impact:   constants.IMPACT_LEVEL_1,
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-column-with-a-high-percentage-of-null-values",
}

func NewNullValueIndexesIssue(objectType string, objectName string, sqlStatement string, isSingleColumnIndex bool, nullFrequency int, columnName string) QueryIssue {
	issue := nullValueIndexes
	if isSingleColumnIndex {
		issue.Description = fmt.Sprintf("%s %s", NULL_VALUE_INDEXES_DESCRIPTION, NULL_VALUE_INDEXES_DESCRIPTION_SINGLE_COLUMN)
	} else {
		issue.Description = fmt.Sprintf("%s %s", NULL_VALUE_INDEXES_DESCRIPTION, NULL_VALUE_INDEXES_DESCRIPTION_MULTI_COLUMN)

	}
	details := map[string]interface{}{
		FREQUENCY_OF_NULLS: fmt.Sprintf("%d%%", nullFrequency),
		COLUMN_NAME:        columnName,
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var mostFrequentValueIndexIssue = issue.Issue{
	Name:     MOST_FREQUENT_VALUE_INDEXES_ISSUE_NAME,
	Type:     MOST_FREQUENT_VALUE_INDEXES,
	Impact:   constants.IMPACT_LEVEL_1,
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-column-with-high-percentage-of-a-particular-value",
}

func NewMostFrequentValueIndexesIssue(objectType string, objectName string, sqlStatement string, isSingleColumnIndex bool, value string, frequency int, columnName string) QueryIssue {
	issue := mostFrequentValueIndexIssue
	if isSingleColumnIndex {
		issue.Description = fmt.Sprintf("%s %s", MOST_FREQUENT_VALUE_INDEX_DESCRIPTION, MOST_FREQUENT_VALUE_INDEX_DESCRIPTION_SINGLE_COLUMN)
	} else {
		issue.Description = fmt.Sprintf("%s %s", MOST_FREQUENT_VALUE_INDEX_DESCRIPTION, MOST_FREQUENT_VALUE_INDEX_DESCRIPTION_MULTI_COLUMN)

	}
	details := map[string]interface{}{
		VALUE:              lo.Ternary(value != "", value, `' '`),
		FREQUENCY_OF_VALUE: fmt.Sprintf("%d%%", frequency),
		COLUMN_NAME:        columnName,
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

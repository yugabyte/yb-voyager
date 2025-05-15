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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
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
	if colName != "" {
		issue.Description = fmt.Sprintf("%s Affected column: %s", issue.Description, colName)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
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
	if colName != "" {
		issue.Description = fmt.Sprintf("%s Affected column: %s", issue.Description, colName)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
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
	if existingDDL != "" {
		issue.Description = fmt.Sprintf("%s\nExisting Index SQL Statement: %s", issue.Description, existingDDL)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
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
		issue.Description = fmt.Sprintf("%s %s\nCardinality of the column '%s' is %d.", LOW_CARDINALITY_DESCRIPTION, LOW_CARDINALITY_DESCRIPTION_SINGLE_COLUMN, columnName, cardinality)
	} else {
		issue.Description = fmt.Sprintf("%s %s\nCardinality of the column '%s' is %d.", LOW_CARDINALITY_DESCRIPTION, LOW_CARDINALITY_DESCRIPTION_MULTI_COLUMN, columnName, cardinality)
	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
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
		issue.Description = fmt.Sprintf("%s %s\nFrequency of NULLs on the column '%s' is %d%%.", NULL_VALUE_INDEXES_DESCRIPTION, NULL_VALUE_INDEXES_DESCRIPTION_SINGLE_COLUMN, columnName, nullFrequency)
	} else {
		issue.Description = fmt.Sprintf("%s %s\nFrequency of NULLs on the column '%s' is %d%%.", NULL_VALUE_INDEXES_DESCRIPTION, NULL_VALUE_INDEXES_DESCRIPTION_MULTI_COLUMN, columnName, nullFrequency)

	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
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
		issue.Description = fmt.Sprintf("%s %s\nFrequently occuring value '%s' with frequency %d%% on the column '%s'.", MOST_FREQUENT_VALUE_INDEX_DESCRIPTION, MOST_FREQUENT_VALUE_INDEX_DESCRIPTION_SINGLE_COLUMN, value, frequency, columnName)
	} else {
		issue.Description = fmt.Sprintf("%s %s\nFrequently occuring value '%s' with frequency %d%% on the column '%s'.", MOST_FREQUENT_VALUE_INDEX_DESCRIPTION, MOST_FREQUENT_VALUE_INDEX_DESCRIPTION_MULTI_COLUMN, value, frequency, columnName)

	}
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{})
}

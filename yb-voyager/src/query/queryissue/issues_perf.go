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
	//BEFORE ADDING ANY KEY FOR DETAILS MAP, ADD IT TO THE SensitiveKeysInIssueDetailsMap in query_issue.go
	COLUMN_NAME                  = "ColumnName"
	EXISTING_INDEX_SQL_STATEMENT = "ExistingIndexSQLStatement"
	CARDINALITY                  = "Cardinality"
	FREQUENCY_OF_NULLS           = "FrequencyOfNulls"
	VALUE                        = "Value"
	FREQUENCY_OF_VALUE           = "FrequencyOfTheValue"
	REFERENCED_COLUMN_NAME       = "ReferencedColumnName"
	COLUMN_TYPE                  = "ColumnType"
	REFERENCED_COLUMN_TYPE       = "ReferencedColumnType"
	FK_COLUMN_NAMES              = "ForeignKeyColumnNames"
	REFERENCED_TABLE_NAME        = "ReferencedTableName"
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

var hotspotsOnTimestampPrimaryOrUniqueKeyConstraint = issue.Issue{
	Type:        HOTSPOTS_ON_TIMESTAMP_PK_UK,
	Name:        HOTSPOTS_ON_TIMESTAMP_PK_UK_ISSUE,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: HOTSPOTS_ON_RANGE_SHARDED_PK_UK_ISSUE_DESCRIPTION,
	GH:          "",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#hotspots-with-range-sharded-timestamp-date-indexes",
}

func NewHotspotOnTimestampPKOrUKIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := hotspotsOnTimestampPrimaryOrUniqueKeyConstraint
	details := map[string]interface{}{
		COLUMN_NAME: colName,
	}

	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var hotspotsOnDatePrimaryOrUniqueKeyConstraint = issue.Issue{
	Type:        HOTSPOTS_ON_DATE_PK_UK,
	Name:        HOTSPOTS_ON_DATE_PK_UK_ISSUE,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: HOTSPOTS_ON_RANGE_SHARDED_PK_UK_ISSUE_DESCRIPTION,
	GH:          "",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#hotspots-with-range-sharded-timestamp-date-indexes",
}

func NewHotspotOnDatePKOrUKIssue(objectType string, objectName string, sqlStatement string, colName string) QueryIssue {
	issue := hotspotsOnDatePrimaryOrUniqueKeyConstraint
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

var foreignKeyDatatypeMismatchIssue = issue.Issue{
	Name:     FOREIGN_KEY_DATATYPE_MISMATCH_ISSUE_NAME,
	Type:     FOREIGN_KEY_DATATYPE_MISMATCH,
	Impact:   constants.IMPACT_LEVEL_1,
	DocsLink: "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#foreign-key-datatype-mismatch", // TODO add link to docs
}

func NewForeignKeyDatatypeMismatchIssue(objectType string, objectName string, sqlStatement string, fkColumnName string, refColumnName string, fkColumnType string, refColumnType string) QueryIssue {
	issue := foreignKeyDatatypeMismatchIssue

	issue.Description = fmt.Sprintf(FOREIGN_KEY_DATATYPE_MISMATCH_DESCRIPTION, fkColumnType, refColumnType)

	details := map[string]interface{}{
		COLUMN_NAME:            fkColumnName,
		REFERENCED_COLUMN_NAME: refColumnName,
		COLUMN_TYPE:            fkColumnType,
		REFERENCED_COLUMN_TYPE: refColumnType,
	}

	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

var missingForeignKeyIndexIssue = issue.Issue{
	Name:        MISSING_FOREIGN_KEY_INDEX_ISSUE_NAME,
	Type:        MISSING_FOREIGN_KEY_INDEX,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: MISSING_FOREIGN_KEY_INDEX_DESCRIPTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#missing-foreign-key-indexes", // TODO add link to docs
}

func NewMissingForeignKeyIndexIssue(objectType string, objectName string, sqlStatement string, fkColumns string, referencedTable string) QueryIssue {
	issue := missingForeignKeyIndexIssue

	details := map[string]interface{}{
		FK_COLUMN_NAMES:       fkColumns,
		REFERENCED_TABLE_NAME: referencedTable,
	}

	return newQueryIssue(issue, objectType, objectName, sqlStatement, details)
}

// NewMissingPrimaryKeyWhenUniqueNotNullIssue returns a recommendation to add a PK when a UNIQUE constraint's columns are all NOT NULL
var missingPrimaryKeyWhenUniqueNotNullIssue = issue.Issue{
	Name:        MISSING_PRIMARY_KEY_WHEN_UNIQUE_NOT_NULL_ISSUE_NAME,
	Type:        MISSING_PRIMARY_KEY_WHEN_UNIQUE_NOT_NULL,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: MISSING_PRIMARY_KEY_WHEN_UNIQUE_NOT_NULL_DESCRIPTION,
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#missing-primary-key-for-table-when-unique-and-not-null-columns-exist",
}

func NewMissingPrimaryKeyWhenUniqueNotNullIssue(objectType string, objectName string, options [][]string) QueryIssue {
	issue := missingPrimaryKeyWhenUniqueNotNullIssue
	details := map[string]interface{}{
		"PrimaryKeyColumnOptions": options,
	}
	return newQueryIssue(issue, objectType, objectName, "", details)
}

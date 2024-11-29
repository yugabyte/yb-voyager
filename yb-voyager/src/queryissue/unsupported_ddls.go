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
	"slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/queryparser"
)

// DDLIssueDetector interface defines methods for detecting issues in DDL objects
type DDLIssueDetector interface {
	DetectIssues(queryparser.DDLObject) ([]issue.IssueInstance, error)
}

// TableIssueDetector handles detection of table-related issues
type TableIssueDetector struct{}

func NewTableIssueDetector() *TableIssueDetector {
	return &TableIssueDetector{}
}

func (d *TableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
	table, ok := obj.(*queryparser.Table)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected Table")
	}

	var issues []issue.IssueInstance

	// Check for generated columns
	if len(table.GeneratedColumns) > 0 {
		issues = append(issues, issue.NewGeneratedColumnsIssue(
			issue.TABLE_OBJECT_TYPE,
			table.GetObjectName(),
			"", // query string
			table.GeneratedColumns,
		))
	}

	// Check for unlogged table
	if table.IsUnlogged {
		issues = append(issues, issue.NewUnloggedTableIssue(
			issue.TABLE_OBJECT_TYPE,
			table.GetObjectName(),
			"", // query string
		))
	}

	return issues, nil
}

// IndexIssueDetector handles detection of index-related issues
type IndexIssueDetector struct{}

func NewIndexIssueDetector() *IndexIssueDetector {
	return &IndexIssueDetector{}
}

func (d *IndexIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
	index, ok := obj.(*queryparser.Index)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected Index")
	}

	var issues []issue.IssueInstance

	// Check for unsupported index methods
	if slices.Contains(UnsupportedIndexMethods, index.AccessMethod) {
		issues = append(issues, issue.NewUnsupportedIndexMethodIssue(
			issue.INDEX_OBJECT_TYPE,
			index.GetObjectName(),
			"", // query string
			index.AccessMethod,
		))
	}

	// Check for storage parameters
	if index.NumStorageOptions > 0 {
		issues = append(issues, issue.NewStorageParameterIssue(
			issue.INDEX_OBJECT_TYPE,
			index.GetObjectName(),
			"", // query string
		))
	}

	//GinVariations
	if index.AccessMethod == GIN_ACCESS_METHOD {
		if len(index.Params) > 1 {
            /*
                e.g. CREATE INDEX idx_name ON public.test USING gin (data, data2);
                stmt:{index_stmt:{idxname:"idx_name" relation:{schemaname:"public" relname:"test" inh:true relpersistence:"p"
                location:125} access_method:"gin" index_params:{index_elem:{name:"data" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
                index_params:{index_elem:{name:"data2" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}}} stmt_location:81 stmt_len:81
            */
			issues = append(issues, issue.NewMultiColumnGinIndexIssue(
				issue.INDEX_OBJECT_TYPE,
				index.GetObjectName(),
				"",
			))
		} else {
            /*
                e.g. CREATE INDEX idx_name ON public.test USING gin (data DESC);
                stmt:{index_stmt:{idxname:"idx_name" relation:{schemaname:"public" relname:"test" inh:true relpersistence:"p" location:44}
                access_method:"gin" index_params:{index_elem:{name:"data" ordering:SORTBY_DESC nulls_ordering:SORTBY_NULLS_DEFAULT}}}} stmt_len:80
            */
			//In case only one Param is there
			param := index.Params[0]
			if param.SortByOrder != queryparser.DEFAULT_SORTING_ORDER {
				issues = append(issues, issue.NewOrderedGinIndexIssue(
					issue.INDEX_OBJECT_TYPE,
					index.GetObjectName(),
					"",
				))
			}
		}
	}


	return issues, nil
}

// AlterTableIssueDetector handles detection of alter table-related issues
type AlterTableIssueDetector struct{}

func NewAlterTableIssueDetector() *AlterTableIssueDetector {
	return &AlterTableIssueDetector{}
}

func (d *AlterTableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
	alter, ok := obj.(*queryparser.AlterTable)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected AlterTable")
	}

	var issues []issue.IssueInstance

	switch alter.AlterType {
	case queryparser.SET_OPTIONS:
		if alter.NumSetAttributes > 0 {
			issues = append(issues, issue.NewSetAttributeIssue(
				issue.TABLE_OBJECT_TYPE,
				alter.GetObjectName(),
				"", // query string
			))
		}
	case queryparser.ADD_CONSTRAINT:
		if alter.NumStorageOptions > 0 {
			issues = append(issues, issue.NewStorageParameterIssue(
				issue.TABLE_OBJECT_TYPE,
				alter.GetObjectName(),
				"", // query string
			))
		}
		if alter.ConstraintType == queryparser.EXCLUSION_CONSTR_TYPE {
			issues = append(issues, issue.NewExclusionConstraintIssue(
				issue.TABLE_OBJECT_TYPE,
				fmt.Sprintf("%s, constraint: (%s)", alter.GetObjectName(), alter.ConstraintName),
				"",
			))
		}
		if alter.ConstraintType != queryparser.FOREIGN_CONSTR_TYPE && alter.IsDeferrable {
			issues = append(issues, issue.NewDeferrableConstraintIssue(
				issue.TABLE_OBJECT_TYPE,
				fmt.Sprintf("%s, constraint: (%s)", alter.GetObjectName(), alter.ConstraintName),
				"",
			))
		}
	case queryparser.DISABLE_RULE:
		issues = append(issues, issue.NewDisableRuleIssue(
			issue.TABLE_OBJECT_TYPE,
			alter.GetObjectName(),
			"", // query string
			alter.RuleName,
		))
	case queryparser.CLUSTER_ON:
		issues = append(issues, issue.NewClusterONIssue(
			issue.TABLE_OBJECT_TYPE,
			alter.GetObjectName(),
			"", // query string
		))
	}

	return issues, nil
}

// Need to handle all the cases for which we don't have any issues detector
type NoOpIssueDetector struct{}

func NewNoOpIssueDetector() *NoOpIssueDetector {
	return &NoOpIssueDetector{}
}

func (n *NoOpIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
	return nil, nil
}

func GetDDLDetector(obj queryparser.DDLObject) (DDLIssueDetector, error) {
	switch obj.(type) {
	case *queryparser.Table:
		return NewTableIssueDetector(), nil
	case *queryparser.Index:
		return NewIndexIssueDetector(), nil
	case *queryparser.AlterTable:
		return NewAlterTableIssueDetector(), nil
	default:
		return NewNoOpIssueDetector(), nil
	}
}

const (
	GIN_ACCESS_METHOD = "gin"
)

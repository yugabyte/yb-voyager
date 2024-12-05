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

	"github.com/samber/lo"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// DDLIssueDetector interface defines methods for detecting issues in DDL objects
type DDLIssueDetector interface {
	DetectIssues(queryparser.DDLObject) ([]issue.IssueInstance, error)
}

//=============TABLE ISSUE DETECTOR ===========================

// TableIssueDetector handles detection of table-related issues
type TableIssueDetector struct {
	ParserIssueDetector
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

	if table.IsInherited {
		issues = append(issues, issue.NewInheritanceIssue(
			issue.TABLE_OBJECT_TYPE,
			table.GetObjectName(),
			"",
		))
	}

	if len(table.Constraints) > 0 {

		for _, c := range table.Constraints {
			if c.ConstraintType == queryparser.EXCLUSION_CONSTR_TYPE {
				issues = append(issues, issue.NewExclusionConstraintIssue(
					issue.TABLE_OBJECT_TYPE,
					fmt.Sprintf("%s, constraint: (%s)", table.GetObjectName(), c.ConstraintName),
					"",
				))
			}

			if c.ConstraintType != queryparser.FOREIGN_CONSTR_TYPE && c.IsDeferrable {
				issues = append(issues, issue.NewDeferrableConstraintIssue(
					issue.TABLE_OBJECT_TYPE,
					fmt.Sprintf("%s, constraint: (%s)", table.GetObjectName(), c.ConstraintName),
					"",
				))
			}

			if c.IsPrimaryKeyORUniqueConstraint() {
				for _, col := range c.Columns {
					unsupportedColumnsForTable, ok := d.columnsWithUnsupportedIndexDatatypes[table.GetObjectName()]
					if !ok {
						break
					}

					typeName, ok := unsupportedColumnsForTable[col]
					if !ok {
						continue
					}
					issues = append(issues, issue.NewPrimaryOrUniqueConsOnUnsupportedIndexTypesIssue(
						issue.TABLE_OBJECT_TYPE,
						fmt.Sprintf("%s, constraint: %s", table.GetObjectName(), c.ConstraintName),
						"",
						typeName,
						true,
					))
				}
			}
		}
	}
	for _, col := range table.Columns {
		liveUnsupportedDatatypes := srcdb.GetPGLiveMigrationUnsupportedDatatypes()
		liveWithFfOrFbUnsupportedDatatypes := srcdb.GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes()

		isUnsupportedDatatype := utils.ContainsAnyStringFromSlice(srcdb.PostgresUnsupportedDataTypes, col.TypeName)
		isUnsupportedDatatypeInLive := utils.ContainsAnyStringFromSlice(liveUnsupportedDatatypes, col.TypeName)

		isUnsupportedDatatypeInLiveWithFFOrFBList := utils.ContainsAnyStringFromSlice(liveWithFfOrFbUnsupportedDatatypes, col.TypeName)
		isUDTDatatype := utils.ContainsAnyStringFromSlice(d.compositeTypes, col.GetFullTypeName()) //if type is array
		isEnumDatatype := utils.ContainsAnyStringFromSlice(d.enumTypes, col.GetFullTypeName())     //is ENUM type
		isArrayOfEnumsDatatype := col.IsArrayType && isEnumDatatype
		isUnsupportedDatatypeInLiveWithFFOrFB := isUnsupportedDatatypeInLiveWithFFOrFBList || isUDTDatatype || isArrayOfEnumsDatatype

		if isUnsupportedDatatype {
			reportUnsupportedDatatypes(col, issue.TABLE_OBJECT_TYPE, table.GetObjectName(), &issues)
		} else if isUnsupportedDatatypeInLive {
			issues = append(issues, issue.NewUnsupportedDatatypesForLMIssue(
				issue.TABLE_OBJECT_TYPE,
				table.GetObjectName(),
				"",
				col.TypeName,
				col.ColumnName,
			))
		} else if isUnsupportedDatatypeInLiveWithFFOrFB {
			//reporting only for TABLE Type  as we don't deal with FOREIGN TABLE in live migration
			reportTypeName := col.GetFullTypeName()
			if col.IsArrayType { // For Array cases to make it clear in issue
				reportTypeName = fmt.Sprintf("%s[]", reportTypeName)
			}
			issues = append(issues, issue.NewUnsupportedDatatypesForLMWithFFOrFBIssue(
				issue.TABLE_OBJECT_TYPE,
				table.GetObjectName(),
				"",
				reportTypeName,
				col.ColumnName,
			))
		}
	}

	if table.IsPartitioned {

		/*
		   1. Adding PK to Partitioned  Table (in cases where ALTER is before create)
		   2. Expression partitions are not allowed if PK/UNIQUE columns are there is table
		   3. List partition strategy is not allowed with multi-column partitions.
		   4. Partition columns should all be included in Primary key set if any on table.
		*/
		alterAddPk := d.primaryConsInAlter[table.GetObjectName()]
		if alterAddPk != nil {
			issues = append(issues, issue.NewAlterTableAddPKOnPartiionIssue(
				issue.TABLE_OBJECT_TYPE,
				table.GetObjectName(),
				alterAddPk.Query,
			))
		}
		primaryKeyColumns := table.PrimaryKeyColumns()
		uniqueKeyColumns := table.UniqueKeyColumns()

		if table.IsExpressionPartition && (len(primaryKeyColumns) > 0 || len(uniqueKeyColumns) > 0) {
			issues = append(issues, issue.NewExpressionPartitionIssue(
				issue.TABLE_OBJECT_TYPE,
				table.GetObjectName(),
				"",
			))
		}

		if table.PartitionStrategy == queryparser.LIST_PARTITION &&
			len(table.PartitionColumns) > 1 {
			issues = append(issues, issue.NewMultiColumnListPartition(
				issue.TABLE_OBJECT_TYPE,
				table.GetObjectName(),
				"",
			))
		}
		partitionColumnsNotInPK, _ := lo.Difference(table.PartitionColumns, primaryKeyColumns)
		if len(primaryKeyColumns) > 0 && len(partitionColumnsNotInPK) > 0 {
			issues = append(issues, issue.NewInsufficientColumnInPKForPartition(
				issue.TABLE_OBJECT_TYPE,
				table.GetObjectName(),
				"",
				partitionColumnsNotInPK,
			))
		}
	}

	return issues, nil
}

func reportUnsupportedDatatypes(col queryparser.TableColumn, objType string, objName string, issues *[]issue.IssueInstance) {
	switch col.TypeName {
	case "xml":
		*issues = append(*issues, issue.NewXMLDatatypeIssue(
			objType,
			objName,
			"",
			col.ColumnName,
		))
	case "xid":
		*issues = append(*issues, issue.NewXIDDatatypeIssue(
			objType,
			objName,
			"",
			col.ColumnName,
		))
	case "geometry", "geography", "box2d", "box3d", "topogeometry":
		*issues = append(*issues, issue.NewPostGisDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	default:
		*issues = append(*issues, issue.NewUnsupportedDatatypesIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	}
}

//=============FOREIGN TABLE ISSUE DETECTOR ===========================

//ForeignTableIssueDetector handles detection Foreign table issues

type ForeignTableIssueDetector struct{}

func (f *ForeignTableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
	foreignTable, ok := obj.(*queryparser.ForeignTable)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected Foreign Table")
	}
	issues := make([]issue.IssueInstance, 0)

	issues = append(issues, issue.NewForeignTableIssue(
		issue.FOREIGN_TABLE_OBJECT_TYPE,
		foreignTable.GetObjectName(),
		"",
		foreignTable.ServerName,
	))

	for _, col := range foreignTable.Columns {
		isUnsupportedDatatype := utils.ContainsAnyStringFromSlice(srcdb.PostgresUnsupportedDataTypes, col.TypeName)
		if isUnsupportedDatatype {
			reportUnsupportedDatatypes(col, issue.FOREIGN_TABLE_OBJECT_TYPE, foreignTable.GetObjectName(), &issues)
		}
	}

	return issues, nil

}

//=============INDEX ISSUE DETECTOR ===========================

// IndexIssueDetector handles detection of index-related issues
type IndexIssueDetector struct {
	ParserIssueDetector
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
			issues = append(issues, issue.NewMultiColumnGinIndexIssue(
				issue.INDEX_OBJECT_TYPE,
				index.GetObjectName(),
				"",
			))
		} else {
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

	//Index on complex datatypes
	/*
	   cases covered
	       1. normal index on column with these types
	       2. expression index with  casting of unsupported column to supported types [No handling as such just to test as colName will not be there]
	       3. expression index with  casting to unsupported types
	       4. normal index on column with UDTs
	       5. these type of indexes on different access method like gin etc.. [TODO to explore more, for now not reporting the indexes on anyother access method than btree]
	*/
	_, ok = d.columnsWithUnsupportedIndexDatatypes[index.GetTableName()]
	if ok && index.AccessMethod == BTREE_ACCESS_METHOD { // Right now not reporting any other access method issues with such types.
		for _, param := range index.Params {
			if param.IsExpression {
				isUnsupportedType := slices.Contains(UnsupportedIndexDatatypes, param.ExprCastTypeName)
				isUDTType := slices.Contains(d.compositeTypes, param.GetFullExprCastTypeName())
				if param.IsExprCastArrayType {
					issues = append(issues, issue.NewIndexOnComplexDatatypesIssue(
						issue.INDEX_OBJECT_TYPE,
						index.GetObjectName(),
						"",
						"array",
					))
				} else if isUnsupportedType || isUDTType {
					reportTypeName := param.ExprCastTypeName
					if isUDTType {
						reportTypeName = "user_defined_type"
					}
					issues = append(issues, issue.NewIndexOnComplexDatatypesIssue(
						issue.INDEX_OBJECT_TYPE,
						index.GetObjectName(),
						"",
						reportTypeName,
					))
				}

			} else {
				colName := param.ColName
				typeName, ok := d.columnsWithUnsupportedIndexDatatypes[index.GetTableName()][colName]
				if !ok {
					continue
				}
				issues = append(issues, issue.NewIndexOnComplexDatatypesIssue(
					issue.INDEX_OBJECT_TYPE,
					index.GetObjectName(),
					"",
					typeName,
				))
			}
		}
	}

	return issues, nil
}

//=============ALTER TABLE ISSUE DETECTOR ===========================

// AlterTableIssueDetector handles detection of alter table-related issues
type AlterTableIssueDetector struct {
	ParserIssueDetector
}

func (aid *AlterTableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
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

		if alter.ConstraintType == queryparser.PRIMARY_CONSTR_TYPE &&
			aid.partitionTablesMap[alter.GetObjectName()] {
			issues = append(issues, issue.NewAlterTableAddPKOnPartiionIssue(
				issue.TABLE_OBJECT_TYPE,
				alter.GetObjectName(),
				"",
			))
		}

		if alter.AddPrimaryKeyOrUniqueCons() {
			for _, col := range alter.ConstraintColumns {
				unsupportedColumnsForTable, ok := aid.columnsWithUnsupportedIndexDatatypes[alter.GetObjectName()]
				if !ok {
					break
				}

				typeName, ok := unsupportedColumnsForTable[col]
				if !ok {
					continue
				}
				issues = append(issues, issue.NewPrimaryOrUniqueConsOnUnsupportedIndexTypesIssue(
					issue.TABLE_OBJECT_TYPE,
					fmt.Sprintf("%s, constraint: %s", alter.GetObjectName(), alter.ConstraintName),
					"",
					typeName,
					false,
				))
			}

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

//=============POLICY ISSUE DETECTOR ===========================

// PolicyIssueDetector handles detection of Create policy issues
type PolicyIssueDetector struct{}

func (p *PolicyIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
	policy, ok := obj.(*queryparser.Policy)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected Policy")
	}
	issues := make([]issue.IssueInstance, 0)
	if len(policy.RoleNames) > 0 {
		issues = append(issues, issue.NewPolicyRoleIssue(
			issue.POLICY_OBJECT_TYPE,
			policy.GetObjectName(),
			"",
			policy.RoleNames,
		))
	}
	return issues, nil
}

//=============TRIGGER ISSUE DETECTOR ===========================

// TriggerIssueDetector handles detection of Create Trigger issues
type TriggerIssueDetector struct {
	ParserIssueDetector
}

func (tid *TriggerIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
	trigger, ok := obj.(*queryparser.Trigger)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected Trigger")
	}
	issues := make([]issue.IssueInstance, 0)

	if trigger.IsConstraint {
		issues = append(issues, issue.NewConstraintTriggerIssue(
			issue.TRIGGER_OBJECT_TYPE,
			trigger.GetObjectName(),
			"",
		))
	}

	if trigger.NumTransitionRelations > 0 {
		issues = append(issues, issue.NewReferencingClauseTrigIssue(
			issue.TRIGGER_OBJECT_TYPE,
			trigger.GetObjectName(),
			"",
		))
	}

	if trigger.IsBeforeRowTrigger() && tid.partitionTablesMap[trigger.GetTableName()] {
		issues = append(issues, issue.NewBeforeRowOnPartitionTableIssue(
			issue.TRIGGER_OBJECT_TYPE,
			trigger.GetObjectName(),
			"",
		))
	}

	return issues, nil
}

//=============NO-OP ISSUE DETECTOR ===========================

// Need to handle all the cases for which we don't have any issues detector
type NoOpIssueDetector struct{}

func (n *NoOpIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]issue.IssueInstance, error) {
	return nil, nil
}

func (p *ParserIssueDetector) GetDDLDetector(obj queryparser.DDLObject) (DDLIssueDetector, error) {
	switch obj.(type) {
	case *queryparser.Table:
		return &TableIssueDetector{
			ParserIssueDetector: *p,
		}, nil
	case *queryparser.Index:
		return &IndexIssueDetector{
			ParserIssueDetector: *p,
		}, nil
	case *queryparser.AlterTable:
		return &AlterTableIssueDetector{
			ParserIssueDetector: *p,
		}, nil
	case *queryparser.Policy:
		return &PolicyIssueDetector{}, nil
	case *queryparser.Trigger:
		return &TriggerIssueDetector{
			ParserIssueDetector: *p,
		}, nil
	case *queryparser.ForeignTable:
		return &ForeignTableIssueDetector{}, nil
	default:
		return &NoOpIssueDetector{}, nil
	}
}

const (
	GIN_ACCESS_METHOD   = "gin"
	BTREE_ACCESS_METHOD = "btree"
)

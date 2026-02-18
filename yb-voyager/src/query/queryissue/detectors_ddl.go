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

	goerrors "github.com/go-errors/errors"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// DDLIssueDetector interface defines methods for detecting issues in DDL objects
type DDLIssueDetector interface {
	DetectIssues(queryparser.DDLObject) ([]QueryIssue, error)
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
	case *queryparser.View:
		return &ViewIssueDetector{}, nil
	case *queryparser.MView:
		return &MViewIssueDetector{}, nil
	case *queryparser.Function:
		return &FunctionIssueDetector{}, nil
	case *queryparser.Collation:
		return &CollationIssueDetector{}, nil
	case *queryparser.Extension:
		return &ExtensionIssueDetector{}, nil

	default:
		return &NoOpIssueDetector{}, nil
	}
}

//=============TABLE ISSUE DETECTOR ===========================

// TableIssueDetector handles detection of table-related issues
type TableIssueDetector struct {
	ParserIssueDetector
}

func (d *TableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	table, ok := obj.(*queryparser.Table)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected Table")
	}

	var issues []QueryIssue

	//For the cases where the table DDL is present in PLPGSQL and not present in actual schema so we need to report issues
	//the TableMetadata will only have the basic information and usage category will be set to unused
	tm := d.getOrCreateTableMetadata(table.GetObjectName())

	// Check for generated columns
	if len(table.GeneratedColumns) > 0 {
		issues = append(issues, NewGeneratedColumnsIssue(
			obj.GetObjectType(),
			table.GetObjectName(),
			"", // query string
			table.GeneratedColumns,
		))
	}

	// Check for unlogged table
	if table.IsUnlogged {
		issues = append(issues, NewUnloggedTableIssue(
			obj.GetObjectType(),
			table.GetObjectName(),
			"", // query string
		))
	}

	if table.IsInherited {
		issues = append(issues, NewInheritanceIssue(
			obj.GetObjectType(),
			table.GetObjectName(),
			"",
		))
	}

	if len(table.Constraints) > 0 {

		for _, c := range table.Constraints {
			if c.ConstraintType == queryparser.EXCLUSION_CONSTR_TYPE {
				issues = append(issues, NewExclusionConstraintIssue(
					obj.GetObjectType(),
					table.GetObjectName(),
					"",
					c.ConstraintName,
				))
			}
			if c.ConstraintType == queryparser.FOREIGN_CONSTR_TYPE {
				detectForeignKeyDatatypeMismatch(
					obj.GetObjectType(),
					table.GetObjectName(),
					c.Columns,
					d.tablesMetadata,
					&issues,
				)

			}

			if c.ConstraintType != queryparser.FOREIGN_CONSTR_TYPE && c.IsDeferrable {
				issues = append(issues, NewDeferrableConstraintIssue(
					obj.GetObjectType(),
					table.GetObjectName(),
					"",
					c.ConstraintName,
				))
			}

			if c.ConstraintType == queryparser.FOREIGN_CONSTR_TYPE && d.getPartitionedTablesMap()[c.ReferencedTable] {
				issues = append(issues, NewForeignKeyReferencesPartitionedTableIssue(
					TABLE_OBJECT_TYPE,
					table.GetObjectName(),
					"",
					c.ConstraintName,
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
					issues = append(issues,
						reportIndexOrConstraintIssuesOnComplexDatatypes(
							obj.GetObjectType(),
							table.GetObjectName(),
							typeName,
							true,
							c.ConstraintName,
						))
				}
				//Report PRIMARY KEY (createdat timestamp) as hotspot issue
				hotspotIssues, err := detectHotspotIssueOnConstraint(tm.IsPartitioned(), c.ConstraintType.String(), c.ConstraintName, c.Columns, d.columnsWithHotspotRangeIndexesDatatypes, obj, tm.Usage)
				if err != nil {
					return nil, err
				}
				issues = append(issues, hotspotIssues...)
			}

		}
	}
	for _, col := range table.Columns {
		liveUnsupportedDatatypes := srcdb.GetPGLiveMigrationUnsupportedDatatypes()
		liveWithFfOrFbUnsupportedDatatypes := srcdb.GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes()

		isUnsupportedDatatype := utils.ContainsAnyStringFromSlice(srcdb.PostgresUnsupportedDataTypes, col.TypeName)
		isUnsupportedDatatypeInLive := utils.ContainsAnyStringFromSlice(liveUnsupportedDatatypes, col.TypeName)

		isUnsupportedDatatypeInLiveWithFFOrFBList := utils.ContainsAnyStringFromSlice(liveWithFfOrFbUnsupportedDatatypes, col.TypeName)
		isUDTDatatype := utils.ContainsAnyStringFromSlice(d.compositeTypes, col.GetFullTypeName())
		isUnsupportedDatatypeInLiveWithFFOrFB := isUnsupportedDatatypeInLiveWithFFOrFBList || isUDTDatatype

		if isUnsupportedDatatype {
			issues = append(issues, ReportUnsupportedDatatypes(col.TypeName, col.ColumnName, obj.GetObjectType(), table.GetObjectName()))
		} else if isUnsupportedDatatypeInLive {
			issues = append(issues, ReportUnsupportedDatatypesInLive(col.TypeName, col.ColumnName, obj.GetObjectType(), table.GetObjectName()))
		} else if isUnsupportedDatatypeInLiveWithFFOrFB {
			//reporting only for TABLE Type  as we don't deal with FOREIGN TABLE in live migration
			reportTypeName := col.GetFullTypeName()
			if isUDTDatatype {
				issues = append(issues, NewUserDefinedDatatypeIssue(
					obj.GetObjectType(),
					table.GetObjectName(),
					"",
					reportTypeName,
					col.ColumnName,
				))
			} else {
				issues = append(issues, ReportUnsupportedDatatypesInLiveWithFFOrFB(col.TypeName, col.ColumnName, obj.GetObjectType(), table.GetObjectName()))
			}
		}

		if col.Compression != "" {
			issues = append(issues, NewCompressionClauseForToasting(
				obj.GetObjectType(),
				obj.GetObjectName(),
				"",
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
			issues = append(issues, NewAlterTableAddPKOnPartiionIssue(
				obj.GetObjectType(),
				table.GetObjectName(),
				alterAddPk.Query,
			))
		}
		primaryKeyColumns := table.PrimaryKeyColumns()
		uniqueKeyColumns := table.UniqueKeyColumns()

		if table.IsExpressionPartition && (len(primaryKeyColumns) > 0 || len(uniqueKeyColumns) > 0) {
			issues = append(issues, NewExpressionPartitionIssue(
				obj.GetObjectType(),
				table.GetObjectName(),
				"",
			))
		}

		if table.PartitionStrategy == queryparser.LIST_PARTITION &&
			len(table.PartitionColumns) > 1 {
			issues = append(issues, NewMultiColumnListPartition(
				obj.GetObjectType(),
				table.GetObjectName(),
				"",
			))
		}
		partitionColumnsNotInPK, _ := lo.Difference(table.PartitionColumns, primaryKeyColumns)
		if len(primaryKeyColumns) > 0 && len(partitionColumnsNotInPK) > 0 {
			issues = append(issues, NewInsufficientColumnInPKForPartition(
				obj.GetObjectType(),
				table.GetObjectName(),
				"",
				partitionColumnsNotInPK,
			))
		}
	}

	return issues, nil
}

func detectForeignKeyDatatypeMismatch(objectType string, objectName string, columnList []string, tablesMetadata map[string]*TableMetadata, issues *[]QueryIssue) {
	for _, col := range columnList {
		tm, ok := tablesMetadata[objectName]
		if !ok {
			continue
		}
		colMetadata, ok := tm.Columns[col]
		if !ok || !colMetadata.IsForeignKey {
			continue
		}
		if colMetadata.ReferencedColumnType == "" {
			log.Warnf("Referenced column type for %s.%s (referenced by foreign key on %s.%s) not found in column metadata",
				colMetadata.ReferencedTable, colMetadata.ReferencedColumn, objectName, col)
			continue
		}
		if colMetadata.DataType == "" {
			log.Warnf("Data type for column %s in table %s not found in column metadata", col, objectName)
			continue
		}

		// For detection logic: compare internal types and modifiers for accurate detection
		// For display: use SQL type names with modifiers for user-facing output

		// Compare only the internal types
		// Not comparing the modifiers here as in the testing, there were no issues with datatypes with mismatched modifiers
		if colMetadata.DataType == colMetadata.ReferencedColumnType {
			continue
		}

		// Generate user-facing display types for the issue
		localSQLTypeName := utils.GetSQLTypeName(colMetadata.DataType)
		localDatatypeWithModifiers := utils.ApplyModifiersToDatatype(localSQLTypeName, colMetadata.DataTypeMods)

		referencedSQLTypeName := utils.GetSQLTypeName(colMetadata.ReferencedColumnType)
		referencedDatatypeWithModifiers := utils.ApplyModifiersToDatatype(referencedSQLTypeName, colMetadata.ReferencedColumnTypeMods)

		*issues = append(*issues, NewForeignKeyDatatypeMismatchIssue(
			objectType,
			objectName,
			"", // query string
			objectName+"."+col,
			colMetadata.ReferencedTable+"."+colMetadata.ReferencedColumn,
			localDatatypeWithModifiers,
			referencedDatatypeWithModifiers,
			tm.Usage,
		))
	}
}

func detectHotspotIssueOnConstraint(isPartitionedTable bool, constraintType string, constraintName string, constraintColumns []string, columnsWithHotspotRangeIndexesDatatypes map[string]map[string]string, obj queryparser.DDLObject, usageCategory string) ([]QueryIssue, error) {
	//not reporting the hotspot issue for partitioned table since we are already reporting it on all the partitions
	if isPartitionedTable {
		return nil, nil
	}
	if len(constraintColumns) <= 0 {
		log.Warnf("empty columns list for %s constraint %s", constraintType, constraintName)
		return nil, nil
	}
	col := constraintColumns[0] // checking the first column only.
	columnWithHotspotTypes, tableHasHotspotTypes := columnsWithHotspotRangeIndexesDatatypes[obj.GetObjectName()]
	if !tableHasHotspotTypes {
		return nil, nil
	}
	//If first column is hotspot type then only report hotspot issue
	hotspotTypeName, isHotspotType := columnWithHotspotTypes[col]
	if !isHotspotType {
		return nil, nil
	}
	return reportHotspotsOnTimestampTypes(hotspotTypeName, obj.GetObjectType(), obj.GetObjectName(), col, false, usageCategory)
}

func ReportUnsupportedDatatypes(baseTypeName string, columnName string, objType string, objName string) QueryIssue {
	var issue QueryIssue
	switch baseTypeName {
	case "xml":
		issue = NewXMLDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "xid":
		issue = NewXIDDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "geometry":
		issue = NewGeometryDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "geography":
		issue = NewGeographyDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "box2d":
		issue = NewBox2DDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "box3d":
		issue = NewBox3DDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "topogeometry":
		issue = NewTopogeometryDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "lo":
		issue = NewLODatatypeIssue(
			objType,
			objName,
			"",
			"LARGE OBJECT",
			columnName,
		)
	case "int8multirange":
		issue = NewInt8MultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "int4multirange":
		issue = NewInt4MultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "datemultirange":
		issue = NewDateMultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "nummultirange":
		issue = NewNumMultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "tsmultirange":
		issue = NewTSMultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "tstzmultirange":
		issue = NewTSTZMultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "raster":
		issue = NewRasterDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "pg_lsn":
		issue = NewPgLsnDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "txid_snapshot":
		issue = NewTxidSnapshotDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	default:
		// Unrecognized types
		// Throwing error for now
		utils.ErrExit("Unrecognized unsupported data type %s", baseTypeName)
	}

	return issue
}

func ReportUnsupportedDatatypesInLive(baseTypeName string, columnName string, objType string, objName string) QueryIssue {
	var issue QueryIssue
	switch baseTypeName {
	case "point":
		issue = NewPointDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "line":
		issue = NewLineDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "lseg":
		issue = NewLsegDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "box":
		issue = NewBoxDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "path":
		issue = NewPathDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "polygon":
		issue = NewPolygonDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "circle":
		issue = NewCircleDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "vector":
		issue = NewVectorDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	case "timetz":
		issue = NewTimetzDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	default:
		// Unrecognized types
		// Throwing error for now
		utils.ErrExit("Unrecognized unsupported data type %s", baseTypeName)
	}

	return issue
}

func ReportUnsupportedDatatypesInLiveWithFFOrFB(baseTypeName string, columnName string, objType string, objName string) QueryIssue {
	var issue QueryIssue
	switch baseTypeName {
	case "tsquery":
		issue = NewTsQueryDatatypeIssue(
			objType,
			objName,
			"",
			baseTypeName,
			columnName,
		)
	default:
		// Unrecognized types
		// Throwing error for now
		utils.ErrExit("Unrecognized unsupported data type %s", baseTypeName)
	}
	return issue
}

//=============FOREIGN TABLE ISSUE DETECTOR ===========================

//ForeignTableIssueDetector handles detection Foreign table issues

type ForeignTableIssueDetector struct{}

func (f *ForeignTableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	foreignTable, ok := obj.(*queryparser.ForeignTable)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected Foreign Table")
	}
	issues := make([]QueryIssue, 0)

	issues = append(issues, NewForeignTableIssue(
		obj.GetObjectType(),
		foreignTable.GetObjectName(),
		"",
		foreignTable.ServerName,
	))

	for _, col := range foreignTable.Columns {
		isUnsupportedDatatype := utils.ContainsAnyStringFromSlice(srcdb.PostgresUnsupportedDataTypes, col.TypeName)
		if isUnsupportedDatatype {
			issues = append(issues, ReportUnsupportedDatatypes(col.TypeName, col.ColumnName, obj.GetObjectType(), foreignTable.GetObjectName()))
		}
	}

	return issues, nil

}

//=============INDEX ISSUE DETECTOR ===========================

// IndexIssueDetector handles detection of index-related issues
type IndexIssueDetector struct {
	ParserIssueDetector
}

func (d *IndexIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	index, ok := obj.(*queryparser.Index)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected Index")
	}

	var issues []QueryIssue

	// Check for unsupported index methods
	switch index.AccessMethod {

	case "gist":
		issues = append(issues, NewUnsupportedGistIndexMethodIssue(
			obj.GetObjectType(),
			index.GetObjectName(),
			"", // query string
		))

	case "brin":
		issues = append(issues, NewUnsupportedBrinIndexMethodIssue(
			obj.GetObjectType(),
			index.GetObjectName(),
			"", // query string
		))

	case "spgist":
		issues = append(issues, NewUnsupportedSpgistIndexMethodIssue(
			obj.GetObjectType(),
			index.GetObjectName(),
			"", // query string
		))

	}

	// Check for storage parameters
	if index.NumStorageOptions > 0 {
		issues = append(issues, NewStorageParameterIssue(
			obj.GetObjectType(),
			index.GetObjectName(),
			"", // query string
		))
	}

	//GinVariations
	if index.AccessMethod == queryparser.GIN_ACCESS_METHOD {
		if len(index.Params) > 1 {
			issues = append(issues, NewMultiColumnGinIndexIssue(
				obj.GetObjectType(),
				index.GetObjectName(),
				"",
			))
		} else {
			//In case only one Param is there
			param := index.Params[0]
			if param.SortByOrder != queryparser.DEFAULT_SORTING_ORDER {
				issues = append(issues, NewOrderedGinIndexIssue(
					obj.GetObjectType(),
					index.GetObjectName(),
					"",
				))
			}
		}
	}

	tm, ok := d.tablesMetadata[index.GetTableName()]
	if !ok {
		log.Warnf("table metadata not found for table: %s", index.GetTableName())
		//Just to handle any case where TABLE DDL present in PLPGSQL and not present in actual schema so we need to report issues
		tm = d.getOrCreateTableMetadata(index.GetTableName())
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
	usageCategory := d.getUsageCategoryForIndex(index.GetSchemaName(), index.TableName, index.IndexName)
	if index.AccessMethod == queryparser.BTREE_ACCESS_METHOD { // Right now not reporting any other access method issues with such types.
		for idx, param := range index.Params {
			if param.IsExpression {
				isUnsupportedType := slices.Contains(UnsupportedIndexDatatypes, param.ExprCastTypeName)
				isUDTType := slices.Contains(d.compositeTypes, param.GetFullExprCastTypeName())
				isHotspotType := slices.Contains(hotspotRangeIndexesTypes, param.ExprCastTypeName)
				if param.IsExprCastArrayType {
					issues = append(issues, NewIndexOnArrayDatatypeIssue(
						obj.GetObjectType(),
						index.GetObjectName(),
						"",
					))
				} else if isUDTType {
					issues = append(issues, NewIndexOnUserDefinedTypeIssue(
						obj.GetObjectType(),
						index.GetObjectName(),
						"",
					))
				} else if isUnsupportedType {
					issues = append(issues, reportIndexOrConstraintIssuesOnComplexDatatypes(
						obj.GetObjectType(),
						index.GetObjectName(),
						param.ExprCastTypeName,
						false,
						"",
					))
				} else if isHotspotType && idx == 0 && !tm.IsPartitioned() {
					//not reporting the hotspot issue for partitioned table since we are already reporting it on all the partitions
					//If first column is hotspot type then only report hotspot issue
					//For expression case not adding any colName for now in the issue
					hotspotIssues, err := reportHotspotsOnTimestampTypes(param.ExprCastTypeName, obj.GetObjectType(), obj.GetObjectName(), "", true, usageCategory)
					if err != nil {
						return nil, err
					}
					issues = append(issues, hotspotIssues...)
				}
			} else {
				colName := param.ColName
				columnWithUnsupportedTypes, tableHasUnsupportedTypes := d.columnsWithUnsupportedIndexDatatypes[index.GetTableName()]
				columnWithHotspotTypes, tableHasHotspotTypes := d.columnsWithHotspotRangeIndexesDatatypes[index.GetTableName()]
				if tableHasUnsupportedTypes {
					typeName, isUnsupportedType := columnWithUnsupportedTypes[colName]
					if isUnsupportedType {
						issues = append(issues, reportIndexOrConstraintIssuesOnComplexDatatypes(
							obj.GetObjectType(),
							index.GetObjectName(),
							typeName,
							false,
							"",
						))
					}
				}
				//TODO: separate out the Types check of Hotspot problem and the Range sharding recommendation
				if tableHasHotspotTypes && idx == 0 && !tm.IsPartitioned() {
					//not reporting the hotspot issue for partitioned table since we are already reporting it on all the partitions
					//If first column is hotspot type then only report hotspot issue
					hotspotTypeName, isHotspotType := columnWithHotspotTypes[colName]
					if isHotspotType {
						hotspotIssues, err := reportHotspotsOnTimestampTypes(hotspotTypeName, obj.GetObjectType(), obj.GetObjectName(), colName, true, usageCategory)
						if err != nil {
							return nil, err
						}
						issues = append(issues, hotspotIssues...)
					}
				}
			}
			if idx == 0 && !param.IsExpression {
				//if this is first column and not an expression index
				indexIssues, err := d.reportVariousIndexPerfOptimizationsOnFirstColumnOfIndex(index)
				if err != nil {
					return nil, err
				}
				issues = append(issues, indexIssues...)
			}
		}
	}
	return issues, nil
}

func (i *IndexIssueDetector) reportVariousIndexPerfOptimizationsOnFirstColumnOfIndex(index *queryparser.Index) ([]QueryIssue, error) {
	tm, ok := i.tablesMetadata[index.GetTableName()]
	if !ok {
		return nil, goerrors.Errorf("table metadata not found for table: %s", index.GetTableName())
	}
	if tm.IsPartitioned() {
		//not reporting the hotspot issue for partitioned table since we are already reporting it on all the partitions
		return nil, nil
	}
	var issues []QueryIssue

	firstColumnParam := index.Params[0]
	qualifiedFirstColumnName := fmt.Sprintf("%s.%s", index.GetTableName(), firstColumnParam.ColName)

	usageCategory := i.getUsageCategoryForIndex(index.GetSchemaName(), index.TableName, index.IndexName)

	isSingleColumnIndex := len(index.Params) == 1

	stat, ok := i.columnStatistics[qualifiedFirstColumnName]
	if !ok {
		return nil, nil
	}

	maxFrequencyPerc := int(stat.MostCommonFrequency * 100)
	nullFrequencyPerc := int(stat.NullFraction * 100)
	nullPartialIndex := false
	mostCommonValPartialIndex := false

	for _, clause := range index.WhereClausePredicates {
		if clause.ColName != firstColumnParam.ColName {
			continue
		}
		if clause.ColIsNotNULL {
			nullPartialIndex = true
		}

		if clause.Value == stat.MostCommonValue && clause.Operator == "<>" {
			mostCommonValPartialIndex = true
		}
	}
	//Now most frequent value is reported all the time as low cardinality is no longer a problem as we have default as RANGE secondary indexes
	if maxFrequencyPerc >= MOST_FREQUENT_VALUE_THRESHOLD && !mostCommonValPartialIndex {
		columnType := ""
		if columnMeta, ok := tm.Columns[firstColumnParam.ColName]; ok && columnMeta != nil {
			columnType = columnMeta.DataType
		}

		//If the index is not LOW cardinality one then see if that has most frequent value or not
		//MOST FREQUENT VALUE INDEX ISSUE
		issues = append(issues, NewMostFrequentValueIndexesIssue(INDEX_OBJECT_TYPE, index.GetObjectName(), "",
			isSingleColumnIndex, stat.MostCommonValue, maxFrequencyPerc, stat.ColumnName, columnType, usageCategory))

	}

	if nullFrequencyPerc >= NULL_FREQUENCY_THRESHOLD && !nullPartialIndex {

		// NULL VALUE INDEX ISSUE
		issues = append(issues, NewNullValueIndexesIssue(INDEX_OBJECT_TYPE, index.GetObjectName(), "", isSingleColumnIndex, nullFrequencyPerc, stat.ColumnName, usageCategory))
	}

	return issues, nil
}

func reportHotspotsOnTimestampTypes(typeName string, objType string, objName string, colName string, isSecondaryIndex bool, usageCategory string) ([]QueryIssue, error) {
	var issues []QueryIssue
	switch typeName {
	case TIMESTAMP, TIMESTAMPTZ:
		issue := lo.Ternary(isSecondaryIndex, NewHotspotOnTimestampIndexIssue(objType, objName, "", colName, usageCategory), NewHotspotOnTimestampPKOrUKIssue(objType, objName, "", colName, usageCategory))
		issues = append(issues, issue)
	case DATE:
		issue := lo.Ternary(isSecondaryIndex, NewHotspotOnDateIndexIssue(objType, objName, "", colName, usageCategory), NewHotspotOnDatePKOrUKIssue(objType, objName, "", colName, usageCategory))
		issues = append(issues, issue)
	default:
		return issues, goerrors.Errorf("unexpected type for the Hotspots on range indexes with timestamp/date types")
	}
	return issues, nil
}

func reportIndexOrConstraintIssuesOnComplexDatatypes(objType string, objName string, typeName string, isPkorUk bool, constraintName string) QueryIssue {
	var queryIssue QueryIssue
	switch typeName {
	case "citext":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnCitextDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnCitextDatatypeIssue(objType, objName, ""))
	case "tsvector":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnTsVectorDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnTsVectorDatatypeIssue(objType, objName, ""))
	case "tsquery":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnTsQueryDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnTsQueryDatatypeIssue(objType, objName, ""))
	case "jsonb":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnJsonbDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnJsonbDatatypeIssue(objType, objName, ""))
	case "inet":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnInetDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnInetDatatypeIssue(objType, objName, ""))
	case "json":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnJsonDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnJsonDatatypeIssue(objType, objName, ""))
	case "macaddr":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnMacaddrDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnMacaddrDatatypeIssue(objType, objName, ""))
	case "macaddr8":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnMacaddr8DatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnMacaddr8DatatypeIssue(objType, objName, ""))
	case "cidr":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnCidrDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnCidrDatatypeIssue(objType, objName, ""))
	case "bit":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnBitDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnBitDatatypeIssue(objType, objName, ""))
	case "varbit":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnVarbitDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnVarbitDatatypeIssue(objType, objName, ""))
	case "daterange":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnDaterangeDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnDaterangeDatatypeIssue(objType, objName, ""))
	case "tsrange":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnTsrangeDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnTsrangeDatatypeIssue(objType, objName, ""))
	case "tstzrange":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnTstzrangeDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnTstzrangeDatatypeIssue(objType, objName, ""))
	case "numrange":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnNumrangeDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnNumrangeDatatypeIssue(objType, objName, ""))
	case "int4range":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnInt4rangeDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnInt4rangeDatatypeIssue(objType, objName, ""))
	case "int8range":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnInt8rangeDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnInt8rangeDatatypeIssue(objType, objName, ""))
	case "interval":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnIntervalDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnIntervalDatatypeIssue(objType, objName, ""))
	case "circle":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnCircleDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnCircleDatatypeIssue(objType, objName, ""))
	case "box":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnBoxDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnBoxDatatypeIssue(objType, objName, ""))
	case "line":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnLineDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnLineDatatypeIssue(objType, objName, ""))
	case "lseg":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnLsegDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnLsegDatatypeIssue(objType, objName, ""))
	case "point":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnPointDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnPointDatatypeIssue(objType, objName, ""))
	case "pg_lsn":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnPgLsnDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnPgLsnDatatypeIssue(objType, objName, ""))
	case "path":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnPathDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnPathDatatypeIssue(objType, objName, ""))
	case "polygon":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnPolygonDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnPolygonDatatypeIssue(objType, objName, ""))
	case "txid_snapshot":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnTxidSnapshotDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnTxidSnapshotDatatypeIssue(objType, objName, ""))
	case "array":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnArrayDatatypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnArrayDatatypeIssue(objType, objName, ""))
	case "user_defined_type":
		queryIssue = lo.Ternary(isPkorUk,
			NewPrimaryOrUniqueConstraintOnUserDefinedTypeIssue(objType, objName, "", typeName, constraintName),
			NewIndexOnUserDefinedTypeIssue(objType, objName, ""))
	default:
		// Unrecognized types
		// Throwing error for now
		utils.ErrExit("Unrecognized unsupported index data type %s", typeName)
	}
	return queryIssue
}

//=============ALTER TABLE ISSUE DETECTOR ===========================

// AlterTableIssueDetector handles detection of alter table-related issues
type AlterTableIssueDetector struct {
	ParserIssueDetector
}

func (aid *AlterTableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	alter, ok := obj.(*queryparser.AlterTable)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected AlterTable")
	}
	tm := aid.getOrCreateTableMetadata(alter.GetObjectName())
	var issues []QueryIssue

	switch alter.AlterType {
	case queryparser.SET_OPTIONS:
		if alter.NumSetAttributes > 0 {
			issues = append(issues, NewSetColumnAttributeIssue(
				obj.GetObjectType(),
				alter.GetObjectName(),
				"", // query string
			))
		}
	case queryparser.ADD_CONSTRAINT:
		if alter.NumStorageOptions > 0 {
			issues = append(issues, NewStorageParameterIssue(
				obj.GetObjectType(),
				alter.GetObjectName(),
				"", // query string
			))
		}
		if alter.ConstraintType == queryparser.EXCLUSION_CONSTR_TYPE {
			issues = append(issues, NewExclusionConstraintIssue(
				obj.GetObjectType(),
				alter.GetObjectName(),
				"",
				alter.ConstraintName,
			))
		}
		if alter.ConstraintType != queryparser.FOREIGN_CONSTR_TYPE && alter.IsDeferrable {
			issues = append(issues, NewDeferrableConstraintIssue(
				obj.GetObjectType(),
				alter.GetObjectName(),
				"",
				alter.ConstraintName,
			))
		}

		if alter.ConstraintType == queryparser.FOREIGN_CONSTR_TYPE &&
			aid.getPartitionedTablesMap()[alter.ConstraintReferencedTable] {
			//FK constraint references partitioned table
			issues = append(issues, NewForeignKeyReferencesPartitionedTableIssue(
				TABLE_OBJECT_TYPE,
				alter.GetObjectName(),
				"",
				alter.ConstraintName,
			))
		}

		// Foreign key datatype mismatch
		if alter.ConstraintType == queryparser.FOREIGN_CONSTR_TYPE {
			detectForeignKeyDatatypeMismatch(
				obj.GetObjectType(),
				alter.GetObjectName(),
				alter.ConstraintColumns,
				aid.tablesMetadata,
				&issues,
			)

		}

		if alter.ConstraintType == queryparser.PRIMARY_CONSTR_TYPE &&
			aid.getPartitionedTablesMap()[alter.GetObjectName()] {
			issues = append(issues, NewAlterTableAddPKOnPartiionIssue(
				obj.GetObjectType(),
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
				issues = append(issues, reportIndexOrConstraintIssuesOnComplexDatatypes(
					obj.GetObjectType(),
					alter.GetObjectName(),
					typeName,
					true,
					alter.ConstraintName,
				))
			}
			//Report PRIMARY KEY (createdat timestamp) as hotspot issue
			hotspotIssues, err := detectHotspotIssueOnConstraint(tm.IsPartitioned(), alter.ConstraintType.String(), alter.ConstraintName, alter.ConstraintColumns, aid.columnsWithHotspotRangeIndexesDatatypes, obj, tm.Usage)
			if err != nil {
				return nil, err
			}
			issues = append(issues, hotspotIssues...)

		}

	case queryparser.DISABLE_RULE:
		issues = append(issues, NewAlterTableDisableRuleIssue(
			obj.GetObjectType(),
			alter.GetObjectName(),
			"", // query string
			alter.RuleName,
		))
	case queryparser.CLUSTER_ON:
		issues = append(issues, NewClusterONIssue(
			obj.GetObjectType(),
			alter.GetObjectName(),
			"", // query string
		))
	case queryparser.SET_COMPRESSION_ALTER_SUB_TYPE:
		/*
			ALTER TABLE ONLY public.tbl_comp1 ALTER COLUMN v SET COMPRESSION pglz;
			stmts:{stmt:{alter_table_stmt:{relation:{schemaname:"public" relname:"tbl_comp1" relpersistence:"p" location:19}
			cmds:{alter_table_cmd:{subtype:AT_SetCompression name:"v" def:{string:{sval:"pglz"}} behavior:DROP_RESTRICT}}
			objtype:OBJECT_TABLE}} stmt_len:71}
		*/
		issues = append(issues, NewCompressionClauseForToasting(
			obj.GetObjectType(),
			obj.GetObjectName(),
			"",
		))
	}

	return issues, nil
}

//=============POLICY ISSUE DETECTOR ===========================

// PolicyIssueDetector handles detection of Create policy issues
type PolicyIssueDetector struct{}

func (p *PolicyIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	policy, ok := obj.(*queryparser.Policy)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected Policy")
	}
	issues := make([]QueryIssue, 0)
	if len(policy.RoleNames) > 0 {
		issues = append(issues, NewPolicyRoleIssue(
			obj.GetObjectType(),
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

func (tid *TriggerIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	trigger, ok := obj.(*queryparser.Trigger)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected Trigger")
	}
	issues := make([]QueryIssue, 0)

	if trigger.IsConstraint {
		issues = append(issues, NewConstraintTriggerIssue(
			obj.GetObjectType(),
			trigger.GetObjectName(),
			"",
		))
	}

	if trigger.NumTransitionRelations > 0 {
		issues = append(issues, NewReferencingClauseTrigIssue(
			obj.GetObjectType(),
			trigger.GetObjectName(),
			"",
		))
	}

	if trigger.IsBeforeRowTrigger() && tid.getPartitionedTablesMap()[trigger.GetTableName()] {
		issues = append(issues, NewBeforeRowOnPartitionTableIssue(
			obj.GetObjectType(),
			trigger.GetObjectName(),
			"",
		))
	}

	if unsupportedLargeObjectFunctions.ContainsOne(trigger.FuncName) {
		//Can't detect trigger func name using the genericIssues's FuncCallDetector
		//as trigger execute Func name is not a FuncCall node, its []pg_query.Node
		issues = append(issues, NewLOFuntionsIssue(
			obj.GetObjectType(),
			trigger.GetObjectName(),
			"",
			[]string{trigger.FuncName},
		))
	}

	return issues, nil
}

// ==============VIEW ISSUE DETECTOR ======================

type ViewIssueDetector struct{}

func (v *ViewIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	view, ok := obj.(*queryparser.View)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected View")
	}
	var issues []QueryIssue

	if view.SecurityInvoker {
		issues = append(issues, NewSecurityInvokerViewIssue(obj.GetObjectType(), obj.GetObjectName(), ""))
	}
	return issues, nil
}

// ================FUNCTION ISSUE DETECTOR ==================

type FunctionIssueDetector struct{}

func (f *FunctionIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	function, ok := obj.(*queryparser.Function)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected Function")
	}
	var issues []QueryIssue

	if function.HasSqlBody {
		//https://www.postgresql.org/docs/15/sql-createfunction.html#:~:text=a%20new%20session.-,sql_body,-The%20body%20of
		issues = append(issues, NewSqlBodyInFunctionIssue(function.GetObjectType(), function.GetObjectName(), ""))
	}

	return issues, nil
}

// ==============MVIEW ISSUE DETECTOR ======================

type MViewIssueDetector struct{}

func (v *MViewIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	return nil, nil
}

//===============COLLATION ISSUE DETECTOR ====================

type CollationIssueDetector struct{}

func (c *CollationIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	collation, ok := obj.(*queryparser.Collation)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected Collation")
	}
	issues := make([]QueryIssue, 0)
	if val, ok := collation.Options["deterministic"]; ok {
		//we have two issues for this - one is fixed in 2.25 where deterministic option is recognized and
		//the other one is for non-determinisitic collaiton which is not fixed in 2.25
		switch val {
		case "false":
			issues = append(issues, NewNonDeterministicCollationIssue(
				collation.GetObjectType(),
				collation.GetObjectName(),
				"",
			))
		default:
			// deterministic attribute is itself not supported in YB either true or false so checking only whether option is present or not
			issues = append(issues, NewDeterministicOptionCollationIssue(
				collation.GetObjectType(),
				collation.GetObjectName(),
				"",
			))
		}
	}
	return issues, nil
}

//=============UNSUPPORTED EXTENSION ISSUE DETECTOR ===========================

// UnsupportedExtensionIssueDetector handles detection of unsupported extension issues
type ExtensionIssueDetector struct{}

func (e *ExtensionIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	extension, ok := obj.(*queryparser.Extension)
	if !ok {
		return nil, goerrors.Errorf("invalid object type: expected Extension")
	}

	issues := make([]QueryIssue, 0)
	if !slices.Contains(SupportedExtensionsOnYB, extension.GetObjectName()) {
		issues = append(issues, NewExtensionsIssue(
			obj.GetObjectType(),
			extension.GetObjectName(),
			"",
		))
	}

	return issues, nil
}

//=============NO-OP ISSUE DETECTOR ===========================

// Need to handle all the cases for which we don't have any issues detector
type NoOpIssueDetector struct{}

func (n *NoOpIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	return nil, nil
}

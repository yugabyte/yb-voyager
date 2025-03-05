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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// DDLIssueDetector interface defines methods for detecting issues in DDL objects
type DDLIssueDetector interface {
	DetectIssues(queryparser.DDLObject) ([]QueryIssue, error)
}

//=============TABLE ISSUE DETECTOR ===========================

// TableIssueDetector handles detection of table-related issues
type TableIssueDetector struct {
	ParserIssueDetector
}

func (d *TableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	table, ok := obj.(*queryparser.Table)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected Table")
	}

	var issues []QueryIssue

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

			if c.ConstraintType != queryparser.FOREIGN_CONSTR_TYPE && c.IsDeferrable {
				issues = append(issues, NewDeferrableConstraintIssue(
					obj.GetObjectType(),
					table.GetObjectName(),
					"",
					c.ConstraintName,
				))
			}

			if c.ConstraintType == queryparser.FOREIGN_CONSTR_TYPE && d.partitionTablesMap[c.ReferencedTable] {
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
					reportPKOrUniqueConstraintOnUnsupportedDatatypesIssue(
						obj.GetObjectType(),
						table.GetObjectName(),
						typeName,
						c.ConstraintName,
						&issues)
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
			reportUnsupportedDatatypes(col, obj.GetObjectType(), table.GetObjectName(), &issues)
		} else if isUnsupportedDatatypeInLive {
			reportUnsupoortedDatatypesInLive(col, obj.GetObjectType(), table.GetObjectName(), &issues)
		} else if isUnsupportedDatatypeInLiveWithFFOrFB {
			//reporting only for TABLE Type  as we don't deal with FOREIGN TABLE in live migration
			reportTypeName := col.GetFullTypeName()
			if isArrayOfEnumsDatatype {
				reportTypeName = fmt.Sprintf("%s[]", reportTypeName)
				issues = append(issues, NewArrayOfEnumDatatypeIssue(
					obj.GetObjectType(),
					table.GetObjectName(),
					"",
					reportTypeName,
					col.ColumnName,
				))
			} else if isUDTDatatype {
				issues = append(issues, NewUserDefinedDatatypeIssue(
					obj.GetObjectType(),
					table.GetObjectName(),
					"",
					reportTypeName,
					col.ColumnName,
				))
			} else {
				reportUnsupportedDatatypesInLiveWithFFOrFB(col, obj.GetObjectType(), table.GetObjectName(), &issues)
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

func reportPKOrUniqueConstraintOnUnsupportedDatatypesIssue(objType string, objName string, typeName string, constraintName string, issues *[]QueryIssue) {
	switch typeName {
	case "citext":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnCitextDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "tsvector":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnTsVectorDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "tsquery":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnTsQueryDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "jsonb":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnJsonbDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "inet":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnInetDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "json":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnJsonDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "macaddr":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnMacaddrDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "macaddr8":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnMacaddr8DatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "cidr":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnCidrDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "bit":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnBitDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "varbit":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnVarbitDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "daterange":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnDaterangeDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "tsrange":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnTsrangeDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "tstzrange":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnTstzrangeDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "numrange":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnNumrangeDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "int4range":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnInt4rangeDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "int8range":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnInt8rangeDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "interval":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnIntervalDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "circle":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnCircleDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "box":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnBoxDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "line":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnLineDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "lseg":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnLsegDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "point":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnPointDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "pg_lsn":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnPgLsnDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "path":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnPathDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "polygon":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnPolygonDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "txid_snapshot":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnTxidSnapshotDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "array":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnArrayDatatypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	case "user_defined_type":
		*issues = append(*issues, NewPrimaryOrUniqueConstraintOnUserDefinedTypeIssue(
			objType,
			objName,
			"",
			typeName,
			constraintName,
		))
	default:
		// Unrecognized types
		// Throwing error for now
		utils.ErrExit("Unrecognized unsupported data type %s", typeName)
	}
}

func reportUnsupportedDatatypes(col queryparser.TableColumn, objType string, objName string, issues *[]QueryIssue) {
	switch col.TypeName {
	case "xml":
		*issues = append(*issues, NewXMLDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "xid":
		*issues = append(*issues, NewXIDDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "geometry":
		*issues = append(*issues, NewGeometryDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "geography":
		*issues = append(*issues, NewGeographyDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "box2d":
		*issues = append(*issues, NewBox2DDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "box3d":
		*issues = append(*issues, NewBox3DDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "topogeometry":
		*issues = append(*issues, NewTopogeometryDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "lo":
		*issues = append(*issues, NewLODatatypeIssue(
			objType,
			objName,
			"",
			"LARGE OBJECT",
			col.ColumnName,
		))
	case "int8multirange":
		*issues = append(*issues, NewInt8MultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "int4multirange":
		*issues = append(*issues, NewInt4MultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "datemultirange":
		*issues = append(*issues, NewDateMultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "nummultirange":
		*issues = append(*issues, NewNumMultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "tsmultirange":
		*issues = append(*issues, NewTSMultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "tstzmultirange":
		*issues = append(*issues, NewTSTZMultiRangeDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "raster":
		*issues = append(*issues, NewRasterDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "pg_lsn":
		*issues = append(*issues, NewPgLsnDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "txid_snapshot":
		*issues = append(*issues, NewTxidSnapshotDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	default:
		// Unrecognized types
		// Throwing error for now
		utils.ErrExit("Unrecognized unsupported data type %s", col.TypeName)
	}
}

func reportUnsupoortedDatatypesInLive(col queryparser.TableColumn, objType string, objName string, issues *[]QueryIssue) {
	switch col.TypeName {
	case "point":
		*issues = append(*issues, NewPointDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "line":
		*issues = append(*issues, NewLineDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "lseg":
		*issues = append(*issues, NewLsegDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "box":
		*issues = append(*issues, NewBoxDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "path":
		*issues = append(*issues, NewPathDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "polygon":
		*issues = append(*issues, NewPolygonDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "circle":
		*issues = append(*issues, NewCircleDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	default:
		// Unrecognized types
		// Throwing error for now
		utils.ErrExit("Unrecognized unsupported data type %s", col.TypeName)
	}
}

func reportUnsupportedDatatypesInLiveWithFFOrFB(col queryparser.TableColumn, objType string, objName string, issues *[]QueryIssue) {
	switch col.TypeName {
	case "tsquery":
		*issues = append(*issues, NewTsQueryDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "tsvector":
		*issues = append(*issues, NewTsVectorDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	case "hstore":
		*issues = append(*issues, NewHstoreDatatypeIssue(
			objType,
			objName,
			"",
			col.TypeName,
			col.ColumnName,
		))
	default:
		// Unrecognized types
		// Throwing error for now
		utils.ErrExit("Unrecognized unsupported data type %s", col.TypeName)
	}
}

//=============FOREIGN TABLE ISSUE DETECTOR ===========================

//ForeignTableIssueDetector handles detection Foreign table issues

type ForeignTableIssueDetector struct{}

func (f *ForeignTableIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	foreignTable, ok := obj.(*queryparser.ForeignTable)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected Foreign Table")
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
			reportUnsupportedDatatypes(col, obj.GetObjectType(), foreignTable.GetObjectName(), &issues)
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
		return nil, fmt.Errorf("invalid object type: expected Index")
	}

	var issues []QueryIssue

	// Check for unsupported index methods
	if slices.Contains(UnsupportedIndexMethods, index.AccessMethod) {
		issues = append(issues, NewUnsupportedIndexMethodIssue(
			obj.GetObjectType(),
			index.GetObjectName(),
			"", // query string
			index.AccessMethod,
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
	if index.AccessMethod == GIN_ACCESS_METHOD {
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
					issues = append(issues, reportIndexOnComplexDatatypesIssue(
						obj.GetObjectType(),
						index.GetObjectName(),
						param.ExprCastTypeName,
					))
				}
			} else {
				colName := param.ColName
				typeName, ok := d.columnsWithUnsupportedIndexDatatypes[index.GetTableName()][colName]
				if !ok {
					continue
				}
				issues = append(issues, reportIndexOnComplexDatatypesIssue(
					obj.GetObjectType(),
					index.GetObjectName(),
					typeName,
				))
			}
		}
	}

	return issues, nil
}

func reportIndexOnComplexDatatypesIssue(objType string, objName string, typeName string) QueryIssue {
	var queryIssue QueryIssue
	switch typeName {
	case "citext":
		queryIssue = NewIndexOnCitextDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "tsvector":
		queryIssue = NewIndexOnTsVectorDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "tsquery":
		queryIssue = NewIndexOnTsQueryDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "jsonb":
		queryIssue = NewIndexOnJsonbDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "inet":
		queryIssue = NewIndexOnInetDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "json":
		queryIssue = NewIndexOnJsonDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "macaddr":
		queryIssue = NewIndexOnMacaddrDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "macaddr8":
		queryIssue = NewIndexOnMacaddr8DatatypeIssue(
			objType,
			objName,
			"",
		)
	case "cidr":
		queryIssue = NewIndexOnCidrDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "bit":
		queryIssue = NewIndexOnBitDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "varbit":
		queryIssue = NewIndexOnVarbitDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "daterange":
		queryIssue = NewIndexOnDaterangeDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "tsrange":
		queryIssue = NewIndexOnTsrangeDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "tstzrange":
		queryIssue = NewIndexOnTstzrangeDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "numrange":
		queryIssue = NewIndexOnNumrangeDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "int4range":
		queryIssue = NewIndexOnInt4rangeDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "int8range":
		queryIssue = NewIndexOnInt8rangeDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "interval":
		queryIssue = NewIndexOnIntervalDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "circle":
		queryIssue = NewIndexOnCircleDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "box":
		queryIssue = NewIndexOnBoxDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "line":
		queryIssue = NewIndexOnLineDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "lseg":
		queryIssue = NewIndexOnLsegDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "point":
		queryIssue = NewIndexOnPointDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "pg_lsn":
		queryIssue = NewIndexOnPgLsnDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "path":
		queryIssue = NewIndexOnPathDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "polygon":
		queryIssue = NewIndexOnPolygonDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "txid_snapshot":
		queryIssue = NewIndexOnTxidSnapshotDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "array":
		queryIssue = NewIndexOnArrayDatatypeIssue(
			objType,
			objName,
			"",
		)
	case "user_defined_type":
		queryIssue = NewIndexOnUserDefinedTypeIssue(
			objType,
			objName,
			"",
		)
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
		return nil, fmt.Errorf("invalid object type: expected AlterTable")
	}

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
			aid.partitionTablesMap[alter.ConstraintReferencedTable] {
			//FK constraint references partitioned table
			issues = append(issues, NewForeignKeyReferencesPartitionedTableIssue(
				TABLE_OBJECT_TYPE,
				alter.GetObjectName(),
				"",
				alter.ConstraintName,
			))
		}

		if alter.ConstraintType == queryparser.PRIMARY_CONSTR_TYPE &&
			aid.partitionTablesMap[alter.GetObjectName()] {
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
				reportPKOrUniqueConstraintOnUnsupportedDatatypesIssue(
					obj.GetObjectType(),
					alter.GetObjectName(),
					typeName,
					alter.ConstraintName,
					&issues)
			}

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
		return nil, fmt.Errorf("invalid object type: expected Policy")
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
		return nil, fmt.Errorf("invalid object type: expected Trigger")
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

	if trigger.IsBeforeRowTrigger() && tid.partitionTablesMap[trigger.GetTableName()] {
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
		return nil, fmt.Errorf("invalid object type: expected View")
	}
	var issues []QueryIssue

	if view.SecurityInvoker {
		issues = append(issues, NewSecurityInvokerViewIssue(obj.GetObjectType(), obj.GetObjectName(), ""))
	}
	return issues, nil
}

// ================FUNCTION ISSUE DETECTOR ==================

type FunctionIssueDetector struct{}

func (v *FunctionIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
	function, ok := obj.(*queryparser.Function)
	if !ok {
		return nil, fmt.Errorf("invalid object type: expected View")
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
		return nil, fmt.Errorf("invalid object type: expected Collation")
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

//=============NO-OP ISSUE DETECTOR ===========================

// Need to handle all the cases for which we don't have any issues detector
type NoOpIssueDetector struct{}

func (n *NoOpIssueDetector) DetectIssues(obj queryparser.DDLObject) ([]QueryIssue, error) {
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
	case *queryparser.View:
		return &ViewIssueDetector{}, nil
	case *queryparser.MView:
		return &MViewIssueDetector{}, nil
	case *queryparser.Function:
		return &FunctionIssueDetector{}, nil
	case *queryparser.Collation:
		return &CollationIssueDetector{}, nil
	default:
		return &NoOpIssueDetector{}, nil
	}
}

const (
	GIN_ACCESS_METHOD   = "gin"
	BTREE_ACCESS_METHOD = "btree"
)

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
package sqltransformer

import (
	"slices"

	goerrors "github.com/go-errors/errors"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

/*
	There can be various transformation possible on the exported schema

	- Merge Constraints into CREATE TABLE															[DONE]
	- Drop/Rename Objects																			[Future]
	- Drop/Rename columns: Voyager doesn't support export data for a set of datatypes,				[Future]
		transformation can be applied to remove those columns from CREATE
	- Datatype transformation: Voyager doesn't support export data for a set of datatypes,			[Future]
		transformation can be applied to change those datatypes to supported ones
	- Rename Table Partitions: In Oracle(ora2pg) case, we might need to rename table partitions		[Future]
		to match the naming convention of the target database or retain original names.

*/

// In future we can convert this to interface and have separate implementations for each transformation.
// But for now, kept it simple as a struct with methods
type Transformer struct {
}

func NewTransformer() *Transformer {
	return &Transformer{}
}

var constraintTypesToMerge = []pg_query.ConstrType{
	pg_query.ConstrType_CONSTR_PRIMARY, // PRIMARY KEY
	pg_query.ConstrType_CONSTR_CHECK,   // CHECK
}

/*
MergeConstraints scans through `stmts` and attempts to merge any
"ALTER TABLE ... ADD CONSTRAINT" into the corresponding "CREATE TABLE" node.

Error handling: If there is an error, it will be logged and the function will
return the original `stmts`.

Note: Need to keep the relative ordering of statements(tables) intact.
Because there can be cases like Foreign Key constraints that depend on the order of tables.
*/
func (t *Transformer) MergeConstraints(stmts []*pg_query.RawStmt) ([]*pg_query.RawStmt, error) {
	createStmtMap := make(map[string]*pg_query.RawStmt)
	for _, stmt := range stmts {
		stmtType := queryparser.GetStatementType(stmt.Stmt.ProtoReflect())
		if stmtType == queryparser.PG_QUERY_CREATE_STMT_NODE {
			objectName := queryparser.GetObjectNameFromRangeVar(stmt.Stmt.GetCreateStmt().Relation)
			createStmtMap[objectName] = stmt
		}
	}

	/*
		Logic to merge constraints into CREATE TABLE
		For each table, take CREATE TABLE stmt and merge its ALTER TABLE ADD CONSTRAINT stmts
		(constraint expected: PRIMARY KEY, UNIQUE, FOREIGN KEY, CHECK, DEFAULT)  (Q: what else constraint type can be there?)

		Assumptions
		1. There is CREATE TABLE stmt for each ALTER TABLE stmt
		2. There will be only one ALTER TABLE cmd per ALTER TABLE stmt

		Note: Iterating over the original array in such a way that we can keep the relative ordering of statements intact.
	*/
	var result []*pg_query.RawStmt
	for _, stmt := range stmts {
		stmtType := queryparser.GetStatementType(stmt.Stmt.ProtoReflect())
		switch stmtType {
		case queryparser.PG_QUERY_CREATE_STMT_NODE:
			result = append(result, stmt)
		case queryparser.PG_QUERY_ALTER_TABLE_STMT_NODE:
			objectName := queryparser.GetObjectNameFromRangeVar(stmt.Stmt.GetAlterTableStmt().Relation)
			alterTableNode := stmt.Stmt.GetAlterTableStmt()

			/*
				There can be multiple sub commands in an ALTER TABLE stmt
				Example: ALTER TABLE my_table
							ADD COLUMN new_col INTEGER,
							ADD CONSTRAINT my_pkey PRIMARY KEY (id);
			*/
			if len(alterTableNode.Cmds) == 0 {
				result = append(result, stmt)
			} else if len(alterTableNode.Cmds) > 1 {
				// Need special handling since there can be some/all cmds as ADD CONSTRAINT (TODO)
				// for now skipping any merge for this case (unlikely case from pg_dump/ora2pg)
				result = append(result, stmt)
			} else {
				alterTableCmd := alterTableNode.Cmds[0].GetAlterTableCmd()
				if alterTableCmd == nil {
					result = append(result, stmt)
					continue
				}

				/*
					Merge constraint if - PRIMARY KEY, UNIQUE constraint or CHECK constraint
					Otherwise, add it to the result slice
				*/
				alterTableCmdType := alterTableCmd.GetSubtype()
				log.Infof("alterTableCmdType: %v", *alterTableCmdType.Enum())
				if *alterTableCmdType.Enum() != pg_query.AlterTableType_AT_AddConstraint {
					// If the ALTER TABLE stmt is not an ADD CONSTRAINT stmt, then need to append it to the result slice
					result = append(result, stmt)
					continue
				}

				constrNode := alterTableCmd.GetDef().GetConstraint()
				if constrNode == nil {
					result = append(result, stmt)
					continue
				}

				constrType := constrNode.GetContype()
				// extra check whether Constraint VALID or NOT: if NOT, then we import it post snapshot
				if !slices.Contains(constraintTypesToMerge, constrType) || constrNode.SkipValidation {
					result = append(result, stmt)
					continue
				}

				// Merge these constraints into the CREATE TABLE stmt
				createStmt, ok := createStmtMap[objectName]
				if !ok {
					return nil, goerrors.Errorf("CREATE TABLE stmt not found for table %v", objectName)
				}
				log.Infof("merging constraint %v into CREATE TABLE for object %v", constrType, objectName)
				createStmt.Stmt.GetCreateStmt().TableElts = append(createStmt.Stmt.GetCreateStmt().TableElts, alterTableCmd.GetDef())
			}

		default:
			result = append(result, stmt)
		}
	}

	return result, nil
}

func (t *Transformer) RemoveRedundantIndexes(stmts []*pg_query.RawStmt, redundantIndexesMap *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, string]) ([]*pg_query.RawStmt, *utils.StructMap[*sqlname.ObjectNameQualifiedWithTableName, *pg_query.RawStmt], error) {
	log.Infof("removing redundant indexes from the schema")
	var sqlStmts []*pg_query.RawStmt
	removedIndexToStmtMap := utils.NewStructMap[*sqlname.ObjectNameQualifiedWithTableName, *pg_query.RawStmt]()
	for _, stmt := range stmts {
		stmtType := queryparser.GetStatementType(stmt.Stmt.ProtoReflect())
		if stmtType != queryparser.PG_QUERY_INDEX_STMT {
			sqlStmts = append(sqlStmts, stmt)
			continue
		}
		objectNameWithTable := queryparser.GetIndexObjectNameFromIndexStmt(stmt.Stmt.GetIndexStmt())
		if _, ok := redundantIndexesMap.Get(objectNameWithTable); ok {
			log.Infof("removing redundant index %s from the schema", objectNameWithTable.CatalogName())
			removedIndexToStmtMap.Put(objectNameWithTable, stmt)
		} else {
			sqlStmts = append(sqlStmts, stmt)
		}

	}

	return sqlStmts, removedIndexToStmtMap, nil
}

func (t *Transformer) ModifySecondaryIndexesToRange(stmts []*pg_query.RawStmt) ([]*pg_query.RawStmt, []*sqlname.ObjectNameQualifiedWithTableName, error) {
	var modifiedObjNames []*sqlname.ObjectNameQualifiedWithTableName
	for idx, stmt := range stmts {
		stmtType := queryparser.GetStatementType(stmt.Stmt.ProtoReflect())
		if stmtType != queryparser.PG_QUERY_INDEX_STMT {
			continue
		}
		indexStmt := stmt.Stmt.GetIndexStmt()
		if indexStmt == nil {
			continue
		}
		if indexStmt.AccessMethod != queryparser.BTREE_ACCESS_METHOD {
			//In Postgres the ordered scans are only supported for btree
			//so restricting the change to only Btree indexes
			//refer https://www.postgresql.org/docs/current/sql-createindex.html#:~:text=For%20index%20methods%20that%20support%20ordered%20scans%20(currently%2C%20only%20B%2Dtree)%2C%20the%20optional%20clauses%20ASC
			continue
		}
		if len(indexStmt.IndexParams) == 0 {
			//Just a sanity check to avoid any nil pointer dereference
			//In general, this should not happen
			continue
		}
		//checking only the first param of the key column of the index
		if indexStmt.IndexParams[0].GetIndexElem() == nil {
			continue
		}
		if indexStmt.IndexParams[0].GetIndexElem().Ordering != queryparser.DEFAULT_SORTING_ORDER {
			//If the index is already ordered, then we don't need to convert it to range index
			continue
		}
		//If the index is not ordered, then we need to convert it to range index
		//Add ASC clause to the index
		indexStmt.IndexParams[0].GetIndexElem().Ordering = queryparser.ASC_SORTING_ORDER
		stmts[idx] = stmt
		modifiedObjNames = append(modifiedObjNames, queryparser.GetIndexObjectNameFromIndexStmt(indexStmt))
	}
	return stmts, modifiedObjNames, nil
}

/*
Splitting all the statements in table.sql file into following categories:
1. Select and Set statements
3. Create and Alter Table statements with PRIMARY KEY constraints
4. Create and Alter Table statements with PRIMARY KEY constraints on TIMESTAMP/ DATE types
5. Alter Table statements with UNIQUE constraints
6. Other statements

and then adding the hash splitting ON for pk constraints and OFF for pk on timestamp/date types and  uk constraints

order of statements after transformation:
1. Select and Setstatements
2. SET HASH SPLITTING ON for pk constraints
3. Create and Alter Table statements with PRIMARY KEY constraints
4. SET HASH SPLITTING OFF for uk constraints
5. Create and Alter Table statements with PRIMARY KEY constraints on TIMESTAMP/ DATE types
6. Alter Table statements with UNIQUE constraints
6. Other statements
*/

func (t *Transformer) AddShardingStrategyForConstraints(stmts []*pg_query.RawStmt) ([]*pg_query.RawStmt, []string, []string, error) {
	log.Infof("adding hash splitting on for pk constraints to the schema")
	selectSetStatements := make([]*pg_query.RawStmt, 0)
	createAndAlterTableWithPK := make([]*pg_query.RawStmt, 0)
	createAndAlterTableWithPKOnTimestampOrDate := make([]*pg_query.RawStmt, 0)
	AlterTableUKConstraints := make([]*pg_query.RawStmt, 0)
	otherStatements := make([]*pg_query.RawStmt, 0)

	pkTablesOnTimestampOrDate := make([]string, 0)
	pkTablesWithHashSharding := make([]string, 0)

	tablesMap, err := getTablesMap(stmts)
	if err != nil {
		return nil, nil, nil, goerrors.Errorf("failed to get tables to column on range types: %v", err)
	}

	for _, stmt := range stmts {
		if queryparser.IsSelectStmt(stmt) || queryparser.IsSetStmt(stmt) {
			selectSetStatements = append(selectSetStatements, stmt)
			continue
		}
		ddlObject, err := queryparser.ProcessDDL(&pg_query.ParseResult{Stmts: []*pg_query.RawStmt{stmt}})
		if err != nil {
			return nil, nil, nil, goerrors.Errorf("failed to process ddl: %v", err)
		}
		switch ddlObject.(type) {
		case *queryparser.Table:
			table, _ := ddlObject.(*queryparser.Table)
			tableName := table.GetObjectName()
			pkConstraint := table.GetPKConstraint()
			if pkConstraint.ConstraintName == "" {
				//if the table doesn't have PK then no need to do further checks add it to create list and continue
				createAndAlterTableWithPK = append(createAndAlterTableWithPK, stmt)
				continue
			}
			isPKOnRangeDatatype, err := t.checkIfPrimaryKeyOnRangeDatatype(pkConstraint.Columns, table)
			if err != nil {
				return nil, nil, nil, goerrors.Errorf("failed to check if primary key on range datatype: %v", err)
			}
			if isPKOnRangeDatatype {
				pkTablesOnTimestampOrDate = append(pkTablesOnTimestampOrDate, tableName)
				createAndAlterTableWithPKOnTimestampOrDate = append(createAndAlterTableWithPKOnTimestampOrDate, stmt)
			} else {
				pkTablesWithHashSharding = append(pkTablesWithHashSharding, tableName)
				createAndAlterTableWithPK = append(createAndAlterTableWithPK, stmt)
			}
		case *queryparser.AlterTable:
			alterTable, _ := ddlObject.(*queryparser.AlterTable)
			switch alterTable.ConstraintType {
			case queryparser.PRIMARY_CONSTR_TYPE:
				table, ok := tablesMap[alterTable.GetObjectName()]
				if !ok {
					return nil, nil, nil, goerrors.Errorf("table %s not found in tables map", alterTable.GetObjectName())
				}
				isPKOnRangeDatatype, err := t.checkIfPrimaryKeyOnRangeDatatype(alterTable.ConstraintColumns, table)
				if err != nil {
					return nil, nil, nil, goerrors.Errorf("failed to check if primary key on range datatype: %v", err)
				}
				if isPKOnRangeDatatype {
					pkTablesOnTimestampOrDate = append(pkTablesOnTimestampOrDate, alterTable.GetObjectName())
					createAndAlterTableWithPKOnTimestampOrDate = append(createAndAlterTableWithPKOnTimestampOrDate, stmt)
				} else {
					pkTablesWithHashSharding = append(pkTablesWithHashSharding, alterTable.GetObjectName())
					createAndAlterTableWithPK = append(createAndAlterTableWithPK, stmt)
				}
			case queryparser.UNIQUE_CONSTR_TYPE:
				AlterTableUKConstraints = append(AlterTableUKConstraints, stmt)
			default:
				otherStatements = append(otherStatements, stmt)
			}
		default:
			otherStatements = append(otherStatements, stmt)
		}
	}

	hashSplittingSessionVariableOnParseTree, err := queryparser.Parse(HASH_SPLITTING_SESSION_VARIABLE_ON)
	if err != nil {
		return nil, nil, nil, goerrors.Errorf("failed to parse hash splitting session variable on: %v", err)
	}
	hashSplittingSessionVariableOffParseTree, err := queryparser.Parse(HASH_SPLITTING_SESSION_VARIABLE_OFF)
	if err != nil {
		return nil, nil, nil, goerrors.Errorf("failed to parse hash splitting session variable off: %v", err)
	}

	//TODO: see how we can add comments in between statements to make the table.sql more readable
	modifiedStmts := make([]*pg_query.RawStmt, 0)
	modifiedStmts = append(modifiedStmts, selectSetStatements...)
	modifiedStmts = append(modifiedStmts, hashSplittingSessionVariableOnParseTree.Stmts...)
	modifiedStmts = append(modifiedStmts, createAndAlterTableWithPK...)
	modifiedStmts = append(modifiedStmts, hashSplittingSessionVariableOffParseTree.Stmts...)
	modifiedStmts = append(modifiedStmts, createAndAlterTableWithPKOnTimestampOrDate...)
	modifiedStmts = append(modifiedStmts, AlterTableUKConstraints...)
	modifiedStmts = append(modifiedStmts, otherStatements...)

	return modifiedStmts, pkTablesOnTimestampOrDate, pkTablesWithHashSharding, nil

}

const (
	TIMESTAMPTZ = "timestamptz"
	TIMESTAMP   = "timestamp"
	DATE        = "date"
)

var RangeDatatypes = []string{
	TIMESTAMPTZ,
	TIMESTAMP,
	DATE,
}

func getTablesMap(stmts []*pg_query.RawStmt) (map[string]*queryparser.Table, error) {
	tablesMap := map[string]*queryparser.Table{}
	for _, stmt := range stmts {
		ddlObject, err := queryparser.ProcessDDL(&pg_query.ParseResult{Stmts: []*pg_query.RawStmt{stmt}})
		if err != nil {
			return nil, goerrors.Errorf("failed to process ddl: %v", err)
		}
		switch ddlObject.(type) {
		case *queryparser.Table:
			table, _ := ddlObject.(*queryparser.Table)
			tableName := table.GetObjectName()
			tablesMap[tableName] = table
		}
	}
	return tablesMap, nil
}

func (t *Transformer) checkIfPrimaryKeyOnRangeDatatype(pkConstraintCols []string, table *queryparser.Table) (bool, error) {
	tableName := table.GetObjectName()
	if len(pkConstraintCols) == 0 {
		return false, nil
	}
	firstCol := pkConstraintCols[0]
	firstColType := table.GetColumnType(firstCol)
	if !slices.Contains(RangeDatatypes, firstColType) {
		return false, nil
	}
	log.Infof("primary key first column - %s on table %s is on hotspot datatype, making it range sharded", firstCol, tableName)
	return true, nil
}

// Assuming first column in the index is the one to be filtered for null values
func (t *Transformer) AddPartialClauseForNullFiltering(parseTree *pg_query.ParseResult) (*pg_query.ParseResult, error) {
	indexNode, ok := queryparser.GetCreateIndexStmtNode(parseTree)
	if !ok {
		return nil, goerrors.Errorf("not a CREATE INDEX statement")
	}
	indexStmt := indexNode.IndexStmt

	// Get the first column name from index params
	if len(indexStmt.IndexParams) == 0 {
		return nil, goerrors.Errorf("index has no parameters")
	}
	colName := indexStmt.IndexParams[0].GetIndexElem().GetName()
	if colName == "" {
		return nil, goerrors.Errorf("first index parameter is an expression, not a column")
	}

	// Build IS NOT NULL node
	isNotNullNode := &pg_query.Node{
		Node: &pg_query.Node_NullTest{
			NullTest: &pg_query.NullTest{
				Arg: &pg_query.Node{
					Node: &pg_query.Node_ColumnRef{
						ColumnRef: &pg_query.ColumnRef{
							Fields: []*pg_query.Node{
								{Node: &pg_query.Node_String_{String_: &pg_query.String{Sval: colName}}},
							},
						},
					},
				},
				Nulltesttype: pg_query.NullTestType_IS_NOT_NULL,
			},
		},
	}

	// AND with existing WHERE clause, or set as the new WHERE clause
	if indexStmt.WhereClause != nil {
		indexStmt.WhereClause = &pg_query.Node{
			Node: &pg_query.Node_BoolExpr{
				BoolExpr: &pg_query.BoolExpr{
					Boolop: pg_query.BoolExprType_AND_EXPR,
					Args:   []*pg_query.Node{indexStmt.WhereClause, isNotNullNode},
				},
			},
		}
	} else {
		indexStmt.WhereClause = isNotNullNode
	}

	return parseTree, nil
}
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
	"fmt"
	"slices"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
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
	pg_query.ConstrType_CONSTR_UNIQUE,  // UNIQUE
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
	if len(stmts) == 0 {
		return stmts, nil
	}

	createStmtMap := make(map[string]*pg_query.RawStmt)
	for _, stmt := range stmts {
		stmtType := queryparser.GetStatementType(stmt.Stmt.ProtoReflect())
		if stmtType == queryparser.PG_QUERY_CREATE_STMT {
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
		case queryparser.PG_QUERY_CREATE_STMT:
			result = append(result, stmt)
		case queryparser.PG_QUERY_ALTER_TABLE_STMT:
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
					return nil, fmt.Errorf("CREATE TABLE stmt not found for table %v", objectName)
				}
				createStmt.Stmt.GetCreateStmt().TableElts = append(createStmt.Stmt.GetCreateStmt().TableElts, alterTableCmd.GetDef())
			}

		default:
			result = append(result, stmt)
		}
	}

	return result, nil
}

// write a tranformation function which converts the given tables into Sharded table by adding clause WITH (colocated = true)
func (t *Transformer) ConvertToShardedTables(stmts []*pg_query.RawStmt, isObjectSharded func(objectName string) bool) ([]*pg_query.RawStmt, error) {
	if len(stmts) == 0 {
		return stmts, nil
	}

	var result []*pg_query.RawStmt
	for _, stmt := range stmts {
		stmtType := queryparser.GetStatementType(stmt.Stmt.ProtoReflect())

		switch stmtType {
		case queryparser.PG_QUERY_CREATE_STMT: // CREATE TABLE case
			objectName := queryparser.GetObjectNameFromRangeVar(stmt.Stmt.GetCreateStmt().Relation)
			if isObjectSharded(objectName) {
				addColocationOptionToCreateTable(stmt.Stmt.GetCreateStmt())
			}

			result = append(result, stmt)
		case queryparser.PG_QUERY_CREATE_TABLE_AS_STMT: // CREATE MATERIALIZED VIEW case
			objectName := queryparser.GetObjectNameFromRangeVar(stmt.Stmt.GetCreateTableAsStmt().Into.Rel)
			if isObjectSharded(objectName) {
				addColocationOptionToCreateMaterializedView(stmt.Stmt.GetCreateTableAsStmt())
			}

			result = append(result, stmt)
		default:
			result = append(result, stmt)
		}

	}

	return result, nil
}

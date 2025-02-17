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
package transform

import (
	"fmt"

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

/*
MergeConstraints scans through `stmts` and attempts to merge any
"ALTER TABLE ... ADD CONSTRAINT" into the corresponding "CREATE TABLE" node.

Error handling: If there is an error, it will be logged and the function will
return the original `stmts`.

Note: Need to keep the relative ordering of statements(tables) intact.
Because there can be cases like Foreign Key constraints that depend on the order of tables.
*/
func (t *Transformer) MergeConstraints(stmts []*pg_query.RawStmt) ([]*pg_query.RawStmt, error) {
	var result []*pg_query.RawStmt
	// TODO: Ensure removing all the ALTER stmts which are merged into CREATE. No duplicates.

	createStmtMap := make(map[string]*pg_query.RawStmt)
	alterStmtsMap := make(map[string][]*pg_query.RawStmt)
	var leftOverStmts []*pg_query.RawStmt
	for _, stmt := range stmts {
		stmtType := queryparser.GetStatementType(stmt.Stmt.ProtoReflect())
		fmt.Printf("stmtType: %v\n", stmtType)
		switch stmtType {
		case queryparser.PG_QUERY_CREATE_STMT:
			objectName := queryparser.GetObjectNameFromRangeVar(stmt.Stmt.GetCreateStmt().Relation)
			createStmtMap[objectName] = stmt
		case queryparser.PG_QUERY_ALTER_TABLE_STMT:
			objectName := queryparser.GetObjectNameFromRangeVar(stmt.Stmt.GetAlterTableStmt().Relation)
			alterStmtsMap[objectName] = append(alterStmtsMap[objectName], stmt)
		default:
			leftOverStmts = append(leftOverStmts, stmt)
		}
	}

	// check for case: there is a ALTER TABLE stmt without a corresponding CREATE TABLE stmt - this should not happen
	for tableName := range alterStmtsMap {
		if _, ok := createStmtMap[tableName]; !ok {
			return nil, fmt.Errorf("ALTER TABLE stmt found for table %v without a corresponding CREATE TABLE stmt", tableName)
		}
	}

	/*
		Logic to merge constraints into CREATE TABLE
		For each table, take the CREATE TABLE stmt and merge all the ALTER TABLE ADD CONSTRAINT stmts into it
		(constraint should be one of the expected types: PRIMARY KEY, UNIQUE, FOREIGN KEY, CHECK, DEFAULT) (Q: what else constraint type can be there?)
	*/
	// Assuming there is no case where there is a ALTER TABLE ADD CONSTRAINT stmt without a corresponding CREATE TABLE stmt
	for tableName, createStmt := range createStmtMap {
		alterStmts := alterStmtsMap[tableName]
		for _, alterStmt := range alterStmts {
			alterTableCmd := alterStmt.Stmt.GetAlterTableStmt().Cmds[0].GetAlterTableCmd()
			fmt.Printf("Subtype of alter for table %v: %v\n\n", tableName, alterTableCmd.GetSubtype())
			alterTableCmdType := alterTableCmd.GetSubtype()
			// Only merge Primary Key and Unique constraints into CREATE TABLE
			if *alterTableCmdType.Enum() == pg_query.AlterTableType_AT_AddConstraint {
				// Merge the constraint into the CREATE TABLE stmt
				fmt.Println("Merging constraint into CREATE TABLE stmt")
				createStmt.Stmt.GetCreateStmt().TableElts = append(createStmt.Stmt.GetCreateStmt().TableElts, alterTableCmd.GetDef())
			} else {
				// If the ALTER TABLE stmt is not an ADD CONSTRAINT stmt, then need to add it to the result slice
				fmt.Println("Adding ALTER TABLE stmt to leftover slice")
				leftOverStmts = append(leftOverStmts, alterStmt)
			}
		}

		// Add the merged CREATE TABLE stmt to the result slice
		result = append(result, createStmt)
	}

	// Make sure if there were any leftover or extras in the passed stmts slice, if yes, then add those to the result slice
	result = append(result, leftOverStmts...)

	return result, nil
}

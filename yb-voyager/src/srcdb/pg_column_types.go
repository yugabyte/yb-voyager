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
package srcdb

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	pgTypTypeComposite  = "c"
	pgTypTypeRange      = "r"
	pgTypTypeMultirange = "m"
)

// columnType is one type a column's type is built from.
type columnType struct {
	// pg_type.typname, qualified with its schema for composite and range types since those
	// are user-defined and the same name can exist in several schemas.
	Name    string
	TypType string
}

type tableColumnTypes struct {
	Columns []string
	// UnderlyingTypes[i] holds every type Columns[i] is built from; [i][0] is its declared type.
	UnderlyingTypes [][]columnType
}

// columnUnderlyingTypesQuery lists, for every column of the tables in the IN list (format
// verb %s, as (schema, table) tuples), each type reachable from its declared type: a domain
// steps to its base type, an array to its element type, and a composite to each of its
// fields' types, recursively.
const columnUnderlyingTypesQuery = `
	WITH RECURSIVE column_type_tree AS (
		SELECT a.attrelid, a.attnum, a.atttypid AS type_oid, 0 AS depth, ARRAY[a.atttypid] AS visited
		FROM pg_attribute AS a
		JOIN pg_class AS c ON c.oid = a.attrelid
		JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE (n.nspname, c.relname) IN (%s)
			AND a.attnum > 0
			AND NOT a.attisdropped
		UNION ALL
		SELECT tree.attrelid, tree.attnum, child.type_oid, tree.depth + 1, tree.visited || child.type_oid
		FROM column_type_tree AS tree
		JOIN pg_type AS t ON t.oid = tree.type_oid
		JOIN LATERAL (
			SELECT t.typbasetype AS type_oid WHERE t.typtype = 'd'
			UNION ALL
			-- typtype = 'b' because a domain over an array copies the array's typcategory
			SELECT t.typelem WHERE t.typtype = 'b' AND t.typcategory = 'A' AND t.typelem <> 0
			UNION ALL
			SELECT field.atttypid
			FROM pg_attribute AS field
			WHERE t.typtype = 'c' AND field.attrelid = t.typrelid AND field.attnum > 0 AND NOT field.attisdropped
		) AS child ON true
		WHERE NOT child.type_oid = ANY(tree.visited)
	),
	-- a composite with several fields of one type reaches that type more than once
	column_type AS (
		SELECT attrelid, attnum, type_oid, MIN(depth) AS depth
		FROM column_type_tree
		GROUP BY attrelid, attnum, type_oid
	)
	SELECT
		n.nspname AS table_schema,
		c.relname AS table_name,
		a.attname AS column_name,
		CASE
			WHEN t.typtype IN ('c', 'r') THEN type_n.nspname || '.' || t.typname
			ELSE t.typname
		END AS type_name,
		t.typtype::text AS typtype
	FROM column_type AS ct
	JOIN pg_attribute AS a ON a.attrelid = ct.attrelid AND a.attnum = ct.attnum
	JOIN pg_class AS c ON c.oid = a.attrelid
	JOIN pg_namespace AS n ON n.oid = c.relnamespace
	JOIN pg_type AS t ON t.oid = ct.type_oid
	JOIN pg_namespace AS type_n ON type_n.oid = t.typnamespace
	-- attnum keeps columns in table definition order; depth puts each column's declared type first
	ORDER BY n.nspname, c.relname, a.attnum, ct.depth, type_name;`

// getTablesColumnTypes fetches the columns of every table in tableList, each with every type
// it is built from, in a single query.
func getTablesColumnTypes(tableList []sqlname.NameTuple, db *sql.DB) (map[sqlname.NameTuple]tableColumnTypes, error) {
	result := make(map[sqlname.NameTuple]tableColumnTypes)
	if len(tableList) == 0 {
		return result, nil
	}

	var tableTuples []string
	tableLookup := make(map[string]sqlname.NameTuple)
	for _, table := range tableList {
		schema, name := table.ForCatalogQuery()
		tableTuples = append(tableTuples, fmt.Sprintf("('%s', '%s')", schema, name))
		tableLookup[table.AsQualifiedCatalogName()] = table
	}
	query := fmt.Sprintf(columnUnderlyingTypesQuery, strings.Join(tableTuples, ", "))
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying column types: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("close rows for query %q: %v", query, closeErr)
		}
	}()

	for rows.Next() {
		var schema, table, column string
		var colType columnType
		if err := rows.Scan(&schema, &table, &column, &colType.Name, &colType.TypType); err != nil {
			return nil, fmt.Errorf("scanning column types: %w", err)
		}
		tableName, ok := tableLookup[fmt.Sprintf("%s.%s", schema, table)]
		if !ok {
			return nil, goerrors.Errorf("column types returned for table %s.%s which was not requested", schema, table)
		}

		info := result[tableName]
		lastIdx := len(info.Columns) - 1
		if lastIdx >= 0 && info.Columns[lastIdx] == column {
			info.UnderlyingTypes[lastIdx] = append(info.UnderlyingTypes[lastIdx], colType)
		} else {
			info.Columns = append(info.Columns, column)
			info.UnderlyingTypes = append(info.UnderlyingTypes, []columnType{colType})
		}
		result[tableName] = info
	}
	return result, rows.Err()
}

// isColumnTypeUnsupported reports whether any type the column is built from is unsupported,
// so a domain, array, or composite over an unsupported type is filtered like that type itself.
func isColumnTypeUnsupported(columnTypes []columnType, unsupportedTypeNames []string, unsupportedTypTypes []string) bool {
	for _, colType := range columnTypes {
		// catalog type names carry no modifiers (VARCHAR(255) is just varchar), so an exact
		// match against the list is enough
		if utils.ContainsAnyStringFromSlice(unsupportedTypeNames, colType.Name) || slices.Contains(unsupportedTypTypes, colType.TypType) {
			return true
		}
	}
	return false
}

// getColumnsWithSupportedTypes splits each table's columns into those whose types can be
// streamed and those that cannot. A table whose columns are all supported maps to ["*"].
func getColumnsWithSupportedTypes(db *sql.DB, tableList []sqlname.NameTuple, unsupportedTypeNames []string, unsupportedTypTypes []string) (*utils.StructMap[sqlname.NameTuple, []string], *utils.StructMap[sqlname.NameTuple, []string], error) {
	supportedTableColumnsMap := utils.NewStructMap[sqlname.NameTuple, []string]()
	unsupportedTableColumnsMap := utils.NewStructMap[sqlname.NameTuple, []string]()

	tablesColumnTypes, err := getTablesColumnTypes(tableList, db)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching table column types: %w", err)
	}
	for _, tableName := range tableList {
		info := tablesColumnTypes[tableName]
		var supportedColumnNames, unsupportedColumnNames []string
		for i, column := range info.Columns {
			if isColumnTypeUnsupported(info.UnderlyingTypes[i], unsupportedTypeNames, unsupportedTypTypes) {
				unsupportedColumnNames = append(unsupportedColumnNames, column)
			} else {
				supportedColumnNames = append(supportedColumnNames, column)
			}
		}
		if len(unsupportedColumnNames) == 0 {
			supportedTableColumnsMap.Put(tableName, []string{"*"})
			continue
		}
		supportedTableColumnsMap.Put(tableName, supportedColumnNames)
		unsupportedTableColumnsMap.Put(tableName, unsupportedColumnNames)
	}
	return supportedTableColumnsMap, unsupportedTableColumnsMap, nil
}

// typeOidsQuery considers only built-in types and types an extension owns, so a user-defined type
// that happens to share a name with one of them is ignored.
const typeOidsQuery = `
	SELECT t.typname, t.oid::bigint
	FROM pg_type AS t
	JOIN pg_namespace AS n ON n.oid = t.typnamespace
	WHERE t.typname = ANY($1)
		AND (
			n.nspname = 'pg_catalog'
			-- deptype 'e': t is a member of an extension
			OR EXISTS (
				SELECT 1 FROM pg_depend AS d
				WHERE d.classid = 'pg_type'::regclass AND d.objid = t.oid AND d.deptype = 'e'
			)
		)`

// getTypeToOid maps each of typeNames to its OID in the database. A type that does not exist, such
// as one from an extension that is not installed, is absent from the result.
func getTypeToOid(db *sql.DB, typeNames []string) (map[string]int, error) {
	rows, err := db.Query(typeOidsQuery, typeNames)
	if err != nil {
		return nil, goerrors.Errorf("querying type oids for %v: %w", typeNames, err)
	}
	defer rows.Close()

	typeToOid := make(map[string]int)
	for rows.Next() {
		var typeName string
		var typeOid int
		if err := rows.Scan(&typeName, &typeOid); err != nil {
			return nil, goerrors.Errorf("scanning type oid: %w", err)
		}
		if _, ok := typeToOid[typeName]; ok {
			return nil, goerrors.Errorf("more than one built-in or extension type is named %q", typeName)
		}
		typeToOid[typeName] = typeOid
	}
	if err := rows.Err(); err != nil {
		return nil, goerrors.Errorf("iterating type oids: %w", err)
	}
	return typeToOid, nil
}

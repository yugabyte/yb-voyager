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
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

// ColumnMetadata stores metadata about a column extracted during DDL parsing.
// It tracks characteristics like data type, index suitability, special types
// (e.g., JSONB, arrays), and foreign key relationships.
type ColumnMetadata struct {
	DataType               string
	DataTypeMods           []int32
	IsUnsupportedForIndex  bool
	IsHotspotForRangeIndex bool
	IsJsonb                bool
	IsArray                bool
	IsUserDefinedType      bool

	IsForeignKey             bool
	ReferencedTable          string
	ReferencedColumn         string
	ReferencedColumnType     string  // Stores the base type (e.g., "varchar", "numeric")
	ReferencedColumnTypeMods []int32 // Stores the type modifiers (e.g., [255] for varchar(255), [8,2] for numeric(8,2))
}

// foreignKeyConstraint represents a foreign key relationship defined in the schema.
// It captures the child table and columns, and the corresponding referenced parent table and columns.
// This structure is used to defer FK processing until all schema definitions are parsed.
type ForeignKeyConstraint struct {
	TableName         string
	ColumnNames       []string
	ReferencedTable   string
	ReferencedColumns []string
}

type ParserIssueDetector struct {
	// key is table name, value is map of column name to ColumnMetadata
	columnMetadata map[string]map[string]*ColumnMetadata

	// list of foreign key constraints in the exported schema
	foreignKeyConstraints []ForeignKeyConstraint

	/*
		this will contain the information in this format:
		public.table1 -> {
			column1: citext | jsonb | inet | tsquery | tsvector | array
			...
		}
		schema2.table2 -> {
			column3: citext | jsonb | inet | tsquery | tsvector | array
			...
		}
		Here only those columns on tables are stored which have unsupported type for Index in YB
	*/
	columnsWithUnsupportedIndexDatatypes map[string]map[string]string

	/*
		this will contain the information in this format:
		public.table1 -> {
			column1: timestamp | timestampz | date
			...
		}
		schema2.table2 -> {
			column3: timestamp | timestampz | date
			...
		}
		Here only those columns on tables are stored which have unsupported type for Index in YB
	*/
	columnsWithHotspotRangeIndexesDatatypes map[string]map[string]string

	// list of composite types with fully qualified typename in the exported schema
	compositeTypes []string

	// list of enum types with fully qualified typename in the exported schema
	enumTypes []string

	// key is partitioned table, value is true
	partitionedTablesMap map[string]bool

	// key is partitioned child table, value is its parent partitioned table
	partitionedFrom map[string]string

	// key is inherited child table, value is slice of parent table names (to handle multiple inheritance)
	inheritedFrom map[string][]string

	// key is partitioned table, value is sqlInfo (sqlstmt, fpath) where the ADD PRIMARY KEY statement resides
	primaryConsInAlter map[string]*queryparser.AlterTable

	isGinIndexPresentInSchema bool

	// Boolean to check if there are any unlogged tables that were filtered
	// out because they are fixed as per the target db version
	isUnloggedTablesIssueFiltered bool

	//Functions in exported schema
	functionObjects []*queryparser.Function

	//columns names with jsonb type
	jsonbColumns []string

	//column is the key (qualifiedTableName.column_name) -> column stats
	columnStatistics map[string]utils.ColumnStatistics

	// Indexes stored per table
	// Key is table name, value is slice of indexes on that table.
	tableIndexes map[string][]*queryparser.Index
}

func NewParserIssueDetector() *ParserIssueDetector {
	return &ParserIssueDetector{
		columnMetadata:                          make(map[string]map[string]*ColumnMetadata),
		compositeTypes:                          make([]string, 0),
		enumTypes:                               make([]string, 0),
		partitionedTablesMap:                    make(map[string]bool),
		primaryConsInAlter:                      make(map[string]*queryparser.AlterTable),
		columnStatistics:                        make(map[string]utils.ColumnStatistics),
		foreignKeyConstraints:                   make([]ForeignKeyConstraint, 0),
		inheritedFrom:                           make(map[string][]string),
		partitionedFrom:                         make(map[string]string),
		columnsWithUnsupportedIndexDatatypes:    make(map[string]map[string]string),
		columnsWithHotspotRangeIndexesDatatypes: make(map[string]map[string]string),
		jsonbColumns:                            make([]string, 0),
		tableIndexes:                            make(map[string][]*queryparser.Index),
	}
}

func (p *ParserIssueDetector) GetCompositeTypes() []string {
	return p.compositeTypes
}

func (p *ParserIssueDetector) GetEnumTypes() []string {
	return p.enumTypes
}

func (p *ParserIssueDetector) GetAllIssues(query string, targetDbVersion *ybversion.YBVersion) ([]QueryIssue, error) {
	issues, err := p.getAllIssues(query)
	if err != nil {
		return issues, err
	}

	return p.getIssuesNotFixedInTargetDbVersion(issues, targetDbVersion)
}

// This function is used to get the jsonb columns from the parser issue detector
// It is used in the JsonbSubscriptingDetector to check if the column is jsonb or not
func (p *ParserIssueDetector) GetJsonbColumns() []string {
	jsonbColumns := make([]string, 0)
	for _, columns := range p.columnMetadata {
		for columnName, meta := range columns {
			if meta.IsJsonb {
				jsonbColumns = append(jsonbColumns, columnName)
			}
		}
	}
	return jsonbColumns
}

// GetColumnsWithUnsupportedIndexDatatypes returns a map of table names to column names
// where the column uses a datatype not supported for indexing in YugabyteDB.
// Each entry also includes the column's data type for context.
func (p *ParserIssueDetector) GetColumnsWithUnsupportedIndexDatatypes() map[string]map[string]string {
	columnsWithUnsupportedIndexDatatypes := make(map[string]map[string]string)
	for tableName, columns := range p.columnMetadata {
		for columnName, meta := range columns {
			if meta.IsUnsupportedForIndex {
				if _, exists := columnsWithUnsupportedIndexDatatypes[tableName]; !exists {
					columnsWithUnsupportedIndexDatatypes[tableName] = make(map[string]string)
				}
				columnsWithUnsupportedIndexDatatypes[tableName][columnName] = meta.DataType
			}
		}
	}

	return columnsWithUnsupportedIndexDatatypes
}

// GetColumnsWithHotspotRangeIndexesDatatypes returns a map of table names to column names
// where the column's datatype is known to cause read/write hotspot issues when used in range indexes.
// Each entry also includes the column's data type for context.
func (p *ParserIssueDetector) GetColumnsWithHotspotRangeIndexesDatatypes() map[string]map[string]string {
	columnsWithHotspotRangeIndexesDatatypes := make(map[string]map[string]string)
	for tableName, columns := range p.columnMetadata {
		for columnName, meta := range columns {
			if meta.IsHotspotForRangeIndex {
				if _, exists := columnsWithHotspotRangeIndexesDatatypes[tableName]; !exists {
					columnsWithHotspotRangeIndexesDatatypes[tableName] = make(map[string]string)
				}
				columnsWithHotspotRangeIndexesDatatypes[tableName][columnName] = meta.DataType
			}
		}
	}
	return columnsWithHotspotRangeIndexesDatatypes
}

func (p *ParserIssueDetector) getAllIssues(query string) ([]QueryIssue, error) {
	plpgsqlIssues, err := p.getPLPGSQLIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting plpgsql issues: %v", err)
	}
	dmlIssues, err := p.getDMLIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting generic issues: %v", err)
	}
	ddlIssues, err := p.getDDLIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting ddl issues: %v", err)
	}
	return lo.Flatten([][]QueryIssue{plpgsqlIssues, dmlIssues, ddlIssues}), nil
}

func (p *ParserIssueDetector) getIssuesNotFixedInTargetDbVersion(issues []QueryIssue, targetDbVersion *ybversion.YBVersion) ([]QueryIssue, error) {
	var filteredIssues []QueryIssue
	for _, i := range issues {
		fixed, err := i.IsFixedIn(targetDbVersion)
		if err != nil {
			return nil, fmt.Errorf("checking if issue %v is supported: %w", i, err)
		}
		if !fixed {
			filteredIssues = append(filteredIssues, i)
		} else {
			if i.Issue.Type == UNLOGGED_TABLES {
				p.isUnloggedTablesIssueFiltered = true
			}
		}
	}
	return filteredIssues, nil
}

func (p *ParserIssueDetector) GetAllPLPGSQLIssues(query string, targetDbVersion *ybversion.YBVersion) ([]QueryIssue, error) {
	issues, err := p.getPLPGSQLIssues(query)
	if err != nil {
		return issues, nil
	}

	return p.getIssuesNotFixedInTargetDbVersion(issues, targetDbVersion)
}

func (p *ParserIssueDetector) getPLPGSQLIssues(query string) ([]QueryIssue, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}

	if !queryparser.IsPLPGSQLObject(parseTree) {
		return nil, nil
	}
	//TODO handle this in DDLPARSER, DDLIssueDetector
	objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)
	plpgsqlQueries, err := queryparser.GetAllPLPGSQLStatements(query)
	if err != nil {
		return nil, fmt.Errorf("error getting all the queries from query: %w", err)
	}
	var issues []QueryIssue
	errorneousQueriesStr := ""
	for _, plpgsqlQuery := range plpgsqlQueries {
		issuesInQuery, err := p.getAllIssues(plpgsqlQuery)
		if err != nil {
			//there can be plpgsql expr queries no parseable via parser e.g. "withdrawal > balance"
			logErr := strings.TrimPrefix(err.Error(), "error getting plpgsql issues: ") // Remove the context added via getAllIssues function when it failed first as it is unneccesary
			errorneousQueriesStr += fmt.Sprintf("PL/pgSQL query - %s and error - %s, ", plpgsqlQuery, logErr)
			continue
		}
		issues = append(issues, issuesInQuery...)
	}
	if errorneousQueriesStr != "" {
		log.Warnf("Found some errorneous PL/pgSQL queries in stmt [%s]: %s", query, errorneousQueriesStr)
	}
	percentTypeSyntaxIssues, err := p.GetPercentTypeSyntaxIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting reference TYPE syntax issues: %v", err)
	}
	issues = append(issues, percentTypeSyntaxIssues...)

	return lo.Map(issues, func(i QueryIssue, _ int) QueryIssue {
		//Replacing the objectType and objectName to the original ObjectType and ObjectName of the PLPGSQL object
		//e.g. replacing the DML_QUERY and "" to FUNCTION and <func_name>
		i.ObjectType = objType
		i.ObjectName = objName
		return i
	}), nil
}

// FinalizeColumnMetadata processes the column metadata after all DDL statements have been parsed.
func (p *ParserIssueDetector) FinalizeColumnMetadata() {
	// Finalize column metadata for inherited tables - copying columns from parent tables to child tables
	p.finalizeColumnsFromParentMap(p.inheritedFrom)

	// Finalize column metadata for partitioned tables - copying columns from partitioned parent tables to their child partitions
	// Convert partitionedFrom: map[string]string → map[string][]string first to pass it to finalizeColumnsFromParentMap
	partitionParentMap := make(map[string][]string)
	for child, parent := range p.partitionedFrom {
		partitionParentMap[child] = []string{parent}
	}
	p.finalizeColumnsFromParentMap(partitionParentMap)

	p.finalizeForeignKeyConstraints()

	p.populateColumnMetadataDerivedVars()

}

// populateColumnMetadataDerivedVars populates the variables in ParserIssueDetector struct that are derived from column metadata.
func (p *ParserIssueDetector) populateColumnMetadataDerivedVars() {
	p.columnsWithUnsupportedIndexDatatypes = p.GetColumnsWithUnsupportedIndexDatatypes()

	p.columnsWithHotspotRangeIndexesDatatypes = p.GetColumnsWithHotspotRangeIndexesDatatypes()

	p.jsonbColumns = p.GetJsonbColumns()
}

// finalizeColumnsFromParentMap copies column metadata from parent tables to their children,
// based on a given parent-child dependency map.
//
// The input is a map from child table name to a list of parent table names,
// and the function uses topological sorting to ensure all parent metadata is available
// before copying columns to their children.
//
// For each child, it creates a column metadata map if it doesn't exist,
// and copies any missing columns from each of its parents.
//
// This function is used to populate column metadata for both inherited and partitioned tables.
func (p *ParserIssueDetector) finalizeColumnsFromParentMap(parentMap map[string][]string) {
	orderedChildren := topoSort(parentMap)

	for _, child := range orderedChildren {
		parentList, exists := parentMap[child]
		if !exists || len(parentList) == 0 {
			continue // This is a root table with no parents
		}
		for _, parent := range parentList {
			parentCols, ok := p.columnMetadata[parent]
			if !ok {
				continue // Parent metadata not found
			}
			if _, exists := p.columnMetadata[child]; !exists {
				p.columnMetadata[child] = make(map[string]*ColumnMetadata)
			}
			for colName, colMeta := range parentCols {
				if _, exists := p.columnMetadata[child][colName]; !exists {
					copy := *colMeta
					p.columnMetadata[child][colName] = &copy
				}
			}
		}
	}
}

// topoSort returns a topological ordering of the keys in a dependency map,
// ensuring that all parent tables are visited before their children.
//
// The input is a map from child table name to a list of parent table names.
// This supports both partitioned tables (single parent) and inherited tables (multiple parents).
//
// It performs a depth-first traversal and returns a slice of table names
// such that for any entry [child -> parents], all parents appear before the child in the result.
//
// Assumes there are no cycles in the dependency map. This should not happen in either partitioned or inherited tables.
func topoSort(dependencyMap map[string][]string) []string {
	visited := make(map[string]bool)
	var result []string

	var dfs func(string)
	dfs = func(curr string) {
		if visited[curr] {
			return
		}
		for _, parent := range dependencyMap[curr] {
			dfs(parent)
		}
		visited[curr] = true
		result = append(result, curr)
	}

	for child := range dependencyMap {
		dfs(child)
	}
	return result
}

// finalizeForeignKeyConstraints updates columnMetadata with foreign key details.
// It iterates through all stored FK constraints and marks the corresponding local columns
// as foreign keys, populating their referenced table, column, and type information.
// This is done after all DDL statements have been processed to ensure complete metadata is
func (p *ParserIssueDetector) finalizeForeignKeyConstraints() {
	for _, fk := range p.foreignKeyConstraints {
		for i, localCol := range fk.ColumnNames {
			if _, ok := p.columnMetadata[fk.TableName]; !ok {
				p.columnMetadata[fk.TableName] = make(map[string]*ColumnMetadata)
			}
			meta, ok := p.columnMetadata[fk.TableName][localCol]
			if !ok {
				meta = &ColumnMetadata{}
				p.columnMetadata[fk.TableName][localCol] = meta
			}

			meta.IsForeignKey = true
			meta.ReferencedTable = fk.ReferencedTable
			if i < len(fk.ReferencedColumns) {
				refCol := fk.ReferencedColumns[i]
				meta.ReferencedColumn = refCol

				if refMeta, ok := p.columnMetadata[fk.ReferencedTable][refCol]; ok {
					// Store the referenced column type and modifiers separately for accurate comparison
					meta.ReferencedColumnType = refMeta.DataType
					meta.ReferencedColumnTypeMods = refMeta.DataTypeMods
				}
			} else {
				log.Warnf("Foreign key column count mismatch for table %s: localCols=%v, refCols=%v",
					fk.TableName, fk.ColumnNames, fk.ReferencedColumns)
			}
		}
	}
}

// this function is to parse the DDL and process it to extract the metadata about schema like isGinIndexPresentInSchema, partition tables, etc.
func (p *ParserIssueDetector) ParseAndProcessDDL(query string) error {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return fmt.Errorf("error parsing a query: %v", err)
	}
	ddlObj, err := queryparser.ProcessDDL(parseTree)
	if err != nil {
		return fmt.Errorf("error parsing DDL: %w", err)
	}

	switch ddlObj.(type) {
	case *queryparser.AlterTable:
		alter, _ := ddlObj.(*queryparser.AlterTable)

		if alter.ConstraintType == queryparser.PRIMARY_CONSTR_TYPE {
			//For the case ALTER and CREATE are not not is expected order where ALTER is before CREATE
			alter.Query = query
			p.primaryConsInAlter[alter.GetObjectName()] = alter

			// Process primary key as index for foreign key detection
			if len(alter.ConstraintColumns) > 0 {
				p.addConstraintAsIndex(alter.SchemaName, alter.TableName, alter.ConstraintColumns, alter.ConstraintName)
			}
		}

		if alter.ConstraintType == queryparser.UNIQUE_CONSTR_TYPE {
			// Process unique constraint as index for foreign key detection
			if len(alter.ConstraintColumns) > 0 {
				p.addConstraintAsIndex(alter.SchemaName, alter.TableName, alter.ConstraintColumns, alter.ConstraintName)
			}
		}

		if alter.ConstraintType == queryparser.FOREIGN_CONSTR_TYPE {
			// Collect the foreign key constraint details from ALTER TABLE statement.
			// These are stored temporarily and will be processed later in FinalizeColumnMetadata,
			// once all tables and columns are parsed and available in columnMetadata.
			p.foreignKeyConstraints = append(p.foreignKeyConstraints, ForeignKeyConstraint{
				TableName:         alter.GetObjectName(),
				ColumnNames:       alter.ConstraintColumns,
				ReferencedTable:   alter.ConstraintReferencedTable,
				ReferencedColumns: alter.ConstraintReferencedColumns,
			})
		}

		// If alter is to attach a partitioned table, track it
		if alter.AlterType == queryparser.ATTACH_PARTITION {
			// Ensure the partitioned table's parent is tracked in partitionedFrom
			if _, exists := p.partitionedFrom[alter.GetObjectName()]; !exists {
				p.partitionedFrom[alter.PartitionedChild] = alter.GetObjectName()
			}
		}

	case *queryparser.Table:
		table, _ := ddlObj.(*queryparser.Table)

		tableName := table.GetObjectName()

		if table.IsPartitioned {
			p.partitionedTablesMap[tableName] = true
		}

		// Track if table is a partition of another table
		if table.IsPartitionOf {
			p.partitionedFrom[tableName] = table.PartitionedFrom
		}

		// Track inheritance relationships
		if table.IsInherited {
			p.inheritedFrom[tableName] = append([]string{}, table.InheritedFrom...)
		}

		// Ensure map is initialized
		if _, exists := p.columnMetadata[tableName]; !exists {
			p.columnMetadata[tableName] = make(map[string]*ColumnMetadata)
		}

		for _, col := range table.Columns {
			// Check if metadata already exists (e.g., from deferred FK)
			meta, exists := p.columnMetadata[tableName][col.ColumnName]
			if !exists {
				meta = &ColumnMetadata{}
				p.columnMetadata[tableName][col.ColumnName] = meta
			}

			// Store the original type name from parse tree
			meta.DataType = col.TypeName
			meta.DataTypeMods = col.TypeMods

			isUnsupportedType := slices.Contains(UnsupportedIndexDatatypes, col.TypeName)
			isUDTType := slices.Contains(p.compositeTypes, col.GetFullTypeName())
			isHotspotType := slices.Contains(hotspotRangeIndexesTypes, col.TypeName)

			switch {
			case col.IsArrayType:
				meta.IsArray = true
				meta.IsUnsupportedForIndex = true
				meta.DataType = "array"

			case isUnsupportedType || isUDTType:
				meta.IsUnsupportedForIndex = true
				if isUDTType {
					meta.IsUserDefinedType = true
					meta.DataType = "user_defined_type"
				}

			case isHotspotType:
				meta.IsHotspotForRangeIndex = true
			}

			if col.TypeName == "jsonb" {
				meta.IsJsonb = true
			}
		}

		// Collect the foreign key constraint details from CREATE TABLE statement.
		// These are stored temporarily and will be processed later in FinalizeColumnMetadata,
		// once all tables and columns are parsed and available in columnMetadata.
		for _, constraint := range table.Constraints {
			if constraint.ConstraintType != queryparser.FOREIGN_CONSTR_TYPE {
				continue
			}

			// Populate the foreign key constraints
			p.foreignKeyConstraints = append(p.foreignKeyConstraints, ForeignKeyConstraint{
				TableName:         tableName,
				ColumnNames:       constraint.Columns,
				ReferencedTable:   constraint.ReferencedTable,
				ReferencedColumns: constraint.ReferencedColumns,
			})
		}

		// Process primary keys and unique constraints as indexes for foreign key detection
		p.processTableConstraintsAsIndexes(table)
	case *queryparser.CreateType:
		typeObj, _ := ddlObj.(*queryparser.CreateType)
		if typeObj.IsEnum {
			p.enumTypes = append(p.enumTypes, typeObj.GetObjectName())
		} else {
			p.compositeTypes = append(p.compositeTypes, typeObj.GetObjectName())
		}
	case *queryparser.Index:
		index, _ := ddlObj.(*queryparser.Index)
		if index.AccessMethod == GIN_ACCESS_METHOD {
			p.isGinIndexPresentInSchema = true
		}

		// Process index for foreign key detection
		p.addIndexToCoverage(index)
	case *queryparser.Function:
		fn, _ := ddlObj.(*queryparser.Function)
		p.functionObjects = append(p.functionObjects, fn)
	}
	return nil
}

func (p *ParserIssueDetector) GetDDLIssues(query string, targetDbVersion *ybversion.YBVersion) ([]QueryIssue, error) {
	issues, err := p.getDDLIssues(query)
	if err != nil {
		return issues, nil
	}

	return p.getIssuesNotFixedInTargetDbVersion(issues, targetDbVersion)

}

func (p *ParserIssueDetector) getDDLIssues(query string) ([]QueryIssue, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing a query: %v", err)
	}
	isDDL, err := queryparser.IsDDL(parseTree)
	if err != nil {
		return nil, fmt.Errorf("error checking if query is ddl: %w", err)
	}
	if !isDDL {
		return nil, nil
	}
	// Parse the query into a DDL object
	ddlObj, err := queryparser.ProcessDDL(parseTree)
	if err != nil {
		return nil, fmt.Errorf("error parsing DDL: %w", err)
	}
	// Get the appropriate issue detector
	detector, err := p.GetDDLDetector(ddlObj)
	if err != nil {
		return nil, fmt.Errorf("error getting issue detector: %w", err)
	}

	// Detect issues
	issues, err := detector.DetectIssues(ddlObj)
	if err != nil {
		return nil, fmt.Errorf("error detecting issues: %w", err)
	}

	// Add the original query to each issue
	for i := range issues {
		if issues[i].SqlStatement == "" {
			issues[i].SqlStatement = query
		}
	}

	/*
		For detecting these generic issues (Advisory locks, XML functions and System columns as of now) on DDL example -
		CREATE INDEX idx_invoices on invoices (xpath('/invoice/customer/text()', data));
		We need to call it on DDLs as well
	*/
	genericIssues, err := p.genericIssues(query)
	if err != nil {
		return nil, fmt.Errorf("error getting generic issues: %w", err)
	}

	for _, i := range genericIssues {
		//In case of genericIssues we don't populate the proper obj type and obj name
		i.ObjectType = ddlObj.GetObjectType()
		i.ObjectName = ddlObj.GetObjectName()
		issues = append(issues, i)
	}
	return issues, nil
}

func (p *ParserIssueDetector) GetPercentTypeSyntaxIssues(query string) ([]QueryIssue, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing the query-%s: %v", query, err)
	}

	objType, objName := queryparser.GetObjectTypeAndObjectName(parseTree)
	typeNames, err := queryparser.GetAllTypeNamesInPlpgSQLStmt(query)
	if err != nil {
		return nil, fmt.Errorf("error getting type names in PLPGSQL: %v", err)
	}

	/*
		Caveats of GetAllTypeNamesInPlpgSQLStmt():
			1. Not returning typename for variables in function parameter from this function (in correct in json as UNKNOWN), for that using the GetTypeNamesFromFuncParameters()
			2. Not returning the return type from this function (not available in json), for that using the GetReturnTypeOfFunc()
	*/
	if queryparser.IsFunctionObject(parseTree) {
		typeNames = append(typeNames, queryparser.GetReturnTypeOfFunc(parseTree))
	}
	typeNames = append(typeNames, queryparser.GetFuncParametersTypeNames(parseTree)...)
	var issues []QueryIssue
	for _, typeName := range typeNames {
		if strings.HasSuffix(typeName, "%TYPE") {
			issues = append(issues, NewPercentTypeSyntaxIssue(objType, objName, typeName)) // TODO: confirm
		}
	}
	return issues, nil
}

func (p *ParserIssueDetector) GetDMLIssues(query string, targetDbVersion *ybversion.YBVersion) ([]QueryIssue, error) {
	issues, err := p.getDMLIssues(query)
	if err != nil {
		return issues, err
	}

	return p.getIssuesNotFixedInTargetDbVersion(issues, targetDbVersion)
}

func (p *ParserIssueDetector) getDMLIssues(query string) ([]QueryIssue, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}
	isDDL, err := queryparser.IsDDL(parseTree)
	if err != nil {
		return nil, fmt.Errorf("error checking if query is a DDL: %v", err)
	}
	if isDDL {
		//Skip all the DDLs coming to this function
		return nil, nil
	}
	issues, err := p.genericIssues(query)
	if err != nil {
		return issues, err
	}
	return issues, err
}

func (p *ParserIssueDetector) genericIssues(query string) ([]QueryIssue, error) {
	parseTree, err := queryparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}
	var result []QueryIssue
	visited := make(map[protoreflect.Message]bool)
	detectors := []UnsupportedConstructDetector{
		NewFuncCallDetector(query),
		NewColumnRefDetector(query),
		NewXmlExprDetector(query),
		NewRangeTableFuncDetector(query),
		NewSelectStmtDetector(query),
		NewCopyCommandUnsupportedConstructsDetector(query),
		NewJsonConstructorFuncDetector(query),
		NewJsonQueryFunctionDetector(query),
		NewMergeStatementDetector(query),
		NewJsonbSubscriptingDetector(query, p.jsonbColumns, p.getJsonbReturnTypeFunctions()),
		NewUniqueNullsNotDistinctDetector(query),
		NewJsonPredicateExprDetector(query),
		NewNonDecimalIntegerLiteralDetector(query),
		NewCommonTableExpressionDetector(query),
		NewDatabaseOptionsDetector(query),
		NewListenNotifyIssueDetector(query),
		NewTwoPhaseCommitDetector(query),
	}

	processor := func(msg protoreflect.Message) error {
		for _, detector := range detectors {
			log.Debugf("running detector %T", detector)
			err := detector.Detect(msg)
			if err != nil {
				log.Debugf("error in detector %T: %v", detector, err)
				return fmt.Errorf("error in detectors %T: %w", detector, err)
			}
		}
		return nil
	}

	parseTreeProtoMsg := queryparser.GetProtoMessageFromParseTree(parseTree)
	err = queryparser.TraverseParseTree(parseTreeProtoMsg, visited, processor)
	if err != nil {
		return result, fmt.Errorf("error traversing parse tree message: %w", err)
	}

	xmlIssueAdded := false
	for _, detector := range detectors {
		issues := detector.GetIssues()
		for _, issue := range issues {
			if issue.Type == XML_FUNCTIONS {
				if xmlIssueAdded {
					// currently, both FuncCallDetector and XmlExprDetector can detect XMLFunctionsIssue
					// but we want to only return one XMLFunctionsIssue.
					// TODO: refactor to avoid this
					// Possible Solutions:
					// 1. Have a dedicated detector for XMLFunctions and Expressions so that a single issue is returned
					// 2. Separate issue types for XML Functions and XML expressions.
					continue
				} else {
					xmlIssueAdded = true
				}
			}
			result = append(result, issue)
		}
	}

	return result, nil
}

func (p *ParserIssueDetector) getJsonbReturnTypeFunctions() []string {
	var jsonbFunctions []string
	jsonbColumns := p.jsonbColumns
	for _, function := range p.functionObjects {
		returnType := function.ReturnType
		if strings.HasSuffix(returnType, "%TYPE") {
			// e.g. public.table_name.column%TYPE
			qualifiedColumn := strings.TrimSuffix(returnType, "%TYPE")
			parts := strings.Split(qualifiedColumn, ".")
			column := parts[len(parts)-1]
			if slices.Contains(jsonbColumns, column) {
				jsonbFunctions = append(jsonbFunctions, function.FuncName)
			}
		} else {
			// e.g. public.udt_type, text, trigger, jsonb
			parts := strings.Split(returnType, ".")
			typeName := parts[len(parts)-1]
			if typeName == "jsonb" {
				jsonbFunctions = append(jsonbFunctions, function.FuncName)
			}
		}
	}
	jsonbFunctions = append(jsonbFunctions, catalogFunctionsReturningJsonb.ToSlice()...)
	return jsonbFunctions
}

func (p *ParserIssueDetector) IsGinIndexPresentInSchema() bool {
	return p.isGinIndexPresentInSchema
}

func (p *ParserIssueDetector) IsUnloggedTablesIssueFiltered() bool {
	return p.isUnloggedTablesIssueFiltered
}

// DetectMissingForeignKeyIndexes detects missing foreign key indexes after all DDL has been processed
// This method should be called after all DDL statements have been parsed to ensure complete metadata
func (p *ParserIssueDetector) DetectMissingForeignKeyIndexes() []QueryIssue {
	var issues []QueryIssue

	// Process all stored foreign key constraints
	for _, fkConstraint := range p.foreignKeyConstraints {
		// Check if this FK has proper index coverage using existing logic
		if !p.hasProperIndexCoverage(fkConstraint) {
			// Create and add the issue
			issue := p.createMissingFKIndexIssue(fkConstraint)
			issues = append(issues, issue)
		}
	}

	return issues
}

// hasProperIndexCoverage checks if a foreign key has proper index coverage.
// Detects exact matches, column permutations, and composite index prefixes.
// Only considers FK columns as leading columns.
//
// Examples:
// FK (x,y,z) → Index (x,y,z) ✓ (exact match)
// FK (x,y,z) → Index (y,z,x) ✓ (permutation)
// FK (x,y,z) → Index (x,y,z,other) ✓ (prefix)
// FK (x,y,z) → Index (other,x,y,z) ✗ (FK not at start) → Detected as missing index
// FK (x,y,z) → Index (x,y) ✗ (missing column) → Detected as missing index
// FK (x,y,z) → No index ✗ → Detected as missing index
func (p *ParserIssueDetector) hasProperIndexCoverage(fk ForeignKeyConstraint) bool {
	indexes, exists := p.tableIndexes[fk.TableName]
	if !exists {
		return false
	}

	for _, index := range indexes {
		if p.hasIndexCoverage(index, fk.ColumnNames) {
			return true
		}
	}

	return false
}

// hasIndexCoverage checks if an index provides coverage for the given foreign key columns.
// It checks if the FK columns exactly match the leading index columns (in any order).
func (p *ParserIssueDetector) hasIndexCoverage(index *queryparser.Index, fkColumns []string) bool {
	// Extract columns and check for expressions in the prefix
	indexColumns := make([]string, 0)
	for i, param := range index.Params {
		// Check if expression exists in the prefix we need to check
		if i < len(fkColumns) && param.IsExpression {
			return false // Expression in the prefix disqualifies the index
		}
		indexColumns = append(indexColumns, param.ColName)
	}

	// Check if we have enough columns in the index
	if len(indexColumns) < len(fkColumns) {
		return false
	}

	// Check if the FK columns exactly match the leading index columns (in any order).
	indexPrefix := indexColumns[:len(fkColumns)]
	return utils.IsSetEqual(fkColumns, indexPrefix)
}

// createMissingFKIndexIssue creates a QueryIssue from a foreign key constraint
func (p *ParserIssueDetector) createMissingFKIndexIssue(fk ForeignKeyConstraint) QueryIssue {
	// Create fully qualified column names
	qualifiedColumnNames := make([]string, len(fk.ColumnNames))
	for i, colName := range fk.ColumnNames {
		qualifiedColumnNames[i] = fmt.Sprintf("%s.%s", fk.TableName, colName)
	}

	return NewMissingForeignKeyIndexIssue(
		"TABLE",
		fk.TableName,
		"", // sqlStatement - we don't have this in stored constraint
		strings.Join(qualifiedColumnNames, ", "),
		fk.ReferencedTable,
	)
}

func (p *ParserIssueDetector) SetColumnStatistics(columnStats []utils.ColumnStatistics) {
	for _, stat := range columnStats {
		p.columnStatistics[stat.GetQualifiedColumnName()] = stat
	}
}

// addIndexToCoverage adds an index to the table indexes map for checking missing foreign key index issue
func (p *ParserIssueDetector) addIndexToCoverage(index *queryparser.Index) {
	tableName := index.GetTableName()

	// Add the index to the table's index list
	if _, exists := p.tableIndexes[tableName]; !exists {
		p.tableIndexes[tableName] = make([]*queryparser.Index, 0)
	}
	p.tableIndexes[tableName] = append(p.tableIndexes[tableName], index)
}

// addConstraintAsIndex creates a mock index object from a constraint and adds it to the table indexes map.
// This is used for primary keys and unique constraints which are also indexes for missing foreign key index detection.
func (p *ParserIssueDetector) addConstraintAsIndex(schemaName, tableName string, columns []string, constraintName string) {
	// Create mock index parameters from columns
	indexParams := make([]queryparser.IndexParam, len(columns))
	for i, colName := range columns {
		indexParams[i] = queryparser.IndexParam{
			ColName:      colName,
			IsExpression: false, // We can't create expressions in primary keys and unique constraints in PG/YB
		}
	}

	// Create a mock index object
	mockIndex := &queryparser.Index{
		SchemaName: schemaName,
		IndexName:  constraintName,
		TableName:  tableName,
		// Primary keys and unique constraints use btree by default.
		//  Mentioned in the docs here:https://www.postgresql.org/docs/current/sql-createtable.html#:~:text=Adding%20a%20PRIMARY%20KEY%20constraint%20will%20automatically%20create%20a%20unique%20btree%20index%20on%20the%20column%20or%20group%20of%20columns%20used%20in%20the%20constraint.%20That%20index%20has%20the%20same%20name%20as%20the%20primary%20key%20constraint
		AccessMethod: BTREE_ACCESS_METHOD,
		Params:       indexParams,
	}

	// Add to table indexes
	p.addIndexToCoverage(mockIndex)
}

// processTableConstraintsAsIndexes processes primary keys and unique constraints as indexes for foreign key detection.
func (p *ParserIssueDetector) processTableConstraintsAsIndexes(table *queryparser.Table) {
	// Process primary keys
	for _, constraint := range table.Constraints {
		if constraint.ConstraintType == queryparser.PRIMARY_CONSTR_TYPE {
			// Primary key constraints are also indexes for missing foreign key index detection
			// We need to ensure the index is processed and covered
			primaryKeyColumns := constraint.Columns
			if len(primaryKeyColumns) > 0 {
				p.addConstraintAsIndex(table.SchemaName, table.TableName, primaryKeyColumns, constraint.ConstraintName)
			}
		}
	}

	// Process unique constraints
	for _, constraint := range table.Constraints {
		if constraint.ConstraintType == queryparser.UNIQUE_CONSTR_TYPE {
			// Unique constraints are also indexes for missing foreign key index detection
			// We need to ensure the index is processed and covered
			uniqueColumns := constraint.Columns
			if len(uniqueColumns) > 0 {
				p.addConstraintAsIndex(table.SchemaName, table.TableName, uniqueColumns, constraint.ConstraintName)
			}
		}
	}
}

// ======= Functions not use parser right now

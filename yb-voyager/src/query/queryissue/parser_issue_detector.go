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
	IsUnsupportedForIndex  bool
	IsHotspotForRangeIndex bool
	IsJsonb                bool
	IsArray                bool
	IsUserDefinedType      bool

	IsForeignKey         bool
	ReferencedTable      string
	ReferencedColumn     string
	ReferencedColumnType string
}

// foreignKeyConstraint represents a foreign key relationship defined in the schema.
// It captures the child table and columns, and the corresponding referenced parent table and columns.
// This structure is used to defer FK processing until all schema definitions are parsed.
type foreignKeyConstraint struct {
	tableName         string
	columnNames       []string
	referencedTable   string
	referencedColumns []string
}

type ParserIssueDetector struct {
	// key is table name, value is map of column name to ColumnMetadata
	columnMetadata map[string]map[string]*ColumnMetadata

	// list of foreign key constraints in the exported schema
	foreignKeyConstraints []foreignKeyConstraint

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
}

func NewParserIssueDetector() *ParserIssueDetector {
	return &ParserIssueDetector{
		columnMetadata:                          make(map[string]map[string]*ColumnMetadata),
		compositeTypes:                          make([]string, 0),
		enumTypes:                               make([]string, 0),
		partitionedTablesMap:                    make(map[string]bool),
		primaryConsInAlter:                      make(map[string]*queryparser.AlterTable),
		columnStatistics:                        make(map[string]utils.ColumnStatistics),
		foreignKeyConstraints:                   make([]foreignKeyConstraint, 0),
		inheritedFrom:                           make(map[string][]string),
		partitionedFrom:                         make(map[string]string),
		columnsWithUnsupportedIndexDatatypes:    make(map[string]map[string]string),
		columnsWithHotspotRangeIndexesDatatypes: make(map[string]map[string]string),
		jsonbColumns:                            make([]string, 0),
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
		for i, localCol := range fk.columnNames {
			if _, ok := p.columnMetadata[fk.tableName]; !ok {
				p.columnMetadata[fk.tableName] = make(map[string]*ColumnMetadata)
			}
			meta, ok := p.columnMetadata[fk.tableName][localCol]
			if !ok {
				meta = &ColumnMetadata{}
				p.columnMetadata[fk.tableName][localCol] = meta
			}

			meta.IsForeignKey = true
			meta.ReferencedTable = fk.referencedTable
			if i < len(fk.referencedColumns) {
				refCol := fk.referencedColumns[i]
				meta.ReferencedColumn = refCol

				if refMeta, ok := p.columnMetadata[fk.referencedTable][refCol]; ok {
					meta.ReferencedColumnType = refMeta.DataType
				}
			} else {
				log.Warnf("Foreign key column count mismatch for table %s: localCols=%v, refCols=%v",
					fk.tableName, fk.columnNames, fk.referencedColumns)
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
		}

		if alter.ConstraintType == queryparser.FOREIGN_CONSTR_TYPE {
			// Collect the foreign key constraint details from ALTER TABLE statement.
			// These are stored temporarily and will be processed later in FinalizeColumnMetadata,
			// once all tables and columns are parsed and available in columnMetadata.
			p.foreignKeyConstraints = append(p.foreignKeyConstraints, foreignKeyConstraint{
				tableName:         alter.GetObjectName(),
				columnNames:       alter.ConstraintColumns,
				referencedTable:   alter.ConstraintReferencedTable,
				referencedColumns: alter.ConstraintReferencedColumns,
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

			// Always update the full type name
			meta.DataType = p.getFullTypeName(col.TypeName, col.TypeMods)

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
			p.foreignKeyConstraints = append(p.foreignKeyConstraints, foreignKeyConstraint{
				tableName:         tableName,
				columnNames:       constraint.Columns,
				referencedTable:   constraint.ReferencedTable,
				referencedColumns: constraint.ReferencedColumns,
			})
		}
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
	case *queryparser.Function:
		fn, _ := ddlObj.(*queryparser.Function)
		p.functionObjects = append(p.functionObjects, fn)
	}
	return nil
}

/*
getFullTypeName constructs the full type name including type modifiers
for supported PostgreSQL types.

Examples:
  - VARCHAR with typmods [10] becomes "varchar(10)"
  - NUMERIC with typmods [8,2] becomes "numeric(8,2)"
  - Types without relevant modifiers or unsupported formats return just the base type name.

Currently supported:
  - varchar(n)
  - bpchar(n) (PostgreSQL's internal name for char(n))
  - numeric(p, s)
*/
func (p *ParserIssueDetector) getFullTypeName(typeName string, typmods []int32) string {
	switch typeName {
	case "varchar", "bpchar": // varchar(n), char(n)
		if len(typmods) == 1 {
			return fmt.Sprintf("%s(%d)", typeName, typmods[0])
		}
		return typeName

	case "numeric":
		if len(typmods) == 2 {
			return fmt.Sprintf("numeric(%d,%d)", typmods[0], typmods[1])
		}
		return typeName

	default:
		return typeName
	}
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

func (p *ParserIssueDetector) SetColumnStatistics(columnStats []utils.ColumnStatistics) {
	for _, stat := range columnStats {
		p.columnStatistics[stat.GetQualifiedColumnName()] = stat
	}
}

// ======= Functions not use parser right now

func GetRedundantIndexIssues(redundantIndexes []utils.RedundantIndexesInfo) []QueryIssue {

	redundantIndexToInfo := make(map[string]utils.RedundantIndexesInfo)

	//This function helps in resolving the existing index in cases where existing index is also a redundant index on some other index
	//So in such cases we need to report the main existing index.
	/*
		e.g. INDEX idx1 on t(id); INDEX idx2 on t(id, id1); INDEX idx3 on t(id, id1,id2);
		redundant index coming from the script can have
		Redundant - idx1, Existing idx2
		Redundant - idx2, Existing idx3
		So in this case we need to report it like
		Redundant - idx1, Existing idx3
		Redundant - idx2, Existing idx3
	*/
	getRootRedundantIndexInfo := func(currRedundantIndexInfo utils.RedundantIndexesInfo) utils.RedundantIndexesInfo {
		for {
			existingIndexOfCurrRedundant := currRedundantIndexInfo.GetExistingIndexObjectName()
			nextRedundantIndexInfo, ok := redundantIndexToInfo[existingIndexOfCurrRedundant]
			if !ok {
				return currRedundantIndexInfo
			}
			currRedundantIndexInfo = nextRedundantIndexInfo
		}
	}
	for _, redundantIndex := range redundantIndexes {
		redundantIndexToInfo[redundantIndex.GetRedundantIndexObjectName()] = redundantIndex
	}
	for _, redundantIndex := range redundantIndexes {
		rootIndexInfo := getRootRedundantIndexInfo(redundantIndex)
		rootExistingIndex := rootIndexInfo.GetExistingIndexObjectName()
		currentExistingIndex := redundantIndex.GetExistingIndexObjectName()
		if rootExistingIndex != currentExistingIndex {
			//If existing index was redundant index then after figuring out the actual existing index use that to report existing index
			redundantIndex.ExistingIndexName = rootIndexInfo.ExistingIndexName
			redundantIndex.ExistingSchemaName = rootIndexInfo.ExistingSchemaName
			redundantIndex.ExistingTableName = rootIndexInfo.ExistingTableName
			redundantIndex.ExistingIndexDDL = rootIndexInfo.ExistingIndexDDL
			redundantIndexToInfo[redundantIndex.GetRedundantIndexObjectName()] = redundantIndex
		}
	}
	var issues []QueryIssue
	for _, redundantIndexInfo := range redundantIndexToInfo {
		issues = append(issues, NewRedundantIndexIssue(INDEX_OBJECT_TYPE, redundantIndexInfo.GetRedundantIndexObjectName(),
			redundantIndexInfo.RedundantIndexDDL, redundantIndexInfo.ExistingIndexDDL))
	}
	return issues
}

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

	pg_query "github.com/pganalyze/pg_query_go/v6"
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
	IsNotNull              bool

	IsForeignKey             bool
	ReferencedTable          string
	ReferencedColumn         string
	ReferencedColumnType     string  // Stores the base type (e.g., "varchar", "numeric")
	ReferencedColumnTypeMods []int32 // Stores the type modifiers (e.g., [255] for varchar(255), [8,2] for numeric(8,2))
}

// ConstraintMetadata stores metadata about table constraints
type ConstraintMetadata struct {
	Type              string // "PRIMARY", "UNIQUE", "FOREIGN", "CHECK", "EXCLUSION"
	Name              string
	Columns           []string
	IsDeferrable      bool
	ReferencedTable   string
	ReferencedColumns []string
}

// TableMetadata stores all relevant information for a table needed for issue detection
type TableMetadata struct {
	TableName          string
	SchemaName         string
	Columns            map[string]*ColumnMetadata
	Constraints        []ConstraintMetadata
	Indexes            []*queryparser.Index
	IsPartitionedTable bool
	InheritedFrom      []string
	PartitionedFrom    string
	PartitionColumns   []string
	Partitions         []*TableMetadata // Stores direct child partitions
}

// IsColumnNotNull checks if a column is NOT NULL in the table.
// Returns false if the column doesn't exist.
func (tm *TableMetadata) IsColumnNotNull(columnName string) bool {
	if col, exists := tm.Columns[columnName]; exists {
		return col.IsNotNull
	}
	return false
}

// HasPrimaryKey returns true if the table has a primary key.
func (tm *TableMetadata) HasPrimaryKey() bool {
	return len(tm.GetPrimaryKeyConstraints()) > 0
}

// IsPartitioned returns true if the table is a partitioned table or a partition of another table.
func (tm *TableMetadata) IsPartitioned() bool {
	return tm.IsPartitionedTable
}

// IsInherited returns true if the table inherits from other tables.
func (tm *TableMetadata) IsInherited() bool {
	return len(tm.InheritedFrom) > 0
}

// GetPartitionColumns returns the partition columns for this table
func (tm *TableMetadata) GetPartitionColumns() []string {
	return tm.PartitionColumns
}

// GetObjectName returns the fully qualified table name
func (tm *TableMetadata) GetObjectName() string {
	return utils.BuildObjectName(tm.SchemaName, tm.TableName)
}

// GetAllPartitionColumnsInHierarchy returns all partition columns from this table and all its descendants
func (tm *TableMetadata) GetAllPartitionColumnsInHierarchy() []string {
	allColumns := make(map[string]bool)

	// Add current table's partition columns
	for _, col := range tm.GetPartitionColumns() {
		allColumns[col] = true
	}

	// Recursively add partition columns from all descendants
	for _, child := range tm.Partitions {
		childColumns := child.GetAllPartitionColumnsInHierarchy()
		for _, col := range childColumns {
			allColumns[col] = true
		}
	}

	// Convert map back to slice
	return lo.Keys(allColumns)
}

// HasAnyRelatedTablePrimaryKey checks if this table or any of its descendants has a primary key
func (tm *TableMetadata) HasAnyRelatedTablePrimaryKey() bool {
	// Check current table
	if tm.HasPrimaryKey() {
		return true
	}

	// Recursively check all descendants
	for _, child := range tm.Partitions {
		if child.HasAnyRelatedTablePrimaryKey() {
			return true
		}
	}

	return false
}

// GetUniqueConstraints returns all unique constraints as a slice of column name slices.
// Returns empty slice if no unique constraints exist.
func (tm *TableMetadata) GetUniqueConstraints() [][]string {
	constraints := tm.GetConstraintsByType("UNIQUE")
	var uniqueConstraints [][]string
	for _, constraint := range constraints {
		uniqueConstraints = append(uniqueConstraints, constraint.Columns)
	}
	return uniqueConstraints
}

// GetPrimaryKeyConstraints returns all primary key constraints.
// Returns empty slice if no primary key constraints exist.
func (tm *TableMetadata) GetPrimaryKeyConstraints() []ConstraintMetadata {
	return tm.GetConstraintsByType("PRIMARY")
}

// GetForeignKeyConstraints returns all foreign key constraints.
// Returns empty slice if no foreign key constraints exist.
func (tm *TableMetadata) GetForeignKeyConstraints() []ConstraintMetadata {
	return tm.GetConstraintsByType("FOREIGN")
}

// GetConstraintsByType returns all constraints of the specified type.
// Returns empty slice if no constraints of that type exist.
func (tm *TableMetadata) GetConstraintsByType(constraintType string) []ConstraintMetadata {
	var constraints []ConstraintMetadata
	for _, constraint := range tm.Constraints {
		if constraint.Type == constraintType {
			constraints = append(constraints, constraint)
		}
	}
	return constraints
}

// AddConstraint adds a constraint to the table metadata
func (tm *TableMetadata) AddConstraint(constraintType, name string, columns []string, referencedTable string, referencedColumns []string) {
	constraint := ConstraintMetadata{
		Type:              constraintType,
		Name:              name,
		Columns:           append([]string{}, columns...),
		ReferencedTable:   referencedTable,
		ReferencedColumns: append([]string{}, referencedColumns...),
	}
	tm.Constraints = append(tm.Constraints, constraint)
}

type ParserIssueDetector struct {

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

	// Table metadata consolidated into a single structure
	// Key is qualified table name (schema.table), value is TableMetadata
	tablesMetadata map[string]*TableMetadata
}

func NewParserIssueDetector() *ParserIssueDetector {
	return &ParserIssueDetector{

		compositeTypes:                          make([]string, 0),
		enumTypes:                               make([]string, 0),
		primaryConsInAlter:                      make(map[string]*queryparser.AlterTable),
		columnStatistics:                        make(map[string]utils.ColumnStatistics),
		columnsWithUnsupportedIndexDatatypes:    make(map[string]map[string]string),
		columnsWithHotspotRangeIndexesDatatypes: make(map[string]map[string]string),
		jsonbColumns:                            make([]string, 0),
		tablesMetadata:                          make(map[string]*TableMetadata),
	}
}

// Helper methods for ParserIssueDetector to work with TableMetadata
// getOrCreateTableMetadata retrieves existing table metadata or creates new if it doesn't exist.
func (p *ParserIssueDetector) getOrCreateTableMetadata(tableName string) *TableMetadata {
	if tm, exists := p.tablesMetadata[tableName]; exists {
		return tm
	}

	// Parse table name to extract schema and table
	parts := strings.Split(tableName, ".")
	schemaName := "public" // default schema
	tableNameOnly := tableName
	if len(parts) == 2 {
		schemaName = parts[0]
		tableNameOnly = parts[1]
	}

	tm := &TableMetadata{
		TableName:   tableNameOnly,
		SchemaName:  schemaName,
		Columns:     make(map[string]*ColumnMetadata),
		Constraints: make([]ConstraintMetadata, 0),
		Indexes:     make([]*queryparser.Index, 0),
	}
	p.tablesMetadata[tableName] = tm
	return tm
}

// getTableMetadata retrieves existing table metadata. Returns nil if not found.
func (p *ParserIssueDetector) getTableMetadata(tableName string) *TableMetadata {
	return p.tablesMetadata[tableName]
}

// Getter methods for ParserIssueDetector to derive information from tablesMetadata
// getPartitionedTablesMap returns a map of partitioned table names to true.
func (p *ParserIssueDetector) getPartitionedTablesMap() map[string]bool {
	partitionedTables := make(map[string]bool)
	for tableName, tm := range p.tablesMetadata {
		if tm.IsPartitioned() {
			partitionedTables[tableName] = true
		}
	}
	return partitionedTables
}

// getPartitionedFrom returns a map of child tables to their parent partitioned table.
func (p *ParserIssueDetector) getPartitionedFrom() map[string]string {
	partitionedFrom := make(map[string]string)
	for tableName, tm := range p.tablesMetadata {
		if tm.PartitionedFrom != "" {
			partitionedFrom[tableName] = tm.PartitionedFrom
		}
	}
	return partitionedFrom
}

// getInheritedFrom returns a map of child tables to their parent table names.
func (p *ParserIssueDetector) getInheritedFrom() map[string][]string {
	inheritedFrom := make(map[string][]string)
	for tableName, tm := range p.tablesMetadata {
		if len(tm.InheritedFrom) > 0 {
			inheritedFrom[tableName] = tm.InheritedFrom
		}
	}
	return inheritedFrom
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
	for _, tm := range p.tablesMetadata {
		for columnName, meta := range tm.Columns {
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
	for tableName, tm := range p.tablesMetadata {
		for columnName, meta := range tm.Columns {
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
	for tableName, tm := range p.tablesMetadata {
		for columnName, meta := range tm.Columns {
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
	p.finalizeColumnsFromParentMap(p.getInheritedFrom())

	// Finalize column metadata for partitioned tables - copying columns from partitioned parent tables to their child partitions
	// Convert partitionedFrom: map[string]string → map[string][]string first to pass it to finalizeColumnsFromParentMap
	partitionParentMap := make(map[string][]string)
	for child, parent := range p.getPartitionedFrom() {
		partitionParentMap[child] = []string{parent}
	}
	p.finalizeColumnsFromParentMap(partitionParentMap)

	p.finalizeForeignKeyConstraints()

	p.populateColumnMetadataDerivedVars()

	// Build partition hierarchies
	p.buildPartitionHierarchies()
}

// buildPartitionHierarchies builds the direct child relationships for all tables
func (p *ParserIssueDetector) buildPartitionHierarchies() {
	// Initialize Partitions slice for all tables
	for _, tm := range p.tablesMetadata {
		tm.Partitions = []*TableMetadata{}
	}

	// Build parent-child relationships
	for _, tm := range p.tablesMetadata {
		if tm.PartitionedFrom != "" {
			// Find parent table
			for _, parentTM := range p.tablesMetadata {
				// Compare both qualified and unqualified names since PartitionedFrom can be either.
				// CREATE TABLE ... PARTITION OF produces unqualified names (e.g., "sales") because
				// PostgreSQL doesn't include schema info in parse tree, while ALTER TABLE ... ATTACH PARTITION
				// produces qualified names (e.g., "public.sales") when schema is explicitly specified.
				// TODO: Use sqlname for proper naming handling for qualified and unqualified names.
				if parentTM.TableName == tm.PartitionedFrom || parentTM.GetObjectName() == tm.PartitionedFrom {
					parentTM.Partitions = append(parentTM.Partitions, tm)
					break
				}
			}
		}
	}
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
			parentTM, ok := p.tablesMetadata[parent]
			if !ok {
				continue // Parent metadata not found
			}
			childTM := p.getOrCreateTableMetadata(child)
			for colName, colMeta := range parentTM.Columns {
				if _, exists := childTM.Columns[colName]; !exists {
					copy := *colMeta
					childTM.Columns[colName] = &copy
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
	for tableName, tm := range p.tablesMetadata {
		for _, constraint := range tm.GetForeignKeyConstraints() {
			for i, localCol := range constraint.Columns {
				meta, ok := tm.Columns[localCol]
				if !ok {
					meta = &ColumnMetadata{}
					tm.Columns[localCol] = meta
				}

				meta.IsForeignKey = true
				meta.ReferencedTable = constraint.ReferencedTable
				if i < len(constraint.ReferencedColumns) {
					refCol := constraint.ReferencedColumns[i]
					meta.ReferencedColumn = refCol

					if refTM, ok := p.tablesMetadata[constraint.ReferencedTable]; ok {
						if refMeta, ok := refTM.Columns[refCol]; ok {
							// Store the referenced column type and modifiers separately for accurate comparison
							meta.ReferencedColumnType = refMeta.DataType
							meta.ReferencedColumnTypeMods = refMeta.DataTypeMods
						}
					}
				} else {
					log.Warnf("Foreign key column count mismatch for table %s: localCols=%v, refCols=%v",
						tableName, constraint.Columns, constraint.ReferencedColumns)
				}
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

			// Track PK columns
			tm := p.getOrCreateTableMetadata(alter.GetObjectName())

			tm.AddConstraint("PRIMARY", alter.ConstraintName, alter.ConstraintColumns, "", nil)
		}

		if alter.ConstraintType == queryparser.UNIQUE_CONSTR_TYPE {
			// Process unique constraint as index for foreign key detection
			if len(alter.ConstraintColumns) > 0 {
				p.addConstraintAsIndex(alter.SchemaName, alter.TableName, alter.ConstraintColumns, alter.ConstraintName)
			}
			// Track UNIQUE columns
			if len(alter.ConstraintColumns) > 0 {
				qualifiedTable := alter.GetObjectName()
				tm := p.getOrCreateTableMetadata(qualifiedTable)
				tm.AddConstraint("UNIQUE", alter.ConstraintName, alter.ConstraintColumns, "", nil)
			}
		}
		// Track NOT NULL alter commands
		// TODO:
		// Currently we are not processing if there are multiple columns under multiple alter commands in the same query
		// Add support for this
		if alter.SetNotNullColumn != "" {
			qualifiedTable := alter.GetObjectName()
			tm := p.getOrCreateTableMetadata(qualifiedTable)
			if colMeta, exists := tm.Columns[alter.SetNotNullColumn]; exists {
				colMeta.IsNotNull = true
			}
		}
		if alter.DropNotNullColumn != "" {
			qualifiedTable := alter.GetObjectName()
			tm := p.getOrCreateTableMetadata(qualifiedTable)
			if colMeta, exists := tm.Columns[alter.DropNotNullColumn]; exists {
				colMeta.IsNotNull = false
			}
		}

		if alter.ConstraintType == queryparser.FOREIGN_CONSTR_TYPE {
			// Add to Constraints slice for comprehensive tracking
			tm := p.getOrCreateTableMetadata(alter.GetObjectName())
			tm.AddConstraint("FOREIGN", alter.ConstraintName, alter.ConstraintColumns, alter.ConstraintReferencedTable, alter.ConstraintReferencedColumns)
		}

		// If alter is to attach a partitioned table, track it
		if alter.AlterType == queryparser.ATTACH_PARTITION {
			// Update the child table's metadata to track its parent
			childTM := p.getOrCreateTableMetadata(alter.PartitionedChild)
			childTM.PartitionedFrom = alter.GetObjectName()
		}

	case *queryparser.Table:
		table, _ := ddlObj.(*queryparser.Table)

		tableName := table.GetObjectName()

		// Get or create table metadata for this table
		tm := p.getOrCreateTableMetadata(tableName)

		// Set table properties
		tm.IsPartitionedTable = table.IsPartitioned
		if table.IsPartitionOf {
			tm.PartitionedFrom = table.PartitionedFrom
		}
		if table.IsInherited {
			tm.InheritedFrom = append([]string{}, table.InheritedFrom...)
		}
		// Store partition columns
		tm.PartitionColumns = append([]string{}, table.PartitionColumns...)

		for _, col := range table.Columns {
			// Check if metadata already exists (e.g., from deferred FK)
			meta, exists := tm.Columns[col.ColumnName]
			if !exists {
				meta = &ColumnMetadata{}
				tm.Columns[col.ColumnName] = meta
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

		// Track constraints for PK/UK and column-level NOT NULL
		for _, constraint := range table.Constraints {
			switch constraint.ConstraintType {
			case queryparser.PRIMARY_CONSTR_TYPE:
				tm.AddConstraint("PRIMARY", constraint.ConstraintName, constraint.Columns, "", nil)
			case queryparser.UNIQUE_CONSTR_TYPE:
				tm.AddConstraint("UNIQUE", constraint.ConstraintName, constraint.Columns, "", nil)
			case queryparser.FOREIGN_CONSTR_TYPE:
				tm.AddConstraint("FOREIGN", constraint.ConstraintName, constraint.Columns, constraint.ReferencedTable, constraint.ReferencedColumns)
			case pg_query.ConstrType_CONSTR_NOTNULL:
				if len(constraint.Columns) == 1 {
					colName := constraint.Columns[0]
					if col, exists := tm.Columns[colName]; exists {
						col.IsNotNull = true
					}
				}
			}
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

	// Process all foreign key constraints from table metadata
	for tableName, tm := range p.tablesMetadata {
		for _, constraint := range tm.GetForeignKeyConstraints() {
			// Check if this FK has proper index coverage using existing logic
			if !p.hasProperIndexCoverage(constraint, tableName) {
				// Create and add the issue
				issue := p.createMissingFKIndexIssue(constraint, tableName)
				issues = append(issues, issue)
			}
		}
	}

	return issues
}

// DetectPrimaryKeyRecommendations recommends adding a PK when there's a UNIQUE constraint with all NOT NULL columns and no PK
func (p *ParserIssueDetector) DetectPrimaryKeyRecommendations() []QueryIssue {
	var issues []QueryIssue

	for _, tm := range p.tablesMetadata {
		// Handle inherited tables (skip for now as per current logic)
		if tm.IsInherited() {
			continue
		}

		// Skip child partitions - PKs should ideally be defined at the root level
		// PostgreSQL only allows one PK per partitioning hierarchy, and child partitions
		// inherit their PK from the root table
		if tm.PartitionedFrom != "" {
			continue
		}

		// Check if this table or any related table already has a primary key
		// - For regular tables: checks if the table itself has a PK
		// - For partitioned tables: checks if any table in the partitioning hierarchy has a PK
		// If so, we cannot suggest a PK for this table as it would create a conflict
		if tm.HasAnyRelatedTablePrimaryKey() {
			continue
		}

		issues = append(issues, p.detectTablePKRecommendations(tm)...)
	}
	return issues
}

// detectTablePKRecommendations handles PK recommendations for both regular and partitioned tables
func (p *ParserIssueDetector) detectTablePKRecommendations(tm *TableMetadata) []QueryIssue {
	var issues []QueryIssue

	// Get partition columns for partitioned tables
	partitionColumns := tm.GetPartitionColumns()
	isPartitioned := tm.IsPartitioned()

	// For partitioned tables, ensure we have partition columns
	if isPartitioned && len(partitionColumns) == 0 {
		// This shouldn't happen for root-level partitioned tables as they define the partitioning strategy
		return issues
	}

	// For root-level partitioned tables, get all partition columns down the hierarchy
	if isPartitioned && tm.PartitionedFrom == "" {
		partitionColumns = tm.GetAllPartitionColumnsInHierarchy()
	}

	// Collect all qualifying UNIQUE column sets where all columns are NOT NULL
	var options [][]string
	// Currently we only detect PK recommendations for UNIQUE constraints
	// TODO: We should also consider UNIQUE INDEXES for PK recommendations
	// https://yugabyte.atlassian.net/browse/DB-18078
	for _, uniqueCols := range tm.GetUniqueConstraints() {
		// Check if all unique columns are NOT NULL
		allNN := true
		for _, col := range uniqueCols {
			if !tm.IsColumnNotNull(col) {
				allNN = false
				break
			}
		}

		if allNN {
			// For partitioned tables, ensure partition columns are included as the PKs must include
			// all partition columns down the entire hierarchy
			if isPartitioned {
				missingPartitionCols, _ := lo.Difference(partitionColumns, uniqueCols)
				if len(missingPartitionCols) > 0 {
					continue
				}
			}

			// Create fully qualified column names for this option
			qualifiedCols := make([]string, len(uniqueCols))
			for i, col := range uniqueCols {
				qualifiedCols[i] = fmt.Sprintf("%s.%s", tm.GetObjectName(), col)
			}
			options = append(options, qualifiedCols)
		}
	}

	if len(options) > 0 {
		issues = append(issues, NewMissingPrimaryKeyWhenUniqueNotNullIssue("TABLE", tm.GetObjectName(), options))
	}

	return issues
}

// getAllPartitionColumnsInHierarchy returns all partition columns down the entire hierarchy
// for a root-level partitioned table. This includes partition columns from all child tables.
func (p *ParserIssueDetector) getAllPartitionColumnsInHierarchy(tm *TableMetadata) []string {
	// Using a map to do automatic deduplication in case if same column is present in multiple child tables
	allPartitionColumns := make(map[string]bool)

	// Start with the current table's partition columns
	for _, col := range tm.GetPartitionColumns() {
		allPartitionColumns[col] = true
	}

	// Find all child tables that inherit from this table
	// Extract just the table name without schema for comparison
	tableNameOnly := tm.TableName
	for _, childTM := range p.tablesMetadata {
		if childTM.PartitionedFrom == tableNameOnly {
			// Recursively get partition columns from this child and all its descendants
			childPartitionColumns := p.getAllPartitionColumnsInHierarchy(childTM)
			for _, col := range childPartitionColumns {
				allPartitionColumns[col] = true
			}
		}
	}

	// Convert map back to slice
	result := make([]string, 0, len(allPartitionColumns))
	for col := range allPartitionColumns {
		result = append(result, col)
	}

	return result
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
func (p *ParserIssueDetector) hasProperIndexCoverage(constraint ConstraintMetadata, tableName string) bool {
	tm := p.getTableMetadata(tableName)
	if tm == nil {
		return false
	}

	for _, index := range tm.Indexes {
		if p.hasIndexCoverage(index, constraint.Columns) {
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
func (p *ParserIssueDetector) createMissingFKIndexIssue(constraint ConstraintMetadata, tableName string) QueryIssue {
	// Create fully qualified column names
	qualifiedColumnNames := make([]string, len(constraint.Columns))
	for i, colName := range constraint.Columns {
		qualifiedColumnNames[i] = fmt.Sprintf("%s.%s", tableName, colName)
	}

	return NewMissingForeignKeyIndexIssue(
		"TABLE",
		tableName,
		"", // sqlStatement - we don't have this in stored constraint
		strings.Join(qualifiedColumnNames, ", "),
		constraint.ReferencedTable,
	)
}

func (p *ParserIssueDetector) SetColumnStatistics(columnStats []utils.ColumnStatistics) {
	for _, stat := range columnStats {
		p.columnStatistics[stat.GetQualifiedColumnName()] = stat
	}
}

// addIndexToCoverage adds an index to the table metadata for checking missing foreign key index issue
func (p *ParserIssueDetector) addIndexToCoverage(index *queryparser.Index) {
	tableName := index.GetTableName()
	tm := p.getOrCreateTableMetadata(tableName)
	tm.Indexes = append(tm.Indexes, index)
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

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
package queryparser

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/samber/lo"
)

// Base parser interface
type DDLParser interface {
	Parse(*pg_query.ParseResult) (DDLObject, error)
}

// Base DDL object interface
type DDLObject interface {
	GetObjectName() string
	GetSchemaName() string
}

// TableParser handles parsing CREATE TABLE statements
type TableParser struct{}

func NewTableParser() *TableParser {
	return &TableParser{}
}

func (p *TableParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	createTableNode, ok := getCreateTableStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE TABLE statement")
	}

	table := &Table{
		SchemaName:       createTableNode.CreateStmt.Relation.Schemaname,
		TableName:        createTableNode.CreateStmt.Relation.Relname,
		IsUnlogged:       createTableNode.CreateStmt.Relation.GetRelpersistence() == "u",
		GeneratedColumns: make([]string, 0),
	}

	// Parse columns and their properties
	for _, element := range createTableNode.CreateStmt.TableElts {
		if colDef := element.GetColumnDef(); colDef != nil {
			if p.isGeneratedColumn(colDef) {
				table.GeneratedColumns = append(table.GeneratedColumns, colDef.Colname)
			}
		}
	}

	return table, nil
}

func (p *TableParser) isGeneratedColumn(colDef *pg_query.ColumnDef) bool {
	for _, constraint := range colDef.Constraints {
		if constraint.GetConstraint().Contype == pg_query.ConstrType_CONSTR_GENERATED {
			return true
		}
	}
	return false
}

// IndexParser handles parsing CREATE INDEX statements
type IndexParser struct{}

func NewIndexParser() *IndexParser {
	return &IndexParser{}
}

func (p *IndexParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	indexNode, ok := getCreateIndexStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE INDEX statement")
	}

	index := &Index{
		SchemaName:        indexNode.IndexStmt.Relation.Schemaname,
		IndexName:         indexNode.IndexStmt.Idxname,
		TableName:         indexNode.IndexStmt.Relation.Relname,
		AccessMethod:      indexNode.IndexStmt.AccessMethod,
		NumStorageOptions: len(indexNode.IndexStmt.GetOptions()),
	}

	return index, nil
}

// AlterTableParser handles parsing ALTER TABLE statements
type AlterTableParser struct{}

func NewAlterTableParser() *AlterTableParser {
	return &AlterTableParser{}
}

func (p *AlterTableParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	alterNode, ok := getAlterStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not an ALTER TABLE statement")
	}

	alter := &AlterTable{
		SchemaName: alterNode.AlterTableStmt.Relation.Schemaname,
		TableName:  alterNode.AlterTableStmt.Relation.Relname,
		AlterType:  alterNode.AlterTableStmt.Cmds[0].GetAlterTableCmd().GetSubtype(),
	}

	// Parse specific alter command
	cmd := alterNode.AlterTableStmt.Cmds[0].GetAlterTableCmd()
	switch alter.AlterType {
	case pg_query.AlterTableType_AT_SetOptions:
		alter.NumSetAttributes = len(cmd.GetDef().GetList().GetItems())
	case pg_query.AlterTableType_AT_AddConstraint:
		alter.NumStorageOptions = len(cmd.GetDef().GetConstraint().GetOptions())
        /*
        e.g. ALTER TABLE ONLY public.meeting ADD CONSTRAINT no_time_overlap EXCLUDE USING gist (room_id WITH =, time_range WITH &&);
        cmds:{alter_table_cmd:{subtype:AT_AddConstraint def:{constraint:{contype:CONSTR_EXCLUSION conname:"no_time_overlap" location:41
        here again same checking the definition of the alter stmt if it has constraint and checking its type
        */
        constraint := cmd.GetDef().GetConstraint()
        alter.ConstraintType = constraint.Contype
        alter.ConstraintName = constraint.Conname
        alter.IsDeferrable = constraint.Deferrable
        
	case pg_query.AlterTableType_AT_DisableRule:
		alter.RuleName = cmd.Name
	}
	
	return alter, nil
}

// DDL Objects
type Table struct {
	SchemaName       string
	TableName        string
	IsUnlogged       bool
	GeneratedColumns []string
}

func (t *Table) GetObjectName() string {
	qualifiedTable := lo.Ternary(t.SchemaName != "", fmt.Sprintf("%s.%s", t.SchemaName, t.TableName), t.TableName)
	return qualifiedTable
}
func (t *Table) GetSchemaName() string { return t.SchemaName }

type Index struct {
	SchemaName        string
	IndexName         string
	TableName         string
	AccessMethod      string
	NumStorageOptions int
}

func (i *Index) GetObjectName() string {
	qualifiedTable := lo.Ternary(i.SchemaName != "", fmt.Sprintf("%s.%s", i.SchemaName, i.TableName), i.TableName)
	return fmt.Sprintf("%s ON %s", i.IndexName, qualifiedTable)
}
func (i *Index) GetSchemaName() string { return i.SchemaName }

const (
	ADD_CONSTRAINT        = pg_query.AlterTableType_AT_AddConstraint
	SET_OPTIONS           = pg_query.AlterTableType_AT_SetOptions
	DISABLE_RULE          = pg_query.AlterTableType_AT_DisableRule
	CLUSTER_ON            = pg_query.AlterTableType_AT_ClusterOn
	EXCLUSION_CONSTR_TYPE = pg_query.ConstrType_CONSTR_EXCLUSION
	FOREIGN_CONSTR_TYPE   = pg_query.ConstrType_CONSTR_FOREIGN
)

type AlterTable struct {
	SchemaName        string
	TableName         string
	AlterType         pg_query.AlterTableType
	RuleName          string
	NumSetAttributes  int
	NumStorageOptions int
	//In case AlterType - ADD_CONSTRAINT
	ConstraintType pg_query.ConstrType
	ConstraintName string
	IsDeferrable   bool
}

func (a *AlterTable) GetObjectName() string {
	qualifiedTable := lo.Ternary(a.SchemaName != "", fmt.Sprintf("%s.%s", a.SchemaName, a.TableName), a.TableName)
	return qualifiedTable
}
func (a *AlterTable) GetSchemaName() string { return a.SchemaName }

//No op parser for objects we don't have parser yet

type NoOpParser struct{}

func NewNoOpParser() *NoOpParser {
	return &NoOpParser{}
}

type Object struct {
	ObjectName string
	SchemaName string
}

func (o *Object) GetObjectName() string { return o.ObjectName }
func (o *Object) GetSchemaName() string { return o.SchemaName }

func (g *NoOpParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	return &Object{}, nil
}

func GetDDLParser(parseTree *pg_query.ParseResult) (DDLParser, error) {
	switch {
	case IsCreateTable(parseTree):
		return NewTableParser(), nil
	case IsCreateIndex(parseTree):
		return NewIndexParser(), nil
	case IsAlterTable(parseTree):
		return NewAlterTableParser(), nil
	default:
		return NewNoOpParser(), nil
	}
}

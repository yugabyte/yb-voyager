package schemadiff

import (
	"database/sql"
	"time"
)

type QueryExecutor interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

type SnapshotProvider interface {
	TakeSnapshot(db QueryExecutor, schemas []string) (*SchemaSnapshot, error)
	DatabaseType() string
}

type SchemaSnapshot struct {
	DatabaseType string    `json:"database_type"` // "postgresql" or "yugabytedb"
	Schemas      []string  `json:"schemas"`
	CapturedAt   time.Time `json:"captured_at"`

	Tables      []Table      `json:"tables"`
	Columns     []Column     `json:"columns"`
	Constraints []Constraint `json:"constraints"`
	Indexes     []Index      `json:"indexes"`
	Sequences   []Sequence   `json:"sequences"`
	Views       []View       `json:"views"`
	MVs         []View       `json:"materialized_views"`
	Functions   []Function   `json:"functions"`
	Triggers    []Trigger    `json:"triggers"`
	EnumTypes   []EnumType   `json:"enum_types"`
	DomainTypes []DomainType `json:"domain_types"`
	Extensions  []Extension  `json:"extensions"`
	Policies    []Policy     `json:"policies"`
}

type Table struct {
	Schema             string `json:"schema"`
	Name               string `json:"name"`
	IsPartitioned      bool   `json:"is_partitioned"`
	PartitionStrategy  string `json:"partition_strategy,omitempty"`
	PartitionKey       string `json:"partition_key,omitempty"`
	ParentTable        string `json:"parent_table,omitempty"`
	ReplicaIdentity    string `json:"replica_identity,omitempty"`
}

func (t Table) QualifiedName() string { return t.Schema + "." + t.Name }

type Column struct {
	Schema         string `json:"schema"`
	TableName      string `json:"table_name"`
	Name           string `json:"name"`
	DataType       string `json:"data_type"`
	TypeModifier   string `json:"type_modifier,omitempty"`
	IsNullable     bool   `json:"is_nullable"`
	DefaultExpr    string `json:"default_expr,omitempty"`
	IsIdentity     string `json:"is_identity,omitempty"` // "a" (ALWAYS), "d" (BY DEFAULT), "" (none)
	OrdinalPos     int    `json:"ordinal_pos"`
	Collation      string `json:"collation,omitempty"`
	GeneratedExpr  string `json:"generated_expr,omitempty"`
	IsGenerated    string `json:"is_generated,omitempty"` // "s" (stored), "" (none)
}

func (c Column) QualifiedName() string {
	return c.Schema + "." + c.TableName + "." + c.Name
}

type ConstraintType string

const (
	ConstraintPK        ConstraintType = "p"
	ConstraintUnique    ConstraintType = "u"
	ConstraintFK        ConstraintType = "f"
	ConstraintCheck     ConstraintType = "c"
	ConstraintExclusion ConstraintType = "x"
)

type Constraint struct {
	Schema          string         `json:"schema"`
	TableName       string         `json:"table_name"`
	Name            string         `json:"name"`
	Type            ConstraintType `json:"type"`
	Columns         []string       `json:"columns,omitempty"`
	Expression      string         `json:"expression,omitempty"`
	RefSchema       string         `json:"ref_schema,omitempty"`
	RefTable        string         `json:"ref_table,omitempty"`
	RefColumns      []string       `json:"ref_columns,omitempty"`
	OnDelete        string         `json:"on_delete,omitempty"`
	OnUpdate        string         `json:"on_update,omitempty"`
	IsDeferrable    bool           `json:"is_deferrable,omitempty"`
	IsDeferred      bool           `json:"is_deferred,omitempty"`
}

func (c Constraint) QualifiedName() string {
	return c.Schema + "." + c.TableName + "." + c.Name
}

type Index struct {
	Schema       string   `json:"schema"`
	TableName    string   `json:"table_name"`
	Name         string   `json:"name"`
	Columns      []string `json:"columns"`
	IsUnique     bool     `json:"is_unique"`
	AccessMethod string   `json:"access_method"`
	IsPartial    bool     `json:"is_partial"`
	WhereClause  string   `json:"where_clause,omitempty"`
	IncludeCols  []string `json:"include_cols,omitempty"`
	IndexDef     string   `json:"index_def,omitempty"`
	SortOptions  []string `json:"sort_options,omitempty"`
}

func (i Index) QualifiedName() string { return i.Schema + "." + i.Name }

type Sequence struct {
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	DataType  string `json:"data_type"`
	OwnedBy   string `json:"owned_by,omitempty"`
	StartVal  int64  `json:"start_value"`
	Increment int64  `json:"increment"`
	MinVal    int64  `json:"min_value"`
	MaxVal    int64  `json:"max_value"`
	IsCyclic  bool   `json:"is_cyclic"`
	CacheSize int64  `json:"cache_size"`
}

func (s Sequence) QualifiedName() string { return s.Schema + "." + s.Name }

type View struct {
	Schema     string   `json:"schema"`
	Name       string   `json:"name"`
	Definition string   `json:"definition"`
	Columns    []string `json:"columns,omitempty"`
}

func (v View) QualifiedName() string { return v.Schema + "." + v.Name }

type Function struct {
	Schema          string `json:"schema"`
	Name            string `json:"name"`
	Arguments       string `json:"arguments"`
	ReturnType      string `json:"return_type"`
	Language        string `json:"language"`
	Kind            string `json:"kind"` // "f" (function), "p" (procedure), "a" (aggregate)
	Volatility      string `json:"volatility"`
	IsSecurityDef   bool   `json:"is_security_definer"`
	IsStrict        bool   `json:"is_strict"`
	ParallelSafety  string `json:"parallel_safety"`
}

func (f Function) QualifiedName() string {
	return f.Schema + "." + f.Name + "(" + f.Arguments + ")"
}

type Trigger struct {
	Schema          string `json:"schema"`
	TableName       string `json:"table_name"`
	Name            string `json:"name"`
	Timing          string `json:"timing"` // BEFORE, AFTER, INSTEAD OF
	Events          string `json:"events"` // INSERT, UPDATE, DELETE, TRUNCATE (combined)
	ForEachRow      bool   `json:"for_each_row"`
	FunctionName    string `json:"function_name"`
	IsEnabled       string `json:"is_enabled"` // O (origin), D (disabled), R (replica), A (always)
	IsConstraint    bool   `json:"is_constraint"`
}

func (t Trigger) QualifiedName() string {
	return t.Schema + "." + t.TableName + "." + t.Name
}

type EnumType struct {
	Schema string   `json:"schema"`
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func (e EnumType) QualifiedName() string { return e.Schema + "." + e.Name }

type DomainType struct {
	Schema     string `json:"schema"`
	Name       string `json:"name"`
	BaseType   string `json:"base_type"`
	IsNullable bool   `json:"is_nullable"`
	Default    string `json:"default,omitempty"`
	CheckExpr  string `json:"check_expr,omitempty"`
}

func (d DomainType) QualifiedName() string { return d.Schema + "." + d.Name }

type Extension struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Policy struct {
	Schema        string `json:"schema"`
	TableName     string `json:"table_name"`
	Name          string `json:"name"`
	IsPermissive  bool   `json:"is_permissive"`
	Command       string `json:"command"`
	UsingExpr     string `json:"using_expr,omitempty"`
	WithCheckExpr string `json:"with_check_expr,omitempty"`
}

func (p Policy) QualifiedName() string {
	return p.Schema + "." + p.TableName + "." + p.Name
}

func NewSnapshotProvider(dbType string) SnapshotProvider {
	switch dbType {
	case "postgresql", "yugabytedb":
		return &PostgresSnapshotProvider{dbType: dbType}
	default:
		return nil
	}
}

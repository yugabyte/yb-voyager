package schemadiff

import "fmt"

type DiffType string

const (
	DiffAdded    DiffType = "ADDED"
	DiffDropped  DiffType = "DROPPED"
	DiffModified DiffType = "MODIFIED"
)

type ObjectType string

const (
	ObjectTable      ObjectType = "TABLE"
	ObjectColumn     ObjectType = "COLUMN"
	ObjectConstraint ObjectType = "CONSTRAINT"
	ObjectIndex      ObjectType = "INDEX"
	ObjectSequence   ObjectType = "SEQUENCE"
	ObjectView       ObjectType = "VIEW"
	ObjectMView      ObjectType = "MATERIALIZED VIEW"
	ObjectFunction   ObjectType = "FUNCTION"
	ObjectTrigger    ObjectType = "TRIGGER"
	ObjectEnumType   ObjectType = "ENUM TYPE"
	ObjectDomainType ObjectType = "DOMAIN TYPE"
	ObjectExtension  ObjectType = "EXTENSION"
	ObjectPolicy     ObjectType = "POLICY"
)

type DiffEntry struct {
	ObjectType  ObjectType  `json:"object_type"`
	ObjectName  string      `json:"object_name"`
	DiffType    DiffType    `json:"diff_type"`
	Property    string      `json:"property,omitempty"`
	OldValue    string      `json:"old_value,omitempty"`
	NewValue    string      `json:"new_value,omitempty"`
	ImpactLevel ImpactLevel `json:"impact_level"`
}

func (d DiffEntry) Summary() string {
	switch d.DiffType {
	case DiffAdded:
		return fmt.Sprintf("[%s ADDED] %s", d.ObjectType, d.ObjectName)
	case DiffDropped:
		return fmt.Sprintf("[%s DROPPED] %s", d.ObjectType, d.ObjectName)
	case DiffModified:
		return fmt.Sprintf("[%s %s] %s", d.ObjectType, d.Property, d.ObjectName)
	}
	return ""
}

type DiffResult struct {
	Entries []DiffEntry `json:"entries"`
}

func Diff(left, right *SchemaSnapshot) *DiffResult {
	result := &DiffResult{}
	diffTables(left, right, result)
	diffColumns(left, right, result)
	diffConstraints(left, right, result)
	diffIndexes(left, right, result)
	diffSequences(left, right, result)
	diffViews(left.Views, right.Views, ObjectView, result)
	diffViews(left.MVs, right.MVs, ObjectMView, result)
	diffFunctions(left, right, result)
	diffTriggers(left, right, result)
	diffEnumTypes(left, right, result)
	diffDomainTypes(left, right, result)
	diffExtensions(left, right, result)
	diffPolicies(left, right, result)
	classifyAll(result)
	return result
}

func diffTables(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Table, len(left.Tables))
	for _, t := range left.Tables {
		leftMap[t.QualifiedName()] = t
	}
	rightMap := make(map[string]Table, len(right.Tables))
	for _, t := range right.Tables {
		rightMap[t.QualifiedName()] = t
	}

	for name, lt := range leftMap {
		rt, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTable, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if lt.IsPartitioned != rt.IsPartitioned {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTable, ObjectName: name,
				DiffType: DiffModified, Property: "IS_PARTITIONED",
				OldValue: fmt.Sprintf("%v", lt.IsPartitioned),
				NewValue: fmt.Sprintf("%v", rt.IsPartitioned),
			})
		}
		if lt.PartitionStrategy != rt.PartitionStrategy {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTable, ObjectName: name,
				DiffType: DiffModified, Property: "PARTITION_STRATEGY",
				OldValue: lt.PartitionStrategy, NewValue: rt.PartitionStrategy,
			})
		}
		if lt.ReplicaIdentity != rt.ReplicaIdentity {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTable, ObjectName: name,
				DiffType: DiffModified, Property: "REPLICA_IDENTITY",
				OldValue: lt.ReplicaIdentity, NewValue: rt.ReplicaIdentity,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTable, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffColumns(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Column, len(left.Columns))
	for _, c := range left.Columns {
		leftMap[c.QualifiedName()] = c
	}
	rightMap := make(map[string]Column, len(right.Columns))
	for _, c := range right.Columns {
		rightMap[c.QualifiedName()] = c
	}

	for name, lc := range leftMap {
		rc, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectColumn, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		fullType := func(c Column) string {
			if c.TypeModifier != "" {
				return c.DataType + "(" + c.TypeModifier + ")"
			}
			return c.DataType
		}
		if fullType(lc) != fullType(rc) {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectColumn, ObjectName: name,
				DiffType: DiffModified, Property: "TYPE_CHANGED",
				OldValue: fullType(lc), NewValue: fullType(rc),
			})
		}
		if lc.IsNullable != rc.IsNullable {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectColumn, ObjectName: name,
				DiffType: DiffModified, Property: "NULLABLE_CHANGED",
				OldValue: fmt.Sprintf("%v", lc.IsNullable),
				NewValue: fmt.Sprintf("%v", rc.IsNullable),
			})
		}
		if lc.DefaultExpr != rc.DefaultExpr {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectColumn, ObjectName: name,
				DiffType: DiffModified, Property: "DEFAULT_CHANGED",
				OldValue: lc.DefaultExpr, NewValue: rc.DefaultExpr,
			})
		}
		if lc.IsIdentity != rc.IsIdentity {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectColumn, ObjectName: name,
				DiffType: DiffModified, Property: "IDENTITY_CHANGED",
				OldValue: lc.IsIdentity, NewValue: rc.IsIdentity,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectColumn, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffConstraints(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Constraint, len(left.Constraints))
	for _, c := range left.Constraints {
		leftMap[c.QualifiedName()] = c
	}
	rightMap := make(map[string]Constraint, len(right.Constraints))
	for _, c := range right.Constraints {
		rightMap[c.QualifiedName()] = c
	}

	for name, lc := range leftMap {
		rc, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectConstraint, ObjectName: name,
				DiffType: DiffDropped,
				Property: string(lc.Type),
			})
			continue
		}
		if lc.Type != rc.Type {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectConstraint, ObjectName: name,
				DiffType: DiffModified, Property: "TYPE_CHANGED",
				OldValue: string(lc.Type), NewValue: string(rc.Type),
			})
		}
		if !sliceEqual(lc.Columns, rc.Columns) {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectConstraint, ObjectName: name,
				DiffType: DiffModified, Property: "COLUMNS_CHANGED",
				OldValue: fmt.Sprintf("%v", lc.Columns),
				NewValue: fmt.Sprintf("%v", rc.Columns),
			})
		}
		if lc.Expression != rc.Expression {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectConstraint, ObjectName: name,
				DiffType: DiffModified, Property: "EXPRESSION_CHANGED",
				OldValue: lc.Expression, NewValue: rc.Expression,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectConstraint, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffIndexes(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Index, len(left.Indexes))
	for _, i := range left.Indexes {
		leftMap[i.QualifiedName()] = i
	}
	rightMap := make(map[string]Index, len(right.Indexes))
	for _, i := range right.Indexes {
		rightMap[i.QualifiedName()] = i
	}

	for name, li := range leftMap {
		ri, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectIndex, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if li.AccessMethod != ri.AccessMethod {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectIndex, ObjectName: name,
				DiffType: DiffModified, Property: "METHOD_CHANGED",
				OldValue: li.AccessMethod, NewValue: ri.AccessMethod,
			})
		}
		if !sliceEqual(li.Columns, ri.Columns) {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectIndex, ObjectName: name,
				DiffType: DiffModified, Property: "COLUMNS_CHANGED",
				OldValue: fmt.Sprintf("%v", li.Columns),
				NewValue: fmt.Sprintf("%v", ri.Columns),
			})
		}
		if li.IsUnique != ri.IsUnique {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectIndex, ObjectName: name,
				DiffType: DiffModified, Property: "UNIQUENESS_CHANGED",
				OldValue: fmt.Sprintf("%v", li.IsUnique),
				NewValue: fmt.Sprintf("%v", ri.IsUnique),
			})
		}
		if li.WhereClause != ri.WhereClause {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectIndex, ObjectName: name,
				DiffType: DiffModified, Property: "WHERE_CHANGED",
				OldValue: li.WhereClause, NewValue: ri.WhereClause,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectIndex, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffSequences(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Sequence, len(left.Sequences))
	for _, s := range left.Sequences {
		leftMap[s.QualifiedName()] = s
	}
	rightMap := make(map[string]Sequence, len(right.Sequences))
	for _, s := range right.Sequences {
		rightMap[s.QualifiedName()] = s
	}

	for name, ls := range leftMap {
		rs, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectSequence, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if ls.DataType != rs.DataType {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectSequence, ObjectName: name,
				DiffType: DiffModified, Property: "TYPE_CHANGED",
				OldValue: ls.DataType, NewValue: rs.DataType,
			})
		}
		if ls.Increment != rs.Increment {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectSequence, ObjectName: name,
				DiffType: DiffModified, Property: "INCREMENT_CHANGED",
				OldValue: fmt.Sprintf("%d", ls.Increment),
				NewValue: fmt.Sprintf("%d", rs.Increment),
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectSequence, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffViews(leftViews, rightViews []View, objType ObjectType, result *DiffResult) {
	leftMap := make(map[string]View, len(leftViews))
	for _, v := range leftViews {
		leftMap[v.QualifiedName()] = v
	}
	rightMap := make(map[string]View, len(rightViews))
	for _, v := range rightViews {
		rightMap[v.QualifiedName()] = v
	}

	for name, lv := range leftMap {
		rv, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: objType, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if lv.Definition != rv.Definition {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: objType, ObjectName: name,
				DiffType: DiffModified, Property: "DEFINITION_CHANGED",
				OldValue: lv.Definition, NewValue: rv.Definition,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: objType, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffFunctions(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Function, len(left.Functions))
	for _, f := range left.Functions {
		leftMap[f.QualifiedName()] = f
	}
	rightMap := make(map[string]Function, len(right.Functions))
	for _, f := range right.Functions {
		rightMap[f.QualifiedName()] = f
	}

	for name, lf := range leftMap {
		rf, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectFunction, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if lf.ReturnType != rf.ReturnType {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectFunction, ObjectName: name,
				DiffType: DiffModified, Property: "RETURN_TYPE_CHANGED",
				OldValue: lf.ReturnType, NewValue: rf.ReturnType,
			})
		}
		if lf.Language != rf.Language {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectFunction, ObjectName: name,
				DiffType: DiffModified, Property: "LANGUAGE_CHANGED",
				OldValue: lf.Language, NewValue: rf.Language,
			})
		}
		if lf.Volatility != rf.Volatility {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectFunction, ObjectName: name,
				DiffType: DiffModified, Property: "VOLATILITY_CHANGED",
				OldValue: lf.Volatility, NewValue: rf.Volatility,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectFunction, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffTriggers(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Trigger, len(left.Triggers))
	for _, t := range left.Triggers {
		leftMap[t.QualifiedName()] = t
	}
	rightMap := make(map[string]Trigger, len(right.Triggers))
	for _, t := range right.Triggers {
		rightMap[t.QualifiedName()] = t
	}

	for name, lt := range leftMap {
		rt, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTrigger, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if lt.FunctionName != rt.FunctionName {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTrigger, ObjectName: name,
				DiffType: DiffModified, Property: "FUNCTION_CHANGED",
				OldValue: lt.FunctionName, NewValue: rt.FunctionName,
			})
		}
		if lt.Timing != rt.Timing || lt.Events != rt.Events {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTrigger, ObjectName: name,
				DiffType: DiffModified, Property: "DEFINITION_CHANGED",
				OldValue: lt.Timing + " " + lt.Events,
				NewValue: rt.Timing + " " + rt.Events,
			})
		}
		if lt.IsEnabled != rt.IsEnabled {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTrigger, ObjectName: name,
				DiffType: DiffModified, Property: "ENABLED_CHANGED",
				OldValue: lt.IsEnabled, NewValue: rt.IsEnabled,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectTrigger, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffEnumTypes(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]EnumType, len(left.EnumTypes))
	for _, e := range left.EnumTypes {
		leftMap[e.QualifiedName()] = e
	}
	rightMap := make(map[string]EnumType, len(right.EnumTypes))
	for _, e := range right.EnumTypes {
		rightMap[e.QualifiedName()] = e
	}

	for name, le := range leftMap {
		re, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectEnumType, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if !sliceEqual(le.Values, re.Values) {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectEnumType, ObjectName: name,
				DiffType: DiffModified, Property: "VALUES_CHANGED",
				OldValue: fmt.Sprintf("%v", le.Values),
				NewValue: fmt.Sprintf("%v", re.Values),
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectEnumType, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffDomainTypes(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]DomainType, len(left.DomainTypes))
	for _, d := range left.DomainTypes {
		leftMap[d.QualifiedName()] = d
	}
	rightMap := make(map[string]DomainType, len(right.DomainTypes))
	for _, d := range right.DomainTypes {
		rightMap[d.QualifiedName()] = d
	}

	for name, ld := range leftMap {
		rd, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectDomainType, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if ld.BaseType != rd.BaseType {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectDomainType, ObjectName: name,
				DiffType: DiffModified, Property: "BASE_TYPE_CHANGED",
				OldValue: ld.BaseType, NewValue: rd.BaseType,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectDomainType, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffExtensions(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Extension, len(left.Extensions))
	for _, e := range left.Extensions {
		leftMap[e.Name] = e
	}
	rightMap := make(map[string]Extension, len(right.Extensions))
	for _, e := range right.Extensions {
		rightMap[e.Name] = e
	}

	for name, le := range leftMap {
		re, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectExtension, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if le.Version != re.Version {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectExtension, ObjectName: name,
				DiffType: DiffModified, Property: "VERSION_CHANGED",
				OldValue: le.Version, NewValue: re.Version,
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectExtension, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func diffPolicies(left, right *SchemaSnapshot, result *DiffResult) {
	leftMap := make(map[string]Policy, len(left.Policies))
	for _, p := range left.Policies {
		leftMap[p.QualifiedName()] = p
	}
	rightMap := make(map[string]Policy, len(right.Policies))
	for _, p := range right.Policies {
		rightMap[p.QualifiedName()] = p
	}

	for name, lp := range leftMap {
		rp, exists := rightMap[name]
		if !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectPolicy, ObjectName: name,
				DiffType: DiffDropped,
			})
			continue
		}
		if lp.UsingExpr != rp.UsingExpr || lp.WithCheckExpr != rp.WithCheckExpr {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectPolicy, ObjectName: name,
				DiffType: DiffModified, Property: "EXPRESSION_CHANGED",
			})
		}
	}

	for name := range rightMap {
		if _, exists := leftMap[name]; !exists {
			result.Entries = append(result.Entries, DiffEntry{
				ObjectType: ObjectPolicy, ObjectName: name,
				DiffType: DiffAdded,
			})
		}
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

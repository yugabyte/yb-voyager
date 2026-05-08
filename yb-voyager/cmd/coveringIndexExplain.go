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

// Low-level helpers for the covering-index EXPLAIN pipeline:
//   - AST walkers to build alias maps and resolve ParamRef -> (schema, table, column)
//   - PREPARE / parameter-type discovery / DEALLOCATE
//   - Per-column value sampling (cached)
//   - Value tuple construction
//   - EXPLAIN (FORMAT JSON) EXECUTE

package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
)

// nullParamSentinel is a magic value used in the EXPLAIN EXECUTE value tuples
// to request an unquoted SQL `NULL` literal at that parameter slot. Used by
// the partial-index sampling path to satisfy `col IS NULL` predicates.
const nullParamSentinel = "\x00__null__\x00"

var relationHasColumnCache = map[string]bool{}
var relationHasColumnFn = relationHasColumnFromCatalog

// ---------- AST utilities ----------

// asParseResult is a narrow helper used by buildAliasMapFromStmts to accept
// *pg_query.ParseResult through an interface{} (used so the walker declaration
// can be placed in the orchestrator file without needing pg_query there).
func asParseResult(v interface{}) *pg_query.ParseResult {
	if pr, ok := v.(*pg_query.ParseResult); ok {
		return pr
	}
	return nil
}

// collectRangeVars walks a node tree and records every RangeVar alias / real name
// into am, marking ambiguous names in ambiguous.
func collectRangeVars(root *pg_query.Node, am map[string]paramTarget, ambiguous map[string]bool) {
	visit(root, func(n *pg_query.Node) {
		rv, ok := n.Node.(*pg_query.Node_RangeVar)
		if !ok {
			return
		}
		schema := rv.RangeVar.GetSchemaname()
		table := rv.RangeVar.GetRelname()
		if table == "" {
			return
		}
		record := func(name string) {
			existing, seen := am[name]
			tgt := paramTarget{schema: schema, table: table}
			if seen && (existing.schema != schema || existing.table != table) {
				ambiguous[name] = true
				return
			}
			am[name] = tgt
		}
		record(table)
		if alias := rv.RangeVar.GetAlias(); alias != nil && alias.GetAliasname() != "" {
			record(alias.GetAliasname())
		}
	})
}

// collectParamLinksFromNode walks a node tree and fills out[paramIdx] =
// paramTarget for every A_Expr whose operands pair a ColumnRef with one or
// more ParamRefs.
//
// Supported shapes (matching plan):
//   - ColumnRef op ParamRef
//   - ParamRef op ColumnRef
//   - ColumnRef IN (ParamRef, ...)           (AEXPR_IN, rexpr is a List of ParamRef)
//   - ColumnRef BETWEEN $N AND $M            (AEXPR_BETWEEN, rexpr is a List)
//
// If we encounter an A_Expr we can't interpret (e.g. ParamRef on both sides,
// ColumnRef vs non-ParamRef-wrapped-in-List), we set *outErr and return early.
func collectParamLinksFromNode(root *pg_query.Node, alias map[string]paramTarget, out map[int]paramTarget, outErr *error) {
	visit(root, func(n *pg_query.Node) {
		aexpr, ok := n.Node.(*pg_query.Node_AExpr)
		if !ok {
			return
		}
		switch aexpr.AExpr.GetKind() {
		case pg_query.A_Expr_Kind_AEXPR_OP,
			pg_query.A_Expr_Kind_AEXPR_OP_ANY,
			pg_query.A_Expr_Kind_AEXPR_OP_ALL,
			pg_query.A_Expr_Kind_AEXPR_LIKE,
			pg_query.A_Expr_Kind_AEXPR_ILIKE,
			pg_query.A_Expr_Kind_AEXPR_DISTINCT,
			pg_query.A_Expr_Kind_AEXPR_NOT_DISTINCT:
			linkSideBySide(aexpr.AExpr.GetLexpr(), aexpr.AExpr.GetRexpr(), alias, out)
		case pg_query.A_Expr_Kind_AEXPR_IN:
			linkColRefInList(aexpr.AExpr.GetLexpr(), aexpr.AExpr.GetRexpr(), alias, out)
		case pg_query.A_Expr_Kind_AEXPR_BETWEEN,
			pg_query.A_Expr_Kind_AEXPR_NOT_BETWEEN,
			pg_query.A_Expr_Kind_AEXPR_BETWEEN_SYM,
			pg_query.A_Expr_Kind_AEXPR_NOT_BETWEEN_SYM:
			linkColRefInList(aexpr.AExpr.GetLexpr(), aexpr.AExpr.GetRexpr(), alias, out)
		}
	})
}

// collectColumnRefsFromNode walks a node tree and records concrete column
// references by relation. This is used to derive INCLUDE candidates from query
// semantics instead of PostgreSQL VERBOSE scan-node targetlists, which can be
// wider than the columns actually needed by the query.
func collectColumnRefsFromNode(root *pg_query.Node, alias map[string]paramTarget, out map[string]map[string]bool) {
	visit(root, func(n *pg_query.Node) {
		col, ok := asColumnRef(n)
		if !ok {
			return
		}
		tgt, ok := resolveColumnTarget(col, alias)
		if !ok || tgt.table == "" || tgt.column == "" {
			return
		}
		addReferencedColumn(out, tgt.schema, tgt.table, tgt.column)
	})
}

func addReferencedColumn(out map[string]map[string]bool, schema, table, column string) {
	key := relationColumnKey(schema, table)
	if out[key] == nil {
		out[key] = make(map[string]bool)
	}
	out[key][column] = true
}

// relationColumnKey returns the lookup key used by referenced-columns maps.
// Identifiers are preserved as-given (parser/catalog already canonicalize case
// for unquoted names; quoted names must round-trip exactly to query the source).
func relationColumnKey(schema, table string) string {
	if schema == "" {
		return table
	}
	return schema + "." + table
}

// linkSideBySide handles expressions like "col op $N" / "$N op col".
func linkSideBySide(l, r *pg_query.Node, alias map[string]paramTarget, out map[int]paramTarget) {
	lCol, lOk := asColumnRef(l)
	rCol, rOk := asColumnRef(r)
	lParam, lIsParam := asParamRef(l)
	rParam, rIsParam := asParamRef(r)
	switch {
	case lOk && rIsParam:
		if t, ok := resolveColumnTarget(lCol, alias); ok {
			out[int(rParam.GetNumber())] = paramTarget{schema: t.schema, table: t.table, column: t.column}
		}
	case rOk && lIsParam:
		if t, ok := resolveColumnTarget(rCol, alias); ok {
			out[int(lParam.GetNumber())] = paramTarget{schema: t.schema, table: t.table, column: t.column}
		}
	}
}

// linkColRefInList handles "col IN (...)" and "col BETWEEN ...": left is a
// ColumnRef, right is a List whose elements may be ParamRef.
func linkColRefInList(l, r *pg_query.Node, alias map[string]paramTarget, out map[int]paramTarget) {
	col, ok := asColumnRef(l)
	if !ok {
		return
	}
	tgt, ok := resolveColumnTarget(col, alias)
	if !ok {
		return
	}
	listNode, ok := r.GetNode().(*pg_query.Node_List)
	if !ok {
		return
	}
	for _, item := range listNode.List.GetItems() {
		if pr, isP := asParamRef(item); isP {
			out[int(pr.GetNumber())] = tgt
		}
	}
}

// asColumnRef returns the *pg_query.ColumnRef carried by a Node (peeling any
// surrounding TypeCast wrappers), or nil/false. Casts are transparent here:
// `col::date` and `col` both refer to the same underlying column for the
// purpose of param/column linking.
func asColumnRef(n *pg_query.Node) (*pg_query.ColumnRef, bool) {
	n = unwrapTypeCast(n)
	if n == nil {
		return nil, false
	}
	c, ok := n.Node.(*pg_query.Node_ColumnRef)
	if !ok {
		return nil, false
	}
	return c.ColumnRef, true
}

// asParamRef returns the *pg_query.ParamRef carried by a Node (peeling any
// surrounding TypeCast wrappers), or nil/false. This is critical so that the
// rewritten `$N::<typename>` form (produced by normalizeTypedLiteralParams)
// still links to its sibling column the same as a bare `$N` would.
func asParamRef(n *pg_query.Node) (*pg_query.ParamRef, bool) {
	n = unwrapTypeCast(n)
	if n == nil {
		return nil, false
	}
	c, ok := n.Node.(*pg_query.Node_ParamRef)
	if !ok {
		return nil, false
	}
	return c.ParamRef, true
}

// unwrapTypeCast strips any nested TypeCast wrappers around a Node, returning
// the innermost non-cast node. Useful when the AST tag we care about (column
// reference, parameter reference) is buried under one or more `::type` casts.
func unwrapTypeCast(n *pg_query.Node) *pg_query.Node {
	for n != nil {
		tc, ok := n.Node.(*pg_query.Node_TypeCast)
		if !ok {
			return n
		}
		n = tc.TypeCast.GetArg()
	}
	return n
}

func maxParamRefNumber(root *pg_query.Node) int {
	maxParam := 0
	visit(root, func(n *pg_query.Node) {
		pr, ok := asParamRef(n)
		if !ok {
			return
		}
		if int(pr.GetNumber()) > maxParam {
			maxParam = int(pr.GetNumber())
		}
	})
	return maxParam
}

// resolveColumnTarget resolves a ColumnRef to a concrete (schema, table, column)
// using the alias map. Returns (_, false) when ambiguous or unresolvable.
func resolveColumnTarget(c *pg_query.ColumnRef, alias map[string]paramTarget) (paramTarget, bool) {
	parts := make([]string, 0, 3)
	for _, f := range c.GetFields() {
		switch x := f.Node.(type) {
		case *pg_query.Node_String_:
			parts = append(parts, x.String_.GetSval())
		case *pg_query.Node_AStar:
			return paramTarget{}, false
		}
	}
	switch len(parts) {
	case 1:
		// Unqualified column. First resolve trivially if all aliases point to a
		// single relation (covers "table AS t" where both table and alias exist in
		// the map). If multiple relations are present, disambiguate by catalog:
		// pick the only relation that actually contains this column.
		uniqueRelations := make([]paramTarget, 0, len(alias))
		seen := make(map[string]bool)
		for _, t := range alias {
			key := relationColumnKey(t.schema, t.table)
			if seen[key] {
				continue
			}
			seen[key] = true
			uniqueRelations = append(uniqueRelations, t)
		}
		if len(uniqueRelations) == 1 {
			only := uniqueRelations[0]
			return paramTarget{schema: only.schema, table: only.table, column: parts[0]}, true
		}
		var matched *paramTarget
		for _, t := range uniqueRelations {
			if !relationHasColumnFn(t, parts[0]) {
				continue
			}
			if matched != nil {
				return paramTarget{}, false
			}
			tCopy := t
			matched = &tCopy
		}
		if matched != nil {
			return paramTarget{schema: matched.schema, table: matched.table, column: parts[0]}, true
		}
		return paramTarget{}, false
	case 2:
		// qualifier.column (qualifier may be alias or table name)
		if t, ok := alias[parts[0]]; ok {
			return paramTarget{schema: t.schema, table: t.table, column: parts[1]}, true
		}
		return paramTarget{}, false
	case 3:
		// schema.table.column
		return paramTarget{schema: parts[0], table: parts[1], column: parts[2]}, true
	}
	return paramTarget{}, false
}

func relationHasColumnFromCatalog(t paramTarget, column string) bool {
	db := source.DB()
	if db == nil || t.table == "" || column == "" {
		return false
	}
	cacheKey := t.schema + "\x00" + t.table + "\x00" + column
	if v, ok := relationHasColumnCache[cacheKey]; ok {
		return v
	}

	var exists bool
	var err error
	if t.schema != "" {
		q := fmt.Sprintf(
			`SELECT EXISTS(
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = %s AND table_name = %s AND column_name = %s
			)`,
			pgQuoteLiteral(t.schema), pgQuoteLiteral(t.table), pgQuoteLiteral(column),
		)
		err = db.QueryRow(q).Scan(&exists)
	} else {
		q := fmt.Sprintf(
			`SELECT EXISTS(
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = ANY(current_schemas(true))
				  AND table_name = %s
				  AND column_name = %s
			)`,
			pgQuoteLiteral(t.table), pgQuoteLiteral(column),
		)
		err = db.QueryRow(q).Scan(&exists)
	}
	if err != nil {
		log.Debugf("covering-index: relationHasColumn lookup failed for %s.%s.%s: %v", t.schema, t.table, column, err)
		return false
	}
	relationHasColumnCache[cacheKey] = exists
	return exists
}

func pgQuoteLiteral(v string) string {
	return `'` + strings.ReplaceAll(v, `'`, `''`) + `'`
}

// visit walks a pg_query Node tree and calls visitor on every node.
// It is intentionally simple: we rely on switch over concrete Node_* kinds to
// recurse into known container fields. Unknown kinds are left to the visitor.
func visit(n *pg_query.Node, visitor func(*pg_query.Node)) {
	if n == nil {
		return
	}
	visitor(n)
	switch x := n.Node.(type) {
	case *pg_query.Node_SelectStmt:
		s := x.SelectStmt
		for _, t := range s.GetTargetList() {
			visit(t, visitor)
		}
		for _, t := range s.GetFromClause() {
			visit(t, visitor)
		}
		visit(s.GetWhereClause(), visitor)
		visit(s.GetHavingClause(), visitor)
		for _, g := range s.GetGroupClause() {
			visit(g, visitor)
		}
		for _, sc := range s.GetSortClause() {
			visit(sc, visitor)
		}
		visit(s.GetLimitOffset(), visitor)
		visit(s.GetLimitCount(), visitor)
		if l := s.GetLarg(); l != nil {
			visit(&pg_query.Node{Node: &pg_query.Node_SelectStmt{SelectStmt: l}}, visitor)
		}
		if r := s.GetRarg(); r != nil {
			visit(&pg_query.Node{Node: &pg_query.Node_SelectStmt{SelectStmt: r}}, visitor)
		}
		for _, c := range s.GetValuesLists() {
			visit(c, visitor)
		}
	case *pg_query.Node_UpdateStmt:
		u := x.UpdateStmt
		for _, t := range u.GetTargetList() {
			visit(t, visitor)
		}
		for _, t := range u.GetFromClause() {
			visit(t, visitor)
		}
		visit(u.GetWhereClause(), visitor)
	case *pg_query.Node_DeleteStmt:
		d := x.DeleteStmt
		for _, t := range d.GetUsingClause() {
			visit(t, visitor)
		}
		visit(d.GetWhereClause(), visitor)
	case *pg_query.Node_InsertStmt:
		i := x.InsertStmt
		visit(i.GetSelectStmt(), visitor)
	case *pg_query.Node_RawStmt:
		visit(x.RawStmt.GetStmt(), visitor)
	case *pg_query.Node_AExpr:
		visit(x.AExpr.GetLexpr(), visitor)
		visit(x.AExpr.GetRexpr(), visitor)
	case *pg_query.Node_BoolExpr:
		for _, a := range x.BoolExpr.GetArgs() {
			visit(a, visitor)
		}
	case *pg_query.Node_List:
		for _, it := range x.List.GetItems() {
			visit(it, visitor)
		}
	case *pg_query.Node_JoinExpr:
		visit(x.JoinExpr.GetLarg(), visitor)
		visit(x.JoinExpr.GetRarg(), visitor)
		visit(x.JoinExpr.GetQuals(), visitor)
	case *pg_query.Node_ResTarget:
		visit(x.ResTarget.GetVal(), visitor)
	case *pg_query.Node_SubLink:
		visit(x.SubLink.GetSubselect(), visitor)
	case *pg_query.Node_FuncCall:
		for _, a := range x.FuncCall.GetArgs() {
			visit(a, visitor)
		}
	case *pg_query.Node_CoalesceExpr:
		for _, a := range x.CoalesceExpr.GetArgs() {
			visit(a, visitor)
		}
	case *pg_query.Node_CaseExpr:
		for _, a := range x.CaseExpr.GetArgs() {
			visit(a, visitor)
		}
		visit(x.CaseExpr.GetArg(), visitor)
		visit(x.CaseExpr.GetDefresult(), visitor)
	case *pg_query.Node_TypeCast:
		visit(x.TypeCast.GetArg(), visitor)
	case *pg_query.Node_CommonTableExpr:
		visit(x.CommonTableExpr.GetCtequery(), visitor)
	}
}

// ---------- pg_stat_statements typed-prefix-literal rewrite ----------

// pg_stat_statements normalizes typed-prefix literals such as
//
//	timestamptz '2025-06-01'
//	interval '7 days'
//	TIMESTAMP(3) WITH TIME ZONE '2025-06-01'
//	character varying(10) 'foo'
//	pg_catalog.timestamptz '2025-06-01'
//
// into
//
//	timestamptz $N
//	interval $N
//	TIMESTAMP(3) WITH TIME ZONE $N
//	character varying(10) $N
//	pg_catalog.timestamptz $N
//
// which is invalid SQL: the type-name-prefix literal grammar requires a string
// literal after the type name, not a parameter placeholder. A naive PREPARE
// over the pgss-normalized text therefore fails with a syntax error and the
// query is silently dropped from the covering-index pipeline.
//
// normalizeTypedLiteralParams rewrites every `<typename> $N` occurrence into
// the equivalent cast `$N::<typename>`, which IS valid syntax (and parses to
// the same expression as the original typed literal once a value is bound).
// The rewrite preserves the original spelling/casing of the type name so the
// downstream report still matches what the user typed.
//
// Patterns are applied longest-first so that multi-word and modifier-bearing
// forms win over their single-word substring (e.g. `character varying $1` is
// rewritten before the bare `character $1` rule could fire).
var typedLiteralParamRewrites = []*regexp.Regexp{
	// Multi-word type names with optional precision modifier on the head word.
	regexp.MustCompile(`(?i)\b((?:national\s+)?(?:character|char)\s+varying(?:\s*\(\s*\d+\s*\))?)\s+(\$\d+)\b`),
	regexp.MustCompile(`(?i)\b(timestamp(?:\s*\(\s*\d+\s*\))?\s+with(?:out)?\s+time\s+zone)\s+(\$\d+)\b`),
	regexp.MustCompile(`(?i)\b(time(?:\s*\(\s*\d+\s*\))?\s+with(?:out)?\s+time\s+zone)\s+(\$\d+)\b`),
	regexp.MustCompile(`(?i)\b(bit\s+varying(?:\s*\(\s*\d+\s*\))?)\s+(\$\d+)\b`),
	regexp.MustCompile(`(?i)\b((?:national\s+)?(?:character|char)(?:\s*\(\s*\d+\s*\))?)\s+(\$\d+)\b`),
	regexp.MustCompile(`(?i)\b(double\s+precision)\s+(\$\d+)\b`),

	// Single-word type names (with optional pg_catalog. schema and modifier).
	// The leaf list is the closed set of built-in types whose `<type> 'literal'`
	// form pg_stat_statements is observed to normalize. New types can be added
	// here without touching call sites.
	regexp.MustCompile(`(?i)\b((?:pg_catalog\.)?(?:` +
		`bool|boolean|text|varchar|bpchar|name|` +
		`date|time|timestamp|timestamptz|timetz|interval|` +
		`numeric|decimal|money|` +
		`int|int2|int4|int8|bigint|integer|smallint|` +
		`real|float|float4|float8|` +
		`json|jsonb|xml|uuid|bytea|` +
		`inet|cidr|macaddr|macaddr8|` +
		`bit|varbit|tsquery|tsvector|` +
		`oid|regclass|regproc|regtype|regnamespace|regrole` +
		`)(?:\s*\(\s*\d+(?:\s*,\s*\d+)?\s*\))?)\s+(\$\d+)\b`),
}

// normalizeTypedLiteralParams rewrites pg_stat_statements-normalized typed
// literal prefixes into the equivalent `$N::<typename>` cast form. The
// transformation is purely textual; it never reorders parameters, so existing
// $1/$2/... numbering and the param-type lookup downstream stay correct.
// Idempotent: re-applying the transform on already-rewritten text is a no-op
// because $N::<typename> never matches `<typename> $N`.
func normalizeTypedLiteralParams(text string) string {
	for _, re := range typedLiteralParamRewrites {
		text = re.ReplaceAllString(text, "$2::$1")
	}
	return text
}

// ---------- PREPARE / parameter type discovery / DEALLOCATE ----------

// stmtCounter is used to mint unique prepared statement names per process.
var stmtCounter atomic.Uint64

// prepareStatement preps the given query on the pinned connection and returns
// the generated statement name plus the parameter_types array string from
// pg_prepared_statements. PREPARE, parameter-types lookup, and any later
// EXECUTE / DEALLOCATE must all run on the same connection so the prepared
// statement (and its row in pg_prepared_statements) is visible throughout.
func prepareStatement(ctx context.Context, conn *sql.Conn, queryText string, expectedParams int) (string, []string, error) {
	stmtName := fmt.Sprintf("voyager_covering_idx_%d", stmtCounter.Add(1))
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PREPARE %s AS %s", stmtName, queryText)); err != nil {
		return "", nil, fmt.Errorf("PREPARE: %w", err)
	}

	var rawTypes []byte
	row := conn.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT parameter_types::text FROM pg_prepared_statements WHERE name = '%s'", stmtName))
	if err := row.Scan(&rawTypes); err != nil {
		deallocateStatement(ctx, conn, stmtName)
		return "", nil, fmt.Errorf("read parameter_types: %w", err)
	}
	types := parsePostgresArray(string(rawTypes))
	if expectedParams > 0 && len(types) != expectedParams {
		// best effort: continue but log.
		log.Debugf("covering-index: param count mismatch: expected %d, got %d", expectedParams, len(types))
	}
	return stmtName, types, nil
}

func deallocateStatement(ctx context.Context, conn *sql.Conn, stmtName string) {
	if stmtName == "" {
		return
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("DEALLOCATE %s", stmtName)); err != nil {
		log.Debugf("covering-index: DEALLOCATE %s failed: %v", stmtName, err)
	}
}

// ---------- Per-column value sampling ----------

var sampleCache = map[string][]string{}

// sampleValuesForColumn samples up to N non-NULL values for (schema, table, col).
// Identifiers are taken verbatim (preserving case for quoted catalog names) and
// then quoted exactly once for execution. Results are cached for the lifetime
// of the process.
func sampleValuesForColumn(schema, table, col string) ([]string, error) {
	cacheKey := schema + "." + table + "." + col
	if v, ok := sampleCache[cacheKey]; ok {
		return v, nil
	}
	qualifiedRel := quoteIdent(table)
	if schema != "" {
		qualifiedRel = quoteIdent(schema) + "." + qualifiedRel
	}
	q := fmt.Sprintf(
		"SELECT %s::text FROM %s WHERE %s IS NOT NULL ORDER BY random() LIMIT %d",
		quoteIdent(col), qualifiedRel, quoteIdent(col),
		queryissue.COVERING_INDEX_SAMPLE_VALUES_PER_COLUMN)
	rows, err := source.DB().Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v *string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		if v != nil {
			vals = append(vals, *v)
		}
	}
	sampleCache[cacheKey] = vals
	return vals, rows.Err()
}

// quoteIdent returns a PostgreSQL-quoted identifier ("name"). If name already
// looks quoted, it is returned as-is.
func quoteIdent(name string) string {
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, `"`) && strings.HasSuffix(name, `"`) {
		return name
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// ---------- Value tuples ----------

// buildValueTuples generates the concrete list of value tuples to EXPLAIN.
// If the full cross-product of samples fits within COVERING_INDEX_MAX_EXPLAIN_PER_QUERY,
// we emit the cross-product; otherwise we fall back to a positional strategy where
// tuple i takes valuesPerParam[p][i % len(valuesPerParam[p])].
func buildValueTuples(valuesPerParam [][]string) [][]string {
	if len(valuesPerParam) == 0 {
		return nil
	}
	// Compute cross-product size.
	total := 1
	for _, vs := range valuesPerParam {
		if len(vs) == 0 {
			return nil
		}
		total *= len(vs)
		if total > queryissue.COVERING_INDEX_MAX_EXPLAIN_PER_QUERY {
			break
		}
	}
	if total <= queryissue.COVERING_INDEX_MAX_EXPLAIN_PER_QUERY {
		return crossProduct(valuesPerParam)
	}
	// Positional fallback.
	maxLen := 0
	for _, vs := range valuesPerParam {
		if len(vs) > maxLen {
			maxLen = len(vs)
		}
	}
	out := make([][]string, 0, queryissue.COVERING_INDEX_MAX_EXPLAIN_PER_QUERY)
	for i := 0; i < queryissue.COVERING_INDEX_MAX_EXPLAIN_PER_QUERY; i++ {
		tuple := make([]string, len(valuesPerParam))
		for p, vs := range valuesPerParam {
			tuple[p] = vs[i%len(vs)]
		}
		out = append(out, tuple)
	}
	return out
}

func crossProduct(in [][]string) [][]string {
	if len(in) == 0 {
		return nil
	}
	out := [][]string{{}}
	for _, col := range in {
		next := make([][]string, 0, len(out)*len(col))
		for _, prefix := range out {
			for _, v := range col {
				cp := make([]string, len(prefix)+1)
				copy(cp, prefix)
				cp[len(prefix)] = v
				next = append(next, cp)
			}
		}
		out = next
	}
	return out
}

func defaultParamValueForType(paramType string) string {
	switch strings.ToLower(paramType) {
	case "boolean", "bool":
		return "false"
	case "date":
		return "2024-01-01"
	case "timestamp", "timestamp without time zone", "timestamp with time zone", "timestamptz":
		return "2024-01-01 00:00:00"
	case "uuid":
		return "00000000-0000-0000-0000-000000000001"
	default:
		return "1"
	}
}

// ---------- EXPLAIN EXECUTE ----------

// runExplainExecute runs EXPLAIN (FORMAT JSON) EXECUTE stmtName(val1, val2, ...)
// on the pinned connection (same one PREPARE ran on). Returns the concatenated
// JSON output bytes.
func runExplainExecute(ctx context.Context, conn *sql.Conn, stmtName string, values []string, paramTypes []string) ([]byte, error) {
	args := make([]string, len(values))
	for i, v := range values {
		// nullParamSentinel asks for an unquoted SQL NULL at this slot
		// (used by partial-index `col IS NULL` sampling).
		if v == nullParamSentinel {
			args[i] = "NULL"
			continue
		}
		literal := `'` + strings.ReplaceAll(v, `'`, `''`) + `'`
		if i < len(paramTypes) && paramTypes[i] != "" {
			literal += "::" + paramTypes[i]
		}
		args[i] = literal
	}
	explainQuery := fmt.Sprintf("EXPLAIN (FORMAT JSON) EXECUTE %s(%s)",
		stmtName, strings.Join(args, ", "))
	rows, err := conn.QueryContext(ctx, explainQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		parts = append(parts, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return []byte(strings.Join(parts, "\n")), nil
}

// ---------- Partial-index predicate parsing ----------

// extractPartialPredicateConstraints parses the deparsed predicate text of a
// partial index (as returned by pg_get_expr on pg_index.indpred) and extracts
// simple per-column equality / IS NULL constraints that downstream sampling
// can use to synthesize EXPLAIN EXECUTE tuples that satisfy the predicate.
//
// Supported clauses (recursively under AND of any combination):
//   - col = 'lit' / 'lit' = col
//   - col IN ('a', 'b', ...)              (AEXPR_IN; rexpr is a List)
//   - col = ANY (ARRAY['a', 'b', ...])    (AEXPR_OP_ANY; rexpr is A_ArrayExpr;
//     this is the form pg_get_expr emits for an IN list because the catalog
//     stores it as ScalarArrayOpExpr).
//   - col IS NULL
//
// Anything else (functions, ranges, OR, NOT-equality, IS NOT NULL, expressions)
// is silently skipped — the partial recommendation will only succeed if the
// random sample happens to satisfy the predicate organically.
//
// Map keys are "schema.table.col" (identifiers preserved verbatim, no
// lowercasing) so callers can merge directly into the per-relation maps.
func extractPartialPredicateConstraints(predText, schema, table string) (eqValues map[string][]string, isNull map[string]bool) {
	eqValues = map[string][]string{}
	isNull = map[string]bool{}
	if strings.TrimSpace(predText) == "" {
		return eqValues, isNull
	}
	parseTree, err := queryparser.Parse("SELECT 1 WHERE " + predText)
	if err != nil || parseTree == nil || len(parseTree.Stmts) == 0 {
		return eqValues, isNull
	}
	stmt := parseTree.Stmts[0].GetStmt()
	if stmt == nil {
		return eqValues, isNull
	}
	sel, ok := stmt.Node.(*pg_query.Node_SelectStmt)
	if !ok {
		return eqValues, isNull
	}
	walkPartialPredicate(sel.SelectStmt.GetWhereClause(), schema, table, eqValues, isNull)
	return eqValues, isNull
}

// walkPartialPredicate recursively walks a predicate AST, descending only
// through AND nodes (the conservative case where every conjunct must hold)
// and harvesting supported leaf clauses.
func walkPartialPredicate(node *pg_query.Node, schema, table string, eqVals map[string][]string, isNull map[string]bool) {
	if node == nil {
		return
	}
	switch n := node.Node.(type) {
	case *pg_query.Node_BoolExpr:
		if n.BoolExpr.GetBoolop() != pg_query.BoolExprType_AND_EXPR {
			return
		}
		for _, arg := range n.BoolExpr.GetArgs() {
			walkPartialPredicate(arg, schema, table, eqVals, isNull)
		}
	case *pg_query.Node_NullTest:
		if n.NullTest.GetNulltesttype() != pg_query.NullTestType_IS_NULL {
			return
		}
		col, ok := asColumnRef(n.NullTest.GetArg())
		if !ok {
			return
		}
		if cn := lastColumnName(col); cn != "" {
			isNull[schema+"."+table+"."+cn] = true
		}
	case *pg_query.Node_AExpr:
		harvestAExprPartial(n.AExpr, schema, table, eqVals)
	}
}

// harvestAExprPartial extracts column-equality constraints from an A_Expr
// representing one of the supported clause shapes (`=`, `IN`, `= ANY`).
func harvestAExprPartial(a *pg_query.A_Expr, schema, table string, eqVals map[string][]string) {
	if opName(a.GetName()) != "=" {
		return
	}
	switch a.GetKind() {
	case pg_query.A_Expr_Kind_AEXPR_OP:
		if col, ok := asColumnRef(a.GetLexpr()); ok {
			if v, ok := constLiteralValue(a.GetRexpr()); ok {
				appendUnique(eqVals, schema+"."+table+"."+lastColumnName(col), v)
				return
			}
		}
		if col, ok := asColumnRef(a.GetRexpr()); ok {
			if v, ok := constLiteralValue(a.GetLexpr()); ok {
				appendUnique(eqVals, schema+"."+table+"."+lastColumnName(col), v)
			}
		}
	case pg_query.A_Expr_Kind_AEXPR_IN:
		col, ok := asColumnRef(a.GetLexpr())
		if !ok {
			return
		}
		list, ok := a.GetRexpr().GetNode().(*pg_query.Node_List)
		if !ok {
			return
		}
		key := schema + "." + table + "." + lastColumnName(col)
		for _, item := range list.List.GetItems() {
			if v, ok := constLiteralValue(item); ok {
				appendUnique(eqVals, key, v)
			}
		}
	case pg_query.A_Expr_Kind_AEXPR_OP_ANY:
		col, ok := asColumnRef(a.GetLexpr())
		if !ok {
			return
		}
		rexpr := unwrapTypeCast(a.GetRexpr())
		if rexpr == nil {
			return
		}
		arr, ok := rexpr.Node.(*pg_query.Node_AArrayExpr)
		if !ok {
			return
		}
		key := schema + "." + table + "." + lastColumnName(col)
		for _, el := range arr.AArrayExpr.GetElements() {
			if v, ok := constLiteralValue(el); ok {
				appendUnique(eqVals, key, v)
			}
		}
	}
}

// opName returns the operator string from an A_Expr.Name list (typically a
// single String_ node like `=`).
func opName(names []*pg_query.Node) string {
	if len(names) == 0 {
		return ""
	}
	last := names[len(names)-1]
	if s := last.GetString_(); s != nil {
		return s.GetSval()
	}
	return ""
}

// lastColumnName returns the last field name of a ColumnRef (i.e. the column
// itself), preserving its original case (no lowercasing of quoted spellings).
func lastColumnName(c *pg_query.ColumnRef) string {
	fields := c.GetFields()
	if len(fields) == 0 {
		return ""
	}
	last := fields[len(fields)-1]
	if s := last.GetString_(); s != nil {
		return s.GetSval()
	}
	return ""
}

// constLiteralValue extracts a string-formatted scalar value from an A_Const
// node (peeling any TypeCast wrappers). Returns ("", false) for non-literals,
// NULLs, or unsupported value kinds. Output is the raw value suitable for
// binding via EXPLAIN EXECUTE — the runExplainExecute path will quote/cast it
// the same way as random sample values.
func constLiteralValue(n *pg_query.Node) (string, bool) {
	n = unwrapTypeCast(n)
	if n == nil {
		return "", false
	}
	a := n.GetAConst()
	if a == nil || a.GetIsnull() {
		return "", false
	}
	switch {
	case a.GetSval() != nil:
		return a.GetSval().GetSval(), true
	case a.GetIval() != nil:
		return strconv.FormatInt(int64(a.GetIval().GetIval()), 10), true
	case a.GetFval() != nil:
		return a.GetFval().GetFval(), true
	case a.GetBoolval() != nil:
		if a.GetBoolval().GetBoolval() {
			return "true", true
		}
		return "false", true
	}
	return "", false
}

func appendUnique(m map[string][]string, key, val string) {
	for _, existing := range m[key] {
		if existing == val {
			return
		}
	}
	m[key] = append(m[key], val)
}

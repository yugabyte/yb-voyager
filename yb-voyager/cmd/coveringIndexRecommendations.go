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

// Per-index covering-index recommendations via parameter-substitution EXPLAIN.
//
// Pipeline:
//  1. Load index metadata (key columns + table + data types + column widths).
//  2. Select high-impact pg_stat_statements rows (by total_exec_time, min calls floor).
//  3. For each query: link $N -> (schema, table, column) via AST; collect referenced
//     columns per relation from the AST; PREPARE; sample real values; build value
//     tuples; EXPLAIN (FORMAT JSON) EXECUTE each tuple; walk plan for Index Scan
//     nodes; aggregate missing = AST-referenced columns for scan relation - index keys.
//  4. Ratio-filter missing columns per index (drop write-heavy via AST workload).
//  5. Emit one queryissue.CoveringIndexRecommendation per surviving index with
//     all admitted columns (INCLUDE columns sorted alphabetically).

package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// ---------- Data model ----------

// indexKey identifies a single source index: {schema, name} in lower-cased, unquoted form.
type indexKey struct {
	schema string
	name   string
}

func (k indexKey) String() string { return k.schema + "." + k.name }

// contributingQuery is a high-impact pg_stat_statements entry that contributed
// to a column being a candidate for INCLUDE on an index.
type contributingQuery struct {
	text        string
	calls       int64
	totalExecMs float64
}

// colUsage aggregates a single candidate include-column for a single index
// across all queries whose EXPLAIN plan produced an Index Scan on that index
// with this column referenced by the query for the scanned relation.
type colUsage struct {
	column       string
	contributing []contributingQuery
	totalCalls   int64
	totalExecMs  float64
	avgWidth     int
}

// indexCandidate is the pre-filter, pre-cap per-index aggregate of candidate
// INCLUDE columns and their usage across the analyzed workload.
type indexCandidate struct {
	key         indexKey
	schema      string
	table       string // unqualified table name
	keyColumns  []string
	missingCols map[string]*colUsage // keyed by column name (lowercase)
	droppedList []droppedColumn      // drops recorded during ratio filter (pre-cap)
}

// droppedColumn mirrors queryissue.DroppedColumn for staging during the pipeline.
type droppedColumn struct {
	name   string
	reason string // "write_heavy" (currently the only reason produced)
}

// ---------- Index metadata ----------

// indexMeta holds per-index key columns + per-column widths, populated once
// at the start of the assessment while the source connection is live.
type indexMeta struct {
	keyColumns     map[indexKey][]string // index -> key cols (ordered)
	indexTable     map[indexKey]struct{ schema, table string }
	columnAvgWidth map[string]int    // "schema.table.col" -> avg width bytes
	columnDataType map[string]string // "schema.table.col" -> dtype

	// Partial-index predicate constraints, keyed by "schema.table.col".
	// partialEqValues maps a column to literal values that satisfy a
	// `col = 'lit'` / `col IN (...)` clause inside any partial index's
	// predicate. partialIsNull marks columns required to be NULL by some
	// partial index's `col IS NULL` clause. analyzeOneQuery prepends these
	// when building EXPLAIN EXECUTE tuples so the planner is able to
	// consider partial indexes whose predicate references a parameterized
	// query column (random sampling alone tends to miss rare values).
	partialEqValues map[string][]string
	partialIsNull   map[string]bool

	// parentIndex maps a partition child index to its IMMEDIATE parent index
	// (populated from pg_inherits). Multi-level partitioning produces a
	// chain leaf -> mid -> root; use rootIndex() to walk it. The walker uses
	// this to fold per-partition scan-node matches up to the parent index
	// that the source schema's CREATE INDEX statement actually declares,
	// so the report can attach the recommendation to a real DDL row.
	parentIndex map[indexKey]indexKey
}

func loadIndexMeta() (*indexMeta, error) {
	m := &indexMeta{
		keyColumns:      make(map[indexKey][]string),
		indexTable:      make(map[indexKey]struct{ schema, table string }),
		columnAvgWidth:  make(map[string]int),
		columnDataType:  make(map[string]string),
		partialEqValues: make(map[string][]string),
		partialIsNull:   make(map[string]bool),
		parentIndex:     make(map[indexKey]indexKey),
	}

	// MAX(pg_get_expr(...)) is used because pg_get_expr's output is a string
	// expression, constant per index, but we need to surface it through the
	// GROUP BY (the predicate column is per-row pre-aggregation).
	idxQuery := `
		SELECT
			n.nspname AS schema_name,
			t.relname AS table_name,
			i.relname AS index_name,
			array_agg(a.attname ORDER BY k.ordinality) AS key_columns,
			MAX(pg_get_expr(ix.indpred, ix.indrelid)) AS partial_pred
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		CROSS JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ordinality)
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
		WHERE ix.indisvalid AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		GROUP BY n.nspname, t.relname, i.relname`
	rows, err := source.DB().Query(idxQuery)
	if err != nil {
		return nil, fmt.Errorf("query index metadata: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var schemaName, tableName, indexName string
		var keyColsRaw []byte
		var partialPred sql.NullString
		if err := rows.Scan(&schemaName, &tableName, &indexName, &keyColsRaw, &partialPred); err != nil {
			return nil, fmt.Errorf("scan index metadata: %w", err)
		}
		k := indexKey{schema: schemaName, name: indexName}
		m.keyColumns[k] = parsePostgresArray(string(keyColsRaw))
		m.indexTable[k] = struct{ schema, table string }{schemaName, tableName}

		if partialPred.Valid && strings.TrimSpace(partialPred.String) != "" {
			eqVals, isNullCols := extractPartialPredicateConstraints(partialPred.String, schemaName, tableName)
			for col, vals := range eqVals {
				for _, v := range vals {
					appendUnique(m.partialEqValues, col, v)
				}
			}
			for col := range isNullCols {
				m.partialIsNull[col] = true
			}
			if len(eqVals) > 0 || len(isNullCols) > 0 {
				log.Debugf("covering-index: partial-predicate constraints for %s.%s: eq=%v isnull=%v",
					schemaName, indexName, eqVals, isNullCols)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	widthQuery := `SELECT schemaname, tablename, attname, avg_width FROM pg_stats
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')`
	wrows, err := source.DB().Query(widthQuery)
	if err != nil {
		log.Warnf("covering-index: failed to read pg_stats.avg_width: %v", err)
	} else {
		defer wrows.Close()
		for wrows.Next() {
			var sch, tab, col string
			var avgW int
			if err := wrows.Scan(&sch, &tab, &col, &avgW); err != nil {
				continue
			}
			m.columnAvgWidth[sch+"."+tab+"."+col] = avgW
		}
	}

	dtypeQuery := fmt.Sprintf("SELECT schema_name, table_name, column_name, data_type FROM %s WHERE source_node = 'primary'",
		migassessment.TABLE_COLUMNS_DATA_TYPES)
	drows, err := assessmentDB.Query(dtypeQuery)
	if err == nil {
		defer drows.Close()
		for drows.Next() {
			var sch, tab, col, dtype string
			if err := drows.Scan(&sch, &tab, &col, &dtype); err != nil {
				continue
			}
			m.columnDataType[sch+"."+tab+"."+col] = dtype
		}
	} else {
		log.Warnf("covering-index: failed to read %s from assessment db: %v", migassessment.TABLE_COLUMNS_DATA_TYPES, err)
	}

	// Partition child -> parent index links. PG >= 11 records this in
	// pg_inherits for index relations the same way it does for tables: each
	// per-partition index has an inhrelid row pointing at its parent index
	// (inhparent). This lets walkNode collapse per-partition scan-node
	// matches up to the parent index that the source schema's CREATE INDEX
	// actually defines. Failure here is non-fatal: we just log and continue
	// without partition-aware resolution.
	parentQuery := `
		SELECT cn.nspname AS child_schema,  ci.relname AS child_name,
		       pn.nspname AS parent_schema, pi.relname AS parent_name
		FROM pg_inherits inh
		JOIN pg_class ci      ON ci.oid = inh.inhrelid
		JOIN pg_class pi      ON pi.oid = inh.inhparent
		JOIN pg_namespace cn  ON cn.oid = ci.relnamespace
		JOIN pg_namespace pn  ON pn.oid = pi.relnamespace
		WHERE ci.relkind IN ('i', 'I') AND pi.relkind IN ('i', 'I')
		  AND cn.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND pn.nspname NOT IN ('pg_catalog', 'information_schema')`
	prows, err := source.DB().Query(parentQuery)
	if err != nil {
		log.Warnf("covering-index: failed to read pg_inherits for partition index links: %v", err)
	} else {
		defer prows.Close()
		for prows.Next() {
			var childSchema, childName, parentSchema, parentName string
			if err := prows.Scan(&childSchema, &childName, &parentSchema, &parentName); err != nil {
				continue
			}
			m.parentIndex[indexKey{schema: childSchema, name: childName}] =
				indexKey{schema: parentSchema, name: parentName}
		}
	}

	return m, nil
}

// rootIndex resolves a partition child index key up to its root parent index
// key by walking the parentIndex chain. Returns the input unchanged if the
// key has no parent recorded (i.e. it is already a root index, or it is a
// non-partitioned standalone index).
//
// The walk is bounded (maxPartitionDepth steps) and cycle-protected: in the
// pathological case where pg_inherits points back at an already-visited
// node, we log a warning once and return the original key so the walker
// still emits a candidate (worst case: keyed at the per-partition child,
// matching pre-fix behavior).
func (m *indexMeta) rootIndex(k indexKey) indexKey {
	const maxPartitionDepth = 16
	if m == nil || len(m.parentIndex) == 0 {
		return k
	}
	seen := map[indexKey]bool{k: true}
	cur := k
	for i := 0; i < maxPartitionDepth; i++ {
		p, ok := m.parentIndex[cur]
		if !ok {
			return cur
		}
		if seen[p] {
			log.Warnf("covering-index: cycle in parentIndex chain starting at %s.%s; using input key",
				k.schema, k.name)
			return k
		}
		seen[p] = true
		cur = p
	}
	return cur
}

// estimateAvgWidth returns a conservative byte-width estimate for a column given
// only its data type, used as a fallback when pg_stats.avg_width is unavailable.
func estimateAvgWidth(dtype string) int {
	dtype = strings.ToLower(strings.TrimSpace(dtype))
	switch {
	case strings.HasPrefix(dtype, "bool"):
		return 1
	case strings.HasPrefix(dtype, "smallint"), strings.HasPrefix(dtype, "int2"):
		return 2
	case strings.HasPrefix(dtype, "integer"), strings.HasPrefix(dtype, "int4"):
		return 4
	case strings.HasPrefix(dtype, "bigint"), strings.HasPrefix(dtype, "int8"),
		strings.HasPrefix(dtype, "double"), strings.HasPrefix(dtype, "float8"),
		strings.HasPrefix(dtype, "timestamp"), strings.HasPrefix(dtype, "date"),
		strings.HasPrefix(dtype, "time"):
		return 8
	case strings.HasPrefix(dtype, "uuid"):
		return 16
	case strings.HasPrefix(dtype, "numeric"), strings.HasPrefix(dtype, "decimal"):
		return 16
	case strings.HasPrefix(dtype, "text"), strings.HasPrefix(dtype, "varchar"),
		strings.HasPrefix(dtype, "character varying"), strings.HasPrefix(dtype, "char"):
		return 32
	case strings.HasPrefix(dtype, "json"):
		return 64
	}
	return 32
}

func (m *indexMeta) widthFor(schema, table, col string) int {
	key := schema + "." + table + "." + col
	if w, ok := m.columnAvgWidth[key]; ok && w > 0 {
		return w
	}
	return estimateAvgWidth(m.columnDataType[key])
}

// lookupPartialConstraints returns the partial-predicate eq-values and IS-NULL
// flag for (schema, table, col). The maps are keyed by "schema.table.col";
// when a query does not schema-qualify its relation, the param-target's schema
// is "", so we fall back to suffix-matching on ".<table>.<col>" — if exactly
// one schema in the metadata satisfies the suffix we use it. Ambiguous
// matches (same table+col across multiple schemas with different constraints)
// are conservatively skipped.
func (m *indexMeta) lookupPartialConstraints(schema, table, col string) (eqValues []string, isNull bool) {
	if schema != "" {
		key := schema + "." + table + "." + col
		return m.partialEqValues[key], m.partialIsNull[key]
	}
	suffix := "." + table + "." + col
	var matchKey string
	consider := func(k string) bool {
		if !strings.HasSuffix(k, suffix) {
			return true
		}
		if matchKey == "" {
			matchKey = k
			return true
		}
		if matchKey != k {
			matchKey = ""
			return false
		}
		return true
	}
	for k := range m.partialEqValues {
		if !consider(k) {
			return nil, false
		}
	}
	for k := range m.partialIsNull {
		if !consider(k) {
			return nil, false
		}
	}
	if matchKey == "" {
		return nil, false
	}
	return m.partialEqValues[matchKey], m.partialIsNull[matchKey]
}

func parsePostgresArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"`)
		out = append(out, p)
	}
	return out
}

// ---------- High-impact query selection ----------

// highImpactQuery carries the pg_stat_statements row data we need end-to-end.
type highImpactQuery struct {
	text        string
	calls       int64
	totalExecMs float64
}

func fetchHighImpactQueries() ([]highImpactQuery, error) {
	q := fmt.Sprintf(`SELECT query, calls, total_exec_time
		FROM pg_stat_statements
		WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		  AND calls >= %d
		ORDER BY total_exec_time DESC
		LIMIT %d`,
		queryissue.COVERING_INDEX_MIN_CALLS, queryissue.COVERING_INDEX_MAX_QUERIES_ANALYZED)
	rows, err := source.DB().Query(q)
	if err != nil {
		return nil, fmt.Errorf("query pg_stat_statements: %w", err)
	}
	defer rows.Close()
	var out []highImpactQuery
	for rows.Next() {
		var h highImpactQuery
		if err := rows.Scan(&h.text, &h.calls, &h.totalExecMs); err != nil {
			return nil, err
		}
		if !isReadOnlySelectQuery(h.text) {
			continue
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func isReadOnlySelectQuery(queryText string) bool {
	parseTree, err := queryparser.Parse(queryText)
	if err != nil {
		return false
	}
	if parseTree == nil || len(parseTree.Stmts) == 0 {
		return false
	}
	for _, stmt := range parseTree.Stmts {
		if stmt == nil || stmt.Stmt == nil || stmt.Stmt.GetSelectStmt() == nil {
			return false
		}
	}
	return true
}

// ---------- Parameter -> column linking ----------

// paramTarget identifies the column that a $N parameter refers to.
type paramTarget struct {
	schema string // may be "" if query didn't qualify
	table  string // unqualified
	column string
}

// linkParamsToColumns returns a map paramIndex(1-based) -> paramTarget for every
// ParamRef found in comparisons against a ColumnRef. If any ParamRef appears in
// a context we cannot resolve, we return an error so the caller can skip the query
// rather than substituting wrong values.
func linkParamsToColumns(queryText string) (map[int]paramTarget, error) {
	parseTree, err := queryparser.Parse(queryText)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if parseTree == nil || len(parseTree.Stmts) == 0 {
		return nil, fmt.Errorf("empty parse tree")
	}

	// Build alias -> (schema, table) map using existing traversal utilities.
	aliasMap := buildAliasMapFromStmts(parseTree)

	out := make(map[int]paramTarget)
	var walkErr error

	// Walk each top-level statement and scan A_Expr nodes for ColumnRef ↔ ParamRef pairs.
	for _, stmt := range parseTree.Stmts {
		if stmt == nil || stmt.Stmt == nil {
			continue
		}
		// Walk the entire statement sub-tree looking for A_Expr nodes.
		collectParamLinksFromNode(stmt.Stmt, aliasMap, out, &walkErr)
	}
	if walkErr != nil {
		return nil, walkErr
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no param/column links found")
	}
	return out, nil
}

func countQueryParams(queryText string) (int, error) {
	parseTree, err := queryparser.Parse(queryText)
	if err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}
	if parseTree == nil || len(parseTree.Stmts) == 0 {
		return 0, fmt.Errorf("empty parse tree")
	}
	maxParam := 0
	for _, stmt := range parseTree.Stmts {
		if stmt == nil || stmt.Stmt == nil {
			continue
		}
		if n := maxParamRefNumber(stmt.Stmt); n > maxParam {
			maxParam = n
		}
	}
	return maxParam, nil
}

// collectReferencedColumnsForQuery returns lower-cased referenced columns keyed by
// relation ("schema.table" when schema is present, otherwise "table"). The map is
// intentionally query/AST-derived: EXPLAIN identifies the index, while this map
// decides which table columns are semantically needed for INCLUDE candidates.
func collectReferencedColumnsForQuery(queryText string) (map[string]map[string]bool, error) {
	parseTree, err := queryparser.Parse(queryText)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if parseTree == nil || len(parseTree.Stmts) == 0 {
		return nil, fmt.Errorf("empty parse tree")
	}
	aliasMap := buildAliasMapFromStmts(parseTree)
	out := make(map[string]map[string]bool)
	for _, stmt := range parseTree.Stmts {
		if stmt == nil || stmt.Stmt == nil {
			continue
		}
		collectColumnRefsFromNode(stmt.Stmt, aliasMap, out)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no referenced columns found")
	}
	return out, nil
}

// buildAliasMapFromStmts walks all RangeVar nodes in the parse tree and maps
// both alias names and real table names to their (schema, table). If a name
// is ambiguous (different tables use it in the same query), we map it to the
// empty sentinel struct which signals "ambiguous".
func buildAliasMapFromStmts(parseTree *pg_query.ParseResult) map[string]paramTarget {
	// Use proto reflection to walk the tree: we reuse the queryparser traversal
	// so we don't have to duplicate pg_query type switching here.
	am := make(map[string]paramTarget)
	ambiguous := make(map[string]bool)

	for _, s := range parseTree.Stmts {
		if s == nil || s.Stmt == nil {
			continue
		}
		collectRangeVars(s.Stmt, am, ambiguous)
	}
	for name := range ambiguous {
		delete(am, name)
	}
	return am
}

// ---------- Compute / filter / caps (high-level) ----------

// computeExplainBasedIncludeColumns is the top-level per-query driver. It returns
// per-index candidate aggregates (missing-columns + contributing queries) before
// any filtering or capping.
func computeExplainBasedIncludeColumns() map[indexKey]*indexCandidate {
	meta, err := loadIndexMeta()
	if err != nil {
		log.Warnf("covering-index: failed to load index metadata: %v", err)
		return nil
	}
	if len(meta.keyColumns) == 0 {
		log.Info("covering-index: no indexes found in source schemas, skipping analysis")
		return nil
	}
	log.Infof("covering-index: loaded metadata for %d indexes", len(meta.keyColumns))

	queries, err := fetchHighImpactQueries()
	if err != nil {
		log.Warnf("covering-index: failed to fetch high-impact queries: %v", err)
		return nil
	}
	if len(queries) == 0 {
		log.Info("covering-index: no high-impact queries in pg_stat_statements, skipping analysis")
		return nil
	}
	log.Infof("covering-index: fetched %d high-impact queries from pg_stat_statements", len(queries))

	// Pin one connection for the entire analysis. PREPARE, parameter-type
	// lookup, EXPLAIN EXECUTE, and DEALLOCATE must all run on the same backend
	// so the prepared statement stays visible. Running them against the pool
	// would hand each call to an arbitrary backend and silently fail.
	ctx := context.Background()
	pgSource, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		log.Warnf("covering-index: source db is not PostgreSQL (%T); skipping analysis", source.DB())
		return nil
	}
	conn, err := pgSource.Conn(ctx)
	if err != nil {
		log.Warnf("covering-index: failed to acquire source connection: %v", err)
		return nil
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SET plan_cache_mode = force_custom_plan"); err != nil {
		log.Warnf("covering-index: could not SET plan_cache_mode: %v (continuing)", err)
	}

	perIdx := make(map[indexKey]*indexCandidate)
	for _, q := range queries {
		analyzeOneQuery(ctx, conn, q, meta, perIdx)
	}
	log.Infof("covering-index: built %d per-index candidates after EXPLAIN walk: %s", len(perIdx), formatIndexKeys(perIdx))
	return perIdx
}

// formatIndexKeys renders the indexKey set as "schema.name, schema.name, ..."
// for log lines. Sorted to keep ordering stable across runs.
func formatIndexKeys[V any](m map[indexKey]V) string {
	if len(m) == 0 {
		return "[]"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k.schema+"."+k.name)
	}
	sort.Strings(keys)
	return "[" + strings.Join(keys, ", ") + "]"
}

// analyzeOneQuery links params, prepares, samples, runs EXPLAIN for each tuple,
// and updates perIdx with missing-column contributions. Per-query failures are
// logged and skipped; they don't abort the pass.
func analyzeOneQuery(ctx context.Context, conn *sql.Conn, q highImpactQuery, meta *indexMeta, perIdx map[indexKey]*indexCandidate) {
	// pg_stat_statements normalizes typed-prefix literals (e.g. `timestamptz
	// '2025-06-01'`) into invalid syntax (`timestamptz $N`). Rewrite to the
	// equivalent cast form before parsing/PREPARE so applications using these
	// idiomatic literal forms aren't silently dropped from the pipeline. The
	// original q.text is kept untouched so it appears verbatim in the report.
	queryText := normalizeTypedLiteralParams(q.text)

	links, err := linkParamsToColumns(queryText)
	if err != nil {
		log.Debugf("covering-index: skip %q: %v", truncateQuery(q.text, 80), err)
		return
	}
	referencedCols, err := collectReferencedColumnsForQuery(queryText)
	if err != nil {
		log.Debugf("covering-index: skip %q: %v", truncateQuery(q.text, 80), err)
		return
	}
	paramCount, err := countQueryParams(queryText)
	if err != nil {
		log.Debugf("covering-index: skip %q: %v", truncateQuery(q.text, 80), err)
		return
	}

	stmtName, paramTypes, err := prepareStatement(ctx, conn, queryText, paramCount)
	if err != nil {
		log.Debugf("covering-index: PREPARE failed for %q: %v", truncateQuery(q.text, 80), err)
		return
	}
	defer deallocateStatement(ctx, conn, stmtName)

	if len(paramTypes) > paramCount {
		paramCount = len(paramTypes)
	}
	valuesPerParam := make([][]string, paramCount)
	for i := 1; i <= paramCount; i++ {
		tgt, ok := links[i]
		if !ok {
			paramType := ""
			if i-1 < len(paramTypes) {
				paramType = paramTypes[i-1]
			}
			valuesPerParam[i-1] = []string{defaultParamValueForType(paramType)}
			continue
		}
		vals, err := sampleValuesForColumn(tgt.schema, tgt.table, tgt.column)
		if err != nil || len(vals) == 0 {
			log.Debugf("covering-index: no samples for %s.%s.%s: %v", tgt.schema, tgt.table, tgt.column, err)
			return
		}
		valuesPerParam[i-1] = augmentSamplesWithPartialConstraints(meta, tgt, vals)
	}

	tuples := buildValueTuples(valuesPerParam)
	for _, tuple := range tuples {
		planJSON, err := runExplainExecute(ctx, conn, stmtName, tuple, paramTypes)
		if err != nil {
			log.Debugf("covering-index: EXPLAIN EXECUTE failed: %v", err)
			continue
		}
		walkPlanForIndexScans(planJSON, meta, perIdx, q, referencedCols)
	}
}

// augmentSamplesWithPartialConstraints prepends partial-index-satisfying values
// in front of the random sample list for (schema, table, column). Without this
// step, parameters whose dominant random value never matches a partial index's
// predicate would silently miss the partial-index recommendation.
//
// Order: literal-equality values first (preserving the order they were
// extracted), then random samples; if any partial index requires `col IS
// NULL`, prepend the NULL sentinel so EXPLAIN EXECUTE binds an unquoted NULL.
// Duplicates against the existing sample are dropped to keep the cross-product
// bounded.
func augmentSamplesWithPartialConstraints(meta *indexMeta, tgt paramTarget, vals []string) []string {
	if meta == nil {
		return vals
	}
	extras, wantNull := meta.lookupPartialConstraints(tgt.schema, tgt.table, tgt.column)
	if len(extras) == 0 && !wantNull {
		return vals
	}
	key := tgt.table + "." + tgt.column
	if tgt.schema != "" {
		key = tgt.schema + "." + key
	}
	seen := make(map[string]bool, len(vals)+len(extras)+1)
	out := make([]string, 0, len(vals)+len(extras)+1)
	if wantNull {
		seen[nullParamSentinel] = true
		out = append(out, nullParamSentinel)
	}
	for _, v := range extras {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	for _, v := range vals {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	log.Infof("covering-index: augmented samples for %s with partial-predicate constraints (eq=%v, is_null=%v)",
		key, extras, wantNull)
	return out
}

// filterRedundantIndexCandidates drops candidates whose underlying index has
// already been classified as REDUNDANT by the schema-assessment redundant-index
// detection. We don't want to recommend INCLUDE columns for an index the user
// is going to drop anyway. Fails open: on error reading the redundant-indexes
// table, returns the input unchanged.
func filterRedundantIndexCandidates(in map[indexKey]*indexCandidate) map[indexKey]*indexCandidate {
	if len(in) == 0 {
		return nil
	}
	redundant, err := fetchRedundantIndexInfoFromAssessmentDB()
	if err != nil {
		log.Warnf("covering-index: failed to fetch redundant indexes; skipping redundant filter: %v", err)
		return in
	}
	return applyRedundantFilterToCandidates(in, redundant)
}

// applyRedundantFilterToCandidates is the pure helper exposed for unit tests.
// It removes any candidate whose lower-cased (schema, name) matches a row in
// redundant. Names from redundant are lower-cased before comparison since
// candidate keys are already lower-cased.
func applyRedundantFilterToCandidates(in map[indexKey]*indexCandidate, redundant []utils.RedundantIndexesInfo) map[indexKey]*indexCandidate {
	if len(in) == 0 {
		return nil
	}
	if len(redundant) == 0 {
		return in
	}
	redundantKeys := lo.SliceToMap(redundant, func(r utils.RedundantIndexesInfo) (indexKey, struct{}) {
		return indexKey{
			schema: strings.ToLower(r.RedundantSchemaName),
			name:   strings.ToLower(r.RedundantIndexName),
		}, struct{}{}
	})
	out := make(map[indexKey]*indexCandidate, len(in))
	dropped := make(map[indexKey]*indexCandidate)
	for k, cand := range in {
		if _, isRedundant := redundantKeys[k]; isRedundant {
			dropped[k] = cand
			continue
		}
		out[k] = cand
	}
	if len(dropped) > 0 {
		log.Infof("covering-index: dropped %d candidate(s) classified as redundant by assessment: %s",
			len(dropped), formatIndexKeys(dropped))
	}
	return out
}

// filterPerIndexCandidatesByUpdateReadRatio drops candidate columns whose
// update writes dominate reads+filters beyond COVERING_INDEX_UPDATE_WRITE_READ_THRESHOLD_PCT.
// It returns the survivors with their dropped list populated.
func filterPerIndexCandidatesByUpdateReadRatio(in map[indexKey]*indexCandidate) map[indexKey]*indexCandidate {
	if len(in) == 0 {
		return nil
	}
	// Precompute per-table column metrics once to avoid O(indexes) re-parsing.
	hasStats, err := assessmentDB.HasSourceQueryStats()
	if err != nil || !hasStats {
		return in
	}
	queryStats, err := assessmentDB.GetSourceQueryStats()
	if err != nil || len(queryStats) == 0 {
		return in
	}
	tableColumnsMap, err := fetchAllTableColumnsFromAssessmentDB()
	if err != nil {
		log.Warnf("covering-index: cannot fetch table columns for ratio filter: %v", err)
		return in
	}

	metricsCache := make(map[string]map[string]columnOpsMetric) // "schema.table" -> col -> metric
	metricsFor := func(schema, table string) map[string]columnOpsMetric {
		qt := schema + "." + table
		if m, ok := metricsCache[qt]; ok {
			return m
		}
		cols, ok := tableColumnsMap[qt]
		if !ok {
			cols = tableColumnsMap[table]
		}
		if len(cols) == 0 {
			metricsCache[qt] = nil
			return nil
		}
		metrics := analyzeTableColumnOps(queryStats, schema, table, cols)
		m := make(map[string]columnOpsMetric, len(metrics))
		for _, mm := range metrics {
			m[mm.columnName] = mm
		}
		metricsCache[qt] = m
		return m
	}

	out := make(map[indexKey]*indexCandidate, len(in))
	for k, cand := range in {
		metrics := metricsFor(cand.schema, cand.table)
		if kept := applyRatioFilterToCandidate(cand, metrics); kept != nil {
			out[k] = kept
		}
	}
	return out
}

// applyRatioFilterToCandidate applies the update/read ratio policy to a single
// candidate, given the precomputed column metrics for its table. Columns whose
// update dominates reads+filters past COVERING_INDEX_UPDATE_WRITE_READ_THRESHOLD_PCT
// are moved to droppedList with reason "write_heavy". Returns nil if no columns
// survive. Exposed for unit tests.
func applyRatioFilterToCandidate(cand *indexCandidate, metrics map[string]columnOpsMetric) *indexCandidate {
	if cand == nil {
		return nil
	}
	kept := make(map[string]*colUsage, len(cand.missingCols))
	for col, usage := range cand.missingCols {
		m, ok := metrics[col]
		if !ok {
			// Candidate columns preserve source spelling (e.g. quoted "Email"),
			// while metrics keys are normalized/lower-cased.
			m, ok = metrics[strings.ToLower(strings.Trim(col, `"`))]
		}
		if ok {
			denom := m.updateWriteOps + m.readOps + m.filterOps
			if denom > 0 {
				pct := float64(m.updateWriteOps) * 100 / float64(denom)
				if pct >= queryissue.COVERING_INDEX_UPDATE_WRITE_READ_THRESHOLD_PCT {
					cand.addDropped(col, "write_heavy")
					continue
				}
			}
		}
		kept[col] = usage
	}
	if len(kept) == 0 {
		return nil
	}
	cand.missingCols = kept
	return cand
}

// buildRecommendations turns per-index candidates into the final
// CoveringIndexRecommendation map. Every surviving missing column (after the
// upstream ratio filter) is admitted; INCLUDE columns are sorted alphabetically
// for deterministic output. RelevantQueries / TotalCalls / TotalExecTimeMs
// are aggregated from the contributing queries of admitted columns (deduped
// by query text). Pre-cap drops recorded by the ratio filter (write_heavy)
// are surfaced verbatim in DroppedColumns.
func buildRecommendations(in map[indexKey]*indexCandidate) map[string]*queryissue.CoveringIndexRecommendation {
	out := make(map[string]*queryissue.CoveringIndexRecommendation, len(in))
	for k, cand := range in {
		rec := buildRecommendationForIndex(cand)
		if rec == nil {
			continue
		}
		out[queryissue.CoveringIndexRecommendationKey(k.schema, k.name)] = rec
	}
	return out
}

func buildRecommendationForIndex(cand *indexCandidate) *queryissue.CoveringIndexRecommendation {
	if cand == nil || len(cand.missingCols) == 0 {
		return nil
	}

	admitted := lo.Keys(cand.missingCols)
	sort.Strings(admitted)

	allRelevant := make(map[string]contributingQuery)
	for _, col := range admitted {
		for _, cq := range cand.missingCols[col].contributing {
			if _, ok := allRelevant[cq.text]; !ok {
				allRelevant[cq.text] = cq
			}
		}
	}

	relevant := make([]queryissue.RelevantQuery, 0, len(allRelevant))
	var totalCalls int64
	var totalExecMs float64
	for _, cq := range allRelevant {
		relevant = append(relevant, queryissue.RelevantQuery{Text: cq.text, Calls: cq.calls, TotalExecMs: cq.totalExecMs})
		totalCalls += cq.calls
		totalExecMs += cq.totalExecMs
	}
	sort.Slice(relevant, func(i, j int) bool { return relevant[i].TotalExecMs > relevant[j].TotalExecMs })
	if len(relevant) > queryissue.COVERING_INDEX_MAX_RELEVANT_QUERIES {
		relevant = relevant[:queryissue.COVERING_INDEX_MAX_RELEVANT_QUERIES]
	}

	dropped := lo.Map(cand.droppedList, func(d droppedColumn, _ int) queryissue.DroppedColumn {
		return queryissue.DroppedColumn{Name: d.name, Reason: d.reason}
	})

	return &queryissue.CoveringIndexRecommendation{
		IncludeColumns:  admitted,
		RelevantQueries: relevant,
		TotalCalls:      totalCalls,
		TotalExecTimeMs: totalExecMs,
		DroppedColumns:  dropped,
	}
}

// ---------- helpers for candidate bookkeeping ----------

// droppedList accumulates upstream drop decisions (ratio filter) so
// buildRecommendations can merge them into the final recommendation's
// DroppedColumns.
func (c *indexCandidate) addDropped(name, reason string) {
	c.droppedList = append(c.droppedList, droppedColumn{name: name, reason: reason})
}

// addMissingCol upserts a column into cand.missingCols and merges a contributing
// query contribution. Column names are stored verbatim so that quoted catalog
// spellings (e.g. "Email") survive into the recommendation output.
func (c *indexCandidate) addMissingCol(col string, q highImpactQuery, width int) {
	if c.missingCols == nil {
		c.missingCols = make(map[string]*colUsage)
	}
	u, ok := c.missingCols[col]
	if !ok {
		u = &colUsage{column: col, avgWidth: width}
		c.missingCols[col] = u
	}
	// Merge contributing (dedupe by text).
	found := false
	for _, cq := range u.contributing {
		if cq.text == q.text {
			found = true
			break
		}
	}
	if !found {
		u.contributing = append(u.contributing, contributingQuery{text: q.text, calls: q.calls, totalExecMs: q.totalExecMs})
		u.totalCalls += q.calls
		u.totalExecMs += q.totalExecMs
	}
	if width > u.avgWidth {
		u.avgWidth = width
	}
}

// truncateQuery shortens a query for log lines.
func truncateQuery(q string, maxLen int) string {
	if len(q) <= maxLen {
		return q
	}
	return q[:maxLen] + "..."
}

// ---------- Index-scan plan walker ----------

// walkPlanForIndexScans scans an EXPLAIN plan JSON recursively for every Index Scan
// node. EXPLAIN tells us which index the planner used; referencedCols tells us
// which columns from that scan relation are actually needed by the query. For
// each node we compute missing = referencedCols[relation] \ index_keys.
func walkPlanForIndexScans(planJSON []byte, meta *indexMeta, perIdx map[indexKey]*indexCandidate, q highImpactQuery, referencedCols map[string]map[string]bool) {
	var plans []map[string]interface{}
	if err := json.Unmarshal(planJSON, &plans); err != nil {
		return
	}
	for _, p := range plans {
		planNode, ok := p["Plan"].(map[string]interface{})
		if !ok {
			continue
		}
		walkNode(planNode, meta, perIdx, q, referencedCols)
	}
}

func walkNode(node map[string]interface{}, meta *indexMeta, perIdx map[indexKey]*indexCandidate, q highImpactQuery, referencedCols map[string]map[string]bool) {
	nodeType, _ := node["Node Type"].(string)
	// Only consider non-covering Index Scan nodes; Index Only Scan would already cover the query's referenced columns.
	if nodeType == "Index Scan" {
		schema, _ := node["Schema"].(string)
		table, _ := node["Relation Name"].(string)
		indexName, _ := node["Index Name"].(string)
		if table != "" && indexName != "" {
			k, ok := resolvePlanIndexKey(meta, schema, table, indexName)
			if !ok {
				return
			}
			// Partition-aware resolution: if this matched a per-partition
			// child index, fold up to the root parent index. Both the
			// candidate map key and the (schema, table) downstream lookups
			// must speak in terms of the parent so they line up with the
			// AST-collected referencedCols (keyed under the partitioned
			// parent table, never per-partition children) and with the
			// source CREATE INDEX that the DDL detector will look up.
			if root := meta.rootIndex(k); root != k {
				parentTbl, hasParentTbl := meta.indexTable[root]
				if hasParentTbl {
					log.Debugf("covering-index: resolved partition child %s.%s -> root %s.%s (parent table %s.%s)",
						k.schema, k.name, root.schema, root.name, parentTbl.schema, parentTbl.table)
					k = root
					schema, table = parentTbl.schema, parentTbl.table
				} else {
					log.Debugf("covering-index: parent index %s.%s for child %s.%s has no recorded table; staying on child key",
						root.schema, root.name, k.schema, k.name)
				}
			}
			if schema == "" {
				schema = meta.indexTable[k].schema
			}
			keyCols, known := meta.keyColumns[k]
			if known {
				keySet := lo.SliceToMap(keyCols, func(s string) (string, bool) { return s, true })
				cand, ok := perIdx[k]
				if !ok {
					cand = &indexCandidate{
						key:         k,
						schema:      schema,
						table:       table,
						keyColumns:  keyCols,
						missingCols: make(map[string]*colUsage),
					}
					perIdx[k] = cand
				}
				cols := referencedColumnsForRelation(referencedCols, schema, table)
				for col := range cols {
					if keySet[col] {
						continue
					}
					width := meta.widthFor(schema, table, col)
					cand.addMissingCol(col, q, width)
				}
				log.Debugf("covering-index: matched scan node to index %s.%s (table %s.%s) for query %q; %d missing cols on this candidate",
					k.schema, k.name, schema, table, truncateQuery(q.text, 80), len(cand.missingCols))
			}
		}
	}
	// Recurse Plans / InitPlan / SubPlan.
	for _, key := range []string{"Plans", "InitPlan", "SubPlan"} {
		if subs, ok := node[key].([]interface{}); ok {
			for _, sp := range subs {
				if sub, ok := sp.(map[string]interface{}); ok {
					walkNode(sub, meta, perIdx, q, referencedCols)
				}
			}
		}
	}
}

func resolvePlanIndexKey(meta *indexMeta, schema, table, indexName string) (indexKey, bool) {
	if schema != "" {
		k := indexKey{schema: schema, name: indexName}
		if _, ok := meta.keyColumns[k]; ok {
			return k, true
		}
	}
	var match indexKey
	found := false
	for k, tbl := range meta.indexTable {
		if k.name != indexName || tbl.table != table {
			continue
		}
		if found {
			return indexKey{}, false
		}
		match = k
		found = true
	}
	if found {
		return match, true
	}
	return indexKey{}, false
}

func referencedColumnsForRelation(colsByRel map[string]map[string]bool, schema, table string) map[string]bool {
	if len(colsByRel) == 0 {
		return nil
	}
	if cols := colsByRel[relationColumnKey(schema, table)]; len(cols) > 0 {
		return cols
	}
	return colsByRel[relationColumnKey("", table)]
}

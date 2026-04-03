package schemadiff

import (
	"fmt"
	"strings"
	"time"
)

type PostgresSnapshotProvider struct {
	dbType string // "postgresql" or "yugabytedb"
}

func (p *PostgresSnapshotProvider) DatabaseType() string {
	return p.dbType
}

func (p *PostgresSnapshotProvider) TakeSnapshot(db QueryExecutor, schemas []string) (*SchemaSnapshot, error) {
	snap := &SchemaSnapshot{
		DatabaseType: p.dbType,
		Schemas:      schemas,
		CapturedAt:   time.Now().UTC(),
	}

	schemaFilter := buildSchemaFilter(schemas)

	var err error
	if snap.Tables, err = p.loadTables(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading tables: %w", err)
	}
	if snap.Columns, err = p.loadColumns(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading columns: %w", err)
	}
	if snap.Constraints, err = p.loadConstraints(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading constraints: %w", err)
	}
	if snap.Indexes, err = p.loadIndexes(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading indexes: %w", err)
	}
	if snap.Sequences, err = p.loadSequences(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading sequences: %w", err)
	}
	if snap.Views, err = p.loadViews(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading views: %w", err)
	}
	if snap.MVs, err = p.loadMaterializedViews(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading materialized views: %w", err)
	}
	if snap.Functions, err = p.loadFunctions(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading functions: %w", err)
	}
	if snap.Triggers, err = p.loadTriggers(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading triggers: %w", err)
	}
	if snap.EnumTypes, err = p.loadEnumTypes(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading enum types: %w", err)
	}
	if snap.DomainTypes, err = p.loadDomainTypes(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading domain types: %w", err)
	}
	if snap.Extensions, err = p.loadExtensions(db); err != nil {
		return nil, fmt.Errorf("loading extensions: %w", err)
	}
	if snap.Policies, err = p.loadPolicies(db, schemaFilter); err != nil {
		return nil, fmt.Errorf("loading policies: %w", err)
	}

	return snap, nil
}

func buildSchemaFilter(schemas []string) string {
	quoted := make([]string, len(schemas))
	for i, s := range schemas {
		quoted[i] = "'" + s + "'"
	}
	return strings.Join(quoted, ", ")
}

func (p *PostgresSnapshotProvider) loadTables(db QueryExecutor, schemaFilter string) ([]Table, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, c.relname, c.relkind,
			COALESCE(pt.partstrat, ''),
			COALESCE(pg_get_expr(pt.partexprs, c.oid), ''),
			COALESCE((SELECT pn.nspname || '.' || pc.relname
				FROM pg_inherits i
				JOIN pg_class pc ON pc.oid = i.inhparent
				JOIN pg_namespace pn ON pn.oid = pc.relnamespace
				WHERE i.inhrelid = c.oid), ''),
			c.relreplident
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_partitioned_table pt ON pt.partrelid = c.oid
		WHERE c.relkind IN ('r', 'p')
		  AND n.nspname IN (%s)
		ORDER BY n.nspname, c.relname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		var relkind string
		var partStrat, partKey, parent string
		var replIdent string
		if err := rows.Scan(&t.Schema, &t.Name, &relkind, &partStrat, &partKey, &parent, &replIdent); err != nil {
			return nil, err
		}
		t.IsPartitioned = (relkind == "p")
		t.PartitionStrategy = partStrat
		t.PartitionKey = partKey
		t.ParentTable = parent
		t.ReplicaIdentity = replIdent
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (p *PostgresSnapshotProvider) loadColumns(db QueryExecutor, schemaFilter string) ([]Column, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, c.relname, a.attname,
			format_type(a.atttypid, a.atttypmod),
			a.attnotnull,
			COALESCE(pg_get_expr(d.adbin, d.adrelid), ''),
			a.attidentity,
			a.attnum,
			COALESCE(col.collname, ''),
			COALESCE(a.attgenerated::text, '')
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		LEFT JOIN pg_collation col ON col.oid = a.attcollation AND a.attcollation != 0
		WHERE a.attnum > 0
		  AND NOT a.attisdropped
		  AND c.relkind IN ('r', 'p', 'v', 'm')
		  AND n.nspname IN (%s)
		ORDER BY n.nspname, c.relname, a.attnum`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var notNull bool
		if err := rows.Scan(&col.Schema, &col.TableName, &col.Name,
			&col.DataType, &notNull, &col.DefaultExpr,
			&col.IsIdentity, &col.OrdinalPos, &col.Collation,
			&col.IsGenerated); err != nil {
			return nil, err
		}
		col.IsNullable = !notNull
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (p *PostgresSnapshotProvider) loadConstraints(db QueryExecutor, schemaFilter string) ([]Constraint, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, c.relname, con.conname, con.contype,
			COALESCE(
				(SELECT array_agg(a.attname ORDER BY k.ord)
				 FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
				 JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum),
				'{}'
			),
			pg_get_constraintdef(con.oid),
			COALESCE(rn.nspname, ''),
			COALESCE(rc.relname, ''),
			con.confdeltype,
			con.confupdtype,
			con.condeferrable,
			con.condeferred
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_class rc ON rc.oid = con.confrelid
		LEFT JOIN pg_namespace rn ON rn.oid = rc.relnamespace
		WHERE n.nspname IN (%s)
		ORDER BY n.nspname, c.relname, con.conname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []Constraint
	for rows.Next() {
		var con Constraint
		var contype string
		var columnsArr []string
		var expr string
		var delType, updType string
		if err := rows.Scan(&con.Schema, &con.TableName, &con.Name, &contype,
			&columnsArr, &expr,
			&con.RefSchema, &con.RefTable,
			&delType, &updType,
			&con.IsDeferrable, &con.IsDeferred); err != nil {
			return nil, err
		}
		con.Type = ConstraintType(contype)
		con.Columns = columnsArr
		con.Expression = expr
		con.OnDelete = delType
		con.OnUpdate = updType
		constraints = append(constraints, con)
	}
	return constraints, rows.Err()
}

func (p *PostgresSnapshotProvider) loadIndexes(db QueryExecutor, schemaFilter string) ([]Index, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, ct.relname, ci.relname,
			am.amname, ix.indisunique,
			pg_get_indexdef(ix.indexrelid),
			(ix.indpred IS NOT NULL),
			COALESCE(pg_get_expr(ix.indpred, ix.indrelid), '')
		FROM pg_index ix
		JOIN pg_class ci ON ci.oid = ix.indexrelid
		JOIN pg_class ct ON ct.oid = ix.indrelid
		JOIN pg_namespace n ON n.oid = ct.relnamespace
		JOIN pg_am am ON am.oid = ci.relam
		WHERE n.nspname IN (%s)
		  AND NOT ix.indisprimary
		ORDER BY n.nspname, ct.relname, ci.relname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var idx Index
		if err := rows.Scan(&idx.Schema, &idx.TableName, &idx.Name,
			&idx.AccessMethod, &idx.IsUnique, &idx.IndexDef,
			&idx.IsPartial, &idx.WhereClause); err != nil {
			return nil, err
		}
		idx.Columns = parseColumnsFromIndexDef(idx.IndexDef)
		indexes = append(indexes, idx)
	}
	return indexes, rows.Err()
}

func (p *PostgresSnapshotProvider) loadSequences(db QueryExecutor, schemaFilter string) ([]Sequence, error) {
	query := fmt.Sprintf(`
		SELECT schemaname, sequencename, data_type,
			start_value, increment_by, min_value, max_value,
			cycle, cache_size
		FROM pg_sequences
		WHERE schemaname IN (%s)
		ORDER BY schemaname, sequencename`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sequences []Sequence
	for rows.Next() {
		var seq Sequence
		if err := rows.Scan(&seq.Schema, &seq.Name, &seq.DataType,
			&seq.StartVal, &seq.Increment, &seq.MinVal, &seq.MaxVal,
			&seq.IsCyclic, &seq.CacheSize); err != nil {
			return nil, err
		}
		sequences = append(sequences, seq)
	}
	return sequences, rows.Err()
}

func (p *PostgresSnapshotProvider) loadViews(db QueryExecutor, schemaFilter string) ([]View, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, c.relname,
			pg_get_viewdef(c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'v'
		  AND n.nspname IN (%s)
		ORDER BY n.nspname, c.relname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []View
	for rows.Next() {
		var v View
		if err := rows.Scan(&v.Schema, &v.Name, &v.Definition); err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

func (p *PostgresSnapshotProvider) loadMaterializedViews(db QueryExecutor, schemaFilter string) ([]View, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, c.relname,
			pg_get_viewdef(c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'm'
		  AND n.nspname IN (%s)
		ORDER BY n.nspname, c.relname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []View
	for rows.Next() {
		var v View
		if err := rows.Scan(&v.Schema, &v.Name, &v.Definition); err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

func (p *PostgresSnapshotProvider) loadFunctions(db QueryExecutor, schemaFilter string) ([]Function, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, p.proname,
			pg_get_function_arguments(p.oid),
			pg_get_function_result(p.oid),
			l.lanname, p.prokind,
			p.provolatile, p.prosecdef, p.proisstrict,
			p.proparallel
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		JOIN pg_language l ON l.oid = p.prolang
		WHERE n.nspname IN (%s)
		  AND p.prokind IN ('f', 'p', 'a')
		ORDER BY n.nspname, p.proname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var functions []Function
	for rows.Next() {
		var f Function
		if err := rows.Scan(&f.Schema, &f.Name, &f.Arguments,
			&f.ReturnType, &f.Language, &f.Kind,
			&f.Volatility, &f.IsSecurityDef, &f.IsStrict,
			&f.ParallelSafety); err != nil {
			return nil, err
		}
		functions = append(functions, f)
	}
	return functions, rows.Err()
}

func (p *PostgresSnapshotProvider) loadTriggers(db QueryExecutor, schemaFilter string) ([]Trigger, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, c.relname, t.tgname,
			t.tgtype, t.tgenabled,
			pf.proname,
			(t.tgconstraint != 0)
		FROM pg_trigger t
		JOIN pg_class c ON c.oid = t.tgrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_proc pf ON pf.oid = t.tgfoid
		WHERE NOT t.tgisinternal
		  AND n.nspname IN (%s)
		ORDER BY n.nspname, c.relname, t.tgname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []Trigger
	for rows.Next() {
		var tr Trigger
		var tgtype int16
		if err := rows.Scan(&tr.Schema, &tr.TableName, &tr.Name,
			&tgtype, &tr.IsEnabled, &tr.FunctionName,
			&tr.IsConstraint); err != nil {
			return nil, err
		}
		tr.Timing, tr.Events, tr.ForEachRow = decodeTriggerType(tgtype)
		triggers = append(triggers, tr)
	}
	return triggers, rows.Err()
}

func (p *PostgresSnapshotProvider) loadEnumTypes(db QueryExecutor, schemaFilter string) ([]EnumType, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, t.typname,
			array_agg(e.enumlabel ORDER BY e.enumsortorder)
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		JOIN pg_enum e ON e.enumtypid = t.oid
		WHERE t.typtype = 'e'
		  AND n.nspname IN (%s)
		GROUP BY n.nspname, t.typname
		ORDER BY n.nspname, t.typname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enums []EnumType
	for rows.Next() {
		var e EnumType
		var vals []string
		if err := rows.Scan(&e.Schema, &e.Name, &vals); err != nil {
			return nil, err
		}
		e.Values = vals
		enums = append(enums, e)
	}
	return enums, rows.Err()
}

func (p *PostgresSnapshotProvider) loadDomainTypes(db QueryExecutor, schemaFilter string) ([]DomainType, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, t.typname,
			format_type(t.typbasetype, t.typtypmod),
			t.typnotnull,
			COALESCE(t.typdefault, ''),
			COALESCE(pg_get_constraintdef(con.oid), '')
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		LEFT JOIN pg_constraint con ON con.contypid = t.oid
		WHERE t.typtype = 'd'
		  AND n.nspname IN (%s)
		ORDER BY n.nspname, t.typname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []DomainType
	for rows.Next() {
		var d DomainType
		var notNull bool
		if err := rows.Scan(&d.Schema, &d.Name, &d.BaseType,
			&notNull, &d.Default, &d.CheckExpr); err != nil {
			return nil, err
		}
		d.IsNullable = !notNull
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

func (p *PostgresSnapshotProvider) loadExtensions(db QueryExecutor) ([]Extension, error) {
	query := `
		SELECT extname, extversion
		FROM pg_extension
		WHERE extname != 'plpgsql'
		ORDER BY extname`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var extensions []Extension
	for rows.Next() {
		var e Extension
		if err := rows.Scan(&e.Name, &e.Version); err != nil {
			return nil, err
		}
		extensions = append(extensions, e)
	}
	return extensions, rows.Err()
}

func (p *PostgresSnapshotProvider) loadPolicies(db QueryExecutor, schemaFilter string) ([]Policy, error) {
	query := fmt.Sprintf(`
		SELECT n.nspname, c.relname, pol.polname,
			pol.polpermissive, pol.polcmd,
			COALESCE(pg_get_expr(pol.polqual, pol.polrelid), ''),
			COALESCE(pg_get_expr(pol.polwithcheck, pol.polrelid), '')
		FROM pg_policy pol
		JOIN pg_class c ON c.oid = pol.polrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname IN (%s)
		ORDER BY n.nspname, c.relname, pol.polname`, schemaFilter)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		var pol Policy
		var polCmd string
		if err := rows.Scan(&pol.Schema, &pol.TableName, &pol.Name,
			&pol.IsPermissive, &polCmd,
			&pol.UsingExpr, &pol.WithCheckExpr); err != nil {
			return nil, err
		}
		pol.Command = polCmd
		policies = append(policies, pol)
	}
	return policies, rows.Err()
}

func decodeTriggerType(tgtype int16) (timing, events string, forEachRow bool) {
	// tgtype bitmask: bit 0 = ROW, bit 1 = BEFORE, bit 2 = INSERT,
	// bit 3 = DELETE, bit 4 = UPDATE, bit 5 = TRUNCATE, bit 6 = INSTEAD
	forEachRow = (tgtype & (1 << 0)) != 0
	if (tgtype & (1 << 6)) != 0 {
		timing = "INSTEAD OF"
	} else if (tgtype & (1 << 1)) != 0 {
		timing = "BEFORE"
	} else {
		timing = "AFTER"
	}

	var eventList []string
	if (tgtype & (1 << 2)) != 0 {
		eventList = append(eventList, "INSERT")
	}
	if (tgtype & (1 << 4)) != 0 {
		eventList = append(eventList, "UPDATE")
	}
	if (tgtype & (1 << 3)) != 0 {
		eventList = append(eventList, "DELETE")
	}
	if (tgtype & (1 << 5)) != 0 {
		eventList = append(eventList, "TRUNCATE")
	}
	events = strings.Join(eventList, " OR ")
	return
}

func parseColumnsFromIndexDef(indexDef string) []string {
	// pg_get_indexdef returns: CREATE [UNIQUE] INDEX name ON table USING method (col1, col2 [,...]) [WHERE ...]
	// We extract the column list between the first ( and the matching )
	start := strings.Index(indexDef, "(")
	if start < 0 {
		return nil
	}
	end := strings.LastIndex(indexDef, ")")
	if end < 0 || end <= start {
		return nil
	}
	colPart := indexDef[start+1 : end]

	// Handle WHERE clause that might contain parentheses
	whereIdx := strings.Index(strings.ToUpper(colPart), " WHERE ")
	if whereIdx >= 0 {
		colPart = colPart[:whereIdx]
	}

	parts := strings.Split(colPart, ",")
	var cols []string
	for _, p := range parts {
		cols = append(cols, strings.TrimSpace(p))
	}
	return cols
}

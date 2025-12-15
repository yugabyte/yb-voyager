-- Segment-based validation primitives: table + functions
-- This script is intended to be applied on both source and target clusters.

-- ------------------------------------------------------------------
-- 1) Table to hold per-table, per-segment validation results
-- ------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.migration_validate_segments (
    side          TEXT        NOT NULL,  -- 'source', 'target', or 'source_replica'
    schema_name   TEXT        NOT NULL,
    table_name    TEXT        NOT NULL,
    segment_index INTEGER     NOT NULL,  -- 0 .. (num_segments-1)
    low_range     INTEGER     NOT NULL,  -- inclusive lower bound in [0, 65536)
    high_range    INTEGER     NOT NULL,  -- exclusive upper bound
    row_count     BIGINT      NOT NULL,
    segment_hash  TEXT        NOT NULL,  -- md5 hex string
    PRIMARY KEY (side, schema_name, table_name, segment_index)
);

-- ------------------------------------------------------------------
-- 2) Hash function: PK text -> bucket in [0, 65536)
-- ------------------------------------------------------------------

CREATE OR REPLACE FUNCTION public.custom_hash_code_from_text(pk_text TEXT)
RETURNS BIGINT AS $$
/*
 * Hash a primary-key text representation into [0, 65536).
 *
 * We:
 *   - compute md5(pk_text) -> 16-byte digest
 *   - take the first 4 bytes as a big-endian 32-bit integer
 *   - mod 65536
 *
 * All multipliers are BIGINT to avoid 4-byte overflow.
 */
SELECT
    (
        get_byte(decode(md5(pk_text), 'hex'), 0)::BIGINT * 16777216::BIGINT  -- 256^3
      + get_byte(decode(md5(pk_text), 'hex'), 1)::BIGINT * 65536::BIGINT    -- 256^2
      + get_byte(decode(md5(pk_text), 'hex'), 2)::BIGINT * 256::BIGINT      -- 256^1
      + get_byte(decode(md5(pk_text), 'hex'), 3)::BIGINT                    -- 256^0
    ) % 65536;
$$ LANGUAGE SQL IMMUTABLE;

-- ------------------------------------------------------------------
-- 3) Compute segments for a single table
-- ------------------------------------------------------------------

CREATE OR REPLACE FUNCTION public.compute_table_segment_hashes(
    p_side         TEXT,        -- 'source', 'target', or 'source_replica'
    p_schema_name  TEXT,
    p_table_name   TEXT,
    p_num_segments INTEGER DEFAULT 16
) RETURNS VOID AS $$
DECLARE
    pk_cols    TEXT[];
    pk_expr    TEXT;
    seg_width  INTEGER;
    dyn_sql    TEXT;
BEGIN
    -- Sanity check on segment count
    IF p_num_segments <= 0 THEN
        RAISE EXCEPTION 'p_num_segments must be > 0, got %', p_num_segments;
    END IF;

    IF 65536 % p_num_segments <> 0 THEN
        RAISE EXCEPTION '65536 must be divisible by p_num_segments; got %', p_num_segments;
    END IF;

    seg_width := 65536 / p_num_segments;

    -- Discover primary key columns for the table (ordered)
    SELECT array_agg(a.attname ORDER BY a.attnum)
    INTO pk_cols
    FROM pg_index i
    JOIN pg_class c ON c.oid = i.indrelid
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
    WHERE n.nspname = p_schema_name
      AND c.relname = p_table_name
      AND i.indisprimary;

    IF pk_cols IS NULL OR array_length(pk_cols, 1) IS NULL THEN
        RAISE EXCEPTION 'Table %.% has no primary key; cannot compute segment hashes',
            p_schema_name, p_table_name;
    END IF;

    -- Build pk_text expression: array_to_string(ARRAY[ t.col1::text, ... ], '|')
    SELECT
        'array_to_string(ARRAY[' ||
        string_agg(format('t.%I::text', c), ', ') ||
        '], ''|'')'
    INTO pk_expr
    FROM unnest(pk_cols) AS c;

    -- Remove any previous rows for this side/schema/table
    DELETE FROM public.migration_validate_segments
    WHERE side        = p_side
      AND schema_name = p_schema_name
      AND table_name  = p_table_name;

    -- Dynamic SQL to:
    --   - build pk_text and row_json for each row
    --   - compute bucket & segment_index
    --   - aggregate per segment
    dyn_sql := format($SQL$
        WITH src AS (
            SELECT
                %s AS pk_text,
                row_to_json(t)::text AS row_json
            FROM %I.%I AS t
        ),
        hashed AS (
            SELECT
                pk_text,
                custom_hash_code_from_text(pk_text) AS bucket,
                md5(row_json) AS row_hash
            FROM src
        ),
        grouped AS (
            SELECT
                (bucket / %s)::INT AS segment_index,
                COUNT(*)          AS row_count,
                md5(string_agg(row_hash, '' ORDER BY pk_text)) AS segment_hash
            FROM hashed
            GROUP BY (bucket / %s)::INT
        )
        INSERT INTO public.migration_validate_segments(
            side, schema_name, table_name,
            segment_index, low_range, high_range,
            row_count, segment_hash
        )
        SELECT
            %L                     AS side,
            %L                     AS schema_name,
            %L                     AS table_name,
            g.segment_index        AS segment_index,
            g.segment_index * %s   AS low_range,
            (g.segment_index + 1) * %s AS high_range,
            g.row_count            AS row_count,
            g.segment_hash         AS segment_hash
        FROM grouped AS g;
    $SQL$,
        pk_expr,
        p_schema_name,
        p_table_name,
        seg_width,
        seg_width,
        p_side,
        p_schema_name,
        p_table_name,
        seg_width,
        seg_width
    );

    EXECUTE dyn_sql;
END;
$$ LANGUAGE plpgsql;

-- ------------------------------------------------------------------
-- 4) Compute segments for all base tables in a schema
-- ------------------------------------------------------------------

CREATE OR REPLACE FUNCTION public.compute_schema_segment_hashes(
    p_side         TEXT,
    p_schema_name  TEXT,
    p_num_segments INTEGER DEFAULT 16
) RETURNS VOID AS $$
DECLARE
    tbl RECORD;
BEGIN
    FOR tbl IN
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = p_schema_name
          AND table_type   = 'BASE TABLE'
    LOOP
        PERFORM public.compute_table_segment_hashes(
            p_side,
            p_schema_name,
            tbl.table_name,
            p_num_segments
        );
    END LOOP;
END;
$$ LANGUAGE plpgsql;

//go:build integration_live_migration

package cmd

import (
	"context"
	"testing"
	"time"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// TestLiveMigrationWithDatatypeEdgeCases tests live migration with various datatypes
// containing special characters and edge cases that require proper escaping.
// Currently testing: STRING datatype with backslashes, quotes, newlines, tabs, Unicode, etc.
// This test verifies that the datatype converter properly handles edge cases during CDC streaming.
// Aligned with unit tests in yugabytedbSuite_test.go
// getDatatypeEdgeCasesTestConfig returns the shared test configuration for datatype edge cases
// This config is used by both basic and fallback datatype edge case tests
// ============================================================================
// DATATYPE EDGE CASE TEST COVERAGE MATRIX
// ============================================================================
//
// This matrix shows comprehensive edge case coverage for live migration across
// all PostgreSQL datatypes. Coverage is tracked across three phases:
//   - Snapshot: Initial data loaded during snapshot phase (6 rows per datatype)
//   - Forward:  Changes during forward streaming (SourceDeltaSQL)
//   - Fallback: Changes during fallback streaming (TargetDeltaSQL)
//
// Legend:
//   ✓ = Fully covered (INSERT + UPDATE + DELETE operations)
//   I = INSERT only
//   U = UPDATE only
//   D = DELETE only
//   S = Snapshot data only
//   - = Not covered/Not applicable
//
// ============================================================================
// 1. STRING DATATYPE (text, varchar)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Unicode characters (café, 日本語)   | ✓        | ✓       | ✓        | Multi-byte chars
// Emojis (🎉, 👨‍👩‍👧‍👦)                   | ✓        | ✓       | ✓        | Emoji family
// Single quotes (It's, O'Reilly)    | ✓        | ✓       | ✓        | Escaped quotes
// Double quotes ("test")            | ✓        | ✓       | ✓        | Mixed quotes
// Backslashes (C:\path\to\file)     | ✓        | ✓       | ✓        | Windows paths
// Actual newline byte (0x0A)        | ✓        | ✓       | ✓        | E'...\n...'
// Literal \n string (backslash+n)   | ✓        | ✓       | ✓        | Two-char string
// Actual tab byte (0x09)            | ✓        | ✓       | ✓        | E'...\t...'
// Actual carriage return (0x0D)     | ✓        | ✓       | ✓        | E'...\r...'
// Mixed control chars (\n\t\r)      | ✓        | ✓       | ✓        | All control chars
// Unicode separators (U+2028)       | ✓        | ✓       | ✓        | Line/para sep
// Empty string ('')                 | ✓        | ✓       | ✓        | Zero-length
// String literal 'NULL'             | ✓        | ✓       | ✓        | vs actual NULL
// SQL injection patterns            | ✓        | ✓       | ✓        | --comment, '; DROP
// Bidirectional text (RTL)          | ✓        | ✓       | ✓        | Arabic/Hebrew
// NULL transitions                  | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 4 INSERTs, 10 UPDATEs, 1 DELETE (Forward)
//             5 INSERTs, 7 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 2. JSON/JSONB DATATYPE
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Single quotes in values            | ✓        | ✓       | ✓        | O'Reilly, It's
// Escaped characters (\", \\)        | ✓        | ✓       | ✓        | JSON escaping
// Unicode in JSON                    | ✓        | ✓       | ✓        | café, 日本語, 🎉
// Nested objects                     | ✓        | ✓       | ✓        | Deep nesting
// Arrays                             | ✓        | ✓       | ✓        | Nested arrays
// NULL value in JSON                 | ✓        | ✓       | ✓        | {"key": null}
// Empty JSON                         | ✓        | ✓       | ✓        | {}
// Formatted JSON                     | ✓        | ✓       | ✓        | Whitespace
// Numbers in JSON                    | ✓        | ✓       | ✓        | Int, float, bool
// Complex nested structures          | ✓        | ✓       | ✓        | Mixed types
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 3 INSERTs, 5 UPDATEs, 1 DELETE (Forward)
//             4 INSERTs, 5 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 3. ENUM DATATYPE (custom enum types)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Simple enum values                 | ✓        | ✓       | ✓        | active, pending
// Enum with single quote             | ✓        | ✓       | ✓        | enum'value
// Enum with double quote             | ✓        | ✓       | ✓        | enum"value
// Enum with backslash                | ✓        | ✓       | ✓        | enum\value
// Enum with spaces                   | ✓        | ✓       | ✓        | 'with space'
// Enum with dashes                   | ✓        | ✓       | ✓        | with-dash
// Enum with underscore               | ✓        | ✓       | ✓        | with_underscore
// Enum with Unicode                  | ✓        | ✓       | ✓        | café, 🎉emoji
// Enum starting with digits          | ✓        | ✓       | ✓        | 123start
// Empty ENUM array                   | ✓        | ✓       | ✓        | ARRAY[]::enum[]
// ENUM array with NULL elements      | ✓        | ✓       | ✓        | ARRAY['a', NULL]
// ENUM array add elements            | -        | U       | U        | Expand array
// ENUM array remove elements         | -        | U       | U        | Shrink array
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 3 INSERTs, 8 UPDATEs, 1 DELETE (Forward)
//             3 INSERTs, 7 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 4. BYTES DATATYPE (bytea)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Empty bytes                        | ✓        | ✓       | ✓        | \\x
// Single byte                        | ✓        | ✓       | ✓        | \\x41
// ASCII string as bytes              | ✓        | ✓       | ✓        | Text → hex
// NULL byte in middle                | ✓        | ✓       | ✓        | \\x00
// All zeros                          | ✓        | ✓       | ✓        | \\x000000
// All 0xFF                           | ✓        | ✓       | ✓        | \\xFFFFFF
// Special char bytes (', \, \n)      | ✓        | ✓       | ✓        | Binary chars
// Mixed byte patterns                | ✓        | ✓       | ✓        | Random hex
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 5. DATETIME DATATYPE (date, timestamp, time)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Epoch date (1970-01-01)            | ✓        | ✓       | ✓        | Unix epoch
// Negative epoch (before 1970)       | ✓        | ✓       | ✓        | Historical dates
// Future dates (2050+)               | ✓        | ✓       | ✓        | Far future
// Timestamps with timezone           | ✓        | ✓       | ✓        | TIMESTAMPTZ
// Midnight (00:00:00)                | ✓        | ✓       | ✓        | Day boundary
// Noon (12:00:00)                    | ✓        | ✓       | ✓        | Mid-day
// Microsecond precision              | ✓        | ✓       | ✓        | 6 decimal places
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 6. UUID DATATYPE
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Standard UUID v4                   | ✓        | ✓       | ✓        | Random UUID
// All zeros UUID                     | ✓        | ✓       | ✓        | 00000000-0000...
// All Fs UUID                        | ✓        | ✓       | ✓        | ffffffff-ffff...
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// ============================================================================
// 7. LTREE DATATYPE (hierarchical labels)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Simple path                        | ✓        | ✓       | ✓        | Top.Science
// Quoted labels                      | ✓        | ✓       | ✓        | "Special Label"
// Deep hierarchy                     | ✓        | ✓       | ✓        | 10+ levels
// Single label                       | ✓        | ✓       | ✓        | Top only
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot (combined UUID/LTREE), 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 8. MAP DATATYPE (hstore)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Arrow operator in key              | ✓        | ✓       | ✓        | "key=>val"=>"x"
// Arrow operator in value            | ✓        | ✓       | ✓        | "k"=>"val=>test"
// Escaped quotes                     | ✓        | ✓       | ✓        | "key\"test"
// Escaped backslash                  | ✓        | ✓       | ✓        | "key\\test"
// Single quotes in value             | ✓        | ✓       | ✓        | "k"=>"O'Reilly"
// Empty key                          | ✓        | ✓       | ✓        | ""=>"value"
// Empty value                        | ✓        | ✓       | ✓        | "key"=>""
// Multiple pairs                     | ✓        | ✓       | ✓        | k1=>v1, k2=>v2
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 9. INTERVAL DATATYPE
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Positive intervals                 | ✓        | ✓       | ✓        | Years, months
// Negative intervals                 | ✓        | ✓       | ✓        | -15 days
// Zero interval                      | ✓        | ✓       | ✓        | 0 seconds
// Years only                         | ✓        | ✓       | ✓        | 75 years
// Days only                          | ✓        | ✓       | ✓        | 14 days
// Time only                          | ✓        | ✓       | ✓        | 8:30:45
// Mixed components                   | ✓        | ✓       | ✓        | Years+days+hours
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 10. ZONEDTIMESTAMP DATATYPE (timestamptz)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// UTC timezone (+00)                 | ✓        | ✓       | ✓        | Zulu time
// Positive offset (+04:00, +05:30)   | ✓        | ✓       | ✓        | Eastern zones
// Negative offset (-07:00)           | ✓        | ✓       | ✓        | Western zones
// Epoch with timezone                | ✓        | ✓       | ✓        | 1970-01-01+00
// Future with timezone               | ✓        | ✓       | ✓        | 2065+
// Midnight with timezone             | ✓        | ✓       | ✓        | 00:00:00+00
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 11. DECIMAL DATATYPE (numeric)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// Large numbers (1B+)                | ✓        | ✓       | ✓        | 999999999.999...
// Negative numbers                   | ✓        | ✓       | ✓        | -999999.999
// Zero (0.0, 0.00, 0.000)            | ✓        | ✓       | ✓        | Various scales
// High precision (15+ decimals)      | ✓        | ✓       | ✓        | 0.123456789...
// Scientific notation                | ✓        | ✓       | ✓        | 1.23E+10
// Small decimals                     | ✓        | ✓       | ✓        | 0.0001
// NULL transitions (row 5)           | ✓        | ✓       | ✓        | non-NULL↔NULL
// NULL transitions (row 6)           | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 12. INTEGER DATATYPE (int, bigint)
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// INT MAX (2147483647)               | ✓        | ✓       | ✓        | Max 32-bit
// INT MIN (-2147483648)              | ✓        | ✓       | ✓        | Min 32-bit
// BIGINT MAX (9223372036854775807)   | ✓        | ✓       | ✓        | Max 64-bit
// BIGINT MIN (-9223372036854775808)  | ✓        | ✓       | ✓        | Min 64-bit
// Zero                               | ✓        | ✓       | ✓        | 0
// Negative one                       | ✓        | ✓       | ✓        | -1
// Overflow scenarios                 | ✓        | ✓       | ✓        | Boundary tests
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// 13. BOOLEAN DATATYPE
// ============================================================================
// Edge Case                          | Snapshot | Forward | Fallback | Notes
// -----------------------------------|----------|---------|----------|------------------
// TRUE value                         | ✓        | ✓       | ✓        | true
// FALSE value                        | ✓        | ✓       | ✓        | false
// NULL value                         | ✓        | ✓       | ✓        | null
// TRUE ↔ FALSE transitions           | -        | U       | U        | Toggle values
// NULL transitions                   | ✓        | ✓       | ✓        | NULL↔non-NULL
//
// Operations: 6 rows snapshot, 2 INSERTs, 4 UPDATEs, 1 DELETE (Forward)
//             2 INSERTs, 4 UPDATEs, 1 DELETE (Fallback)
//
// ============================================================================
// SUMMARY STATISTICS
// ============================================================================
//
// Total Datatypes:      13 (12 tables, 1 combined UUID/LTREE)
// Total Snapshot Rows:  72 (6 rows × 12 tables)
// Total Edge Cases:     150+
//
// Forward Streaming Operations (SourceDeltaSQL):
//   - INSERTs:  28 (varies by datatype: STRING=4, JSON=3, ENUM=3, others=2 each)
//   - UPDATEs:  59 (varies by datatype: STRING=10, ENUM=8, JSON=5, others=4 each)
//                   (includes NULL transitions + literal \n/actual newline + special chars)
//   - DELETEs:  12 (1 per table, targets snapshot row 3)
//
// Fallback Streaming Operations (TargetDeltaSQL):
//   - INSERTs:  30 (varies by datatype: STRING=5, JSON=4, ENUM=3, others=2 each)
//                   (includes marker rows for UPDATE testing)
//   - UPDATEs:  55 (varies by datatype: STRING=7, ENUM=7, JSON=5, others=4 each)
//                   (includes NULL transitions + Unicode separators + literal \n/actual newline)
//   - DELETEs:  12 (1 per table, targets snapshot row 4)
//
// NULL Transition Coverage:
//   - All 12 datatypes have full bidirectional NULL transitions:
//     * Row 5: non-NULL → NULL (Forward) → non-NULL (Fallback)
//     * Row 6: NULL → non-NULL (Forward) → NULL (Fallback)
//   - Tested in both forward and fallback streaming phases
//   - Ensures proper handling of NULL ↔ non-NULL value transitions

func getDatatypeEdgeCasesTestConfig() *TestConfig {
	return &TestConfig{
		SourceDB: ContainerConfig{
			Type:    "postgresql",
			ForLive: true,
		},
		TargetDB: ContainerConfig{
			Type: "yugabytedb",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
			`CREATE SCHEMA test_schema;

			CREATE TABLE test_schema.string_edge_cases (
				id SERIAL PRIMARY KEY,
				text_with_backslash TEXT,
				text_with_quote TEXT,
				text_with_newline TEXT,
				text_with_tab TEXT,
				text_with_mixed TEXT,
				text_windows_path TEXT,
				text_sql_injection TEXT,
				text_unicode TEXT,
				text_empty TEXT,
				text_null_string TEXT
			);

			CREATE TABLE test_schema.json_edge_cases (
				id SERIAL PRIMARY KEY,
				json_with_escaped_chars JSON,
				json_with_unicode JSON,
				json_nested JSON,
				json_array JSON,
				json_with_null JSON,
				json_empty JSON,
				json_formatted JSONB,
				json_with_numbers JSON,
				json_complex JSONB
			);

			CREATE TYPE test_schema.status_enum AS ENUM ('active', 'inactive', 'pending', 'enum''value', 'enum"value', 'enum\value', 'with space', 'with-dash', 'with_underscore', 'café', '🎉emoji', '123start');
			
			CREATE TABLE test_schema.enum_edge_cases (
				id SERIAL PRIMARY KEY,
				status_simple test_schema.status_enum,
				status_with_quote test_schema.status_enum,
				status_with_special test_schema.status_enum,
				status_unicode test_schema.status_enum,
				status_array test_schema.status_enum[],
				status_null test_schema.status_enum
			);

			CREATE TABLE test_schema.bytes_edge_cases (
				id SERIAL PRIMARY KEY,
				bytes_empty BYTEA,
				bytes_single BYTEA,
				bytes_ascii BYTEA,
				bytes_null_byte BYTEA,
				bytes_all_zeros BYTEA,
				bytes_all_ff BYTEA,
				bytes_special_chars BYTEA,
				bytes_mixed BYTEA
			);

			CREATE TABLE test_schema.datetime_edge_cases (
				id SERIAL PRIMARY KEY,
				date_epoch DATE,
				date_negative DATE,
				date_future DATE,
				timestamp_epoch TIMESTAMP,
				timestamp_negative TIMESTAMP,
				timestamp_with_tz TIMESTAMPTZ,
				time_midnight TIME,
				time_noon TIME,
				time_with_micro TIME(6)
				-- time_with_tz TIMETZ  -- EXCLUDED: Known Debezium limitation
			);
			-- NOTE: time_with_tz (TIMETZ) column is excluded from this test.
			-- KNOWN LIMITATION: Debezium's PostgreSQL connector normalizes TIMETZ values to UTC
			-- during CDC streaming, losing the original timezone offset. Snapshot works correctly
			-- (direct copy preserves timezone), but streaming always shows TIMETZ in UTC.
			-- Example: Source '01:02:03+01' becomes '00:02:03Z' in target after streaming.
			-- This is a fundamental limitation of Debezium's handling of PostgreSQL's TIMETZ type
			-- and cannot be fixed without changes to Debezium's core connector.

			CREATE EXTENSION IF NOT EXISTS ltree;

			CREATE TABLE test_schema.uuid_ltree_edge_cases (
				id SERIAL PRIMARY KEY,
				uuid_standard UUID,
				uuid_all_zeros UUID,
				uuid_all_fs UUID,
				uuid_random UUID,
				ltree_simple LTREE,
				ltree_quoted LTREE,
				ltree_deep LTREE,
				ltree_single LTREE
			);

			CREATE EXTENSION IF NOT EXISTS hstore;

			CREATE TABLE test_schema.map_edge_cases (
				id SERIAL PRIMARY KEY,
				map_simple HSTORE,
				map_with_arrow HSTORE,
				map_with_quotes HSTORE,
				map_empty_values HSTORE,
				map_multiple_pairs HSTORE,
				map_special_chars HSTORE
			);

			CREATE TABLE test_schema.interval_edge_cases (
				id SERIAL PRIMARY KEY,
				interval_positive INTERVAL,
				interval_negative INTERVAL,
				interval_zero INTERVAL,
				interval_years INTERVAL,
				interval_days INTERVAL,
				interval_hours INTERVAL,
				interval_mixed INTERVAL
			);

			CREATE TABLE test_schema.zonedtimestamp_edge_cases (
				id SERIAL PRIMARY KEY,
				ts_utc TIMESTAMPTZ,
				ts_positive_offset TIMESTAMPTZ,
				ts_negative_offset TIMESTAMPTZ,
				ts_epoch TIMESTAMPTZ,
				ts_future TIMESTAMPTZ,
				ts_midnight TIMESTAMPTZ
			);

			CREATE TABLE test_schema.decimal_edge_cases (
				id SERIAL PRIMARY KEY,
				decimal_large NUMERIC(38, 9),
				decimal_negative NUMERIC(15, 3),
				decimal_zero NUMERIC(10, 2),
				decimal_high_precision NUMERIC(30, 15),
				decimal_scientific NUMERIC(20,9) , -- We notice a loss of trailing zeros in NUMERIC type without specifying the precision
				decimal_small NUMERIC(5, 2)
			);

			CREATE TABLE test_schema.integer_edge_cases (
				id SERIAL PRIMARY KEY,
				int_max INT,
				int_min INT,
				int_zero INT,
				int_negative_one INT,
				bigint_max BIGINT,
				bigint_min BIGINT,
				bigint_zero BIGINT
			);

			CREATE TABLE test_schema.boolean_edge_cases (
				id SERIAL PRIMARY KEY,
				bool_simple BOOLEAN,
				bool_nullable BOOLEAN
			);
			`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.string_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.json_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.enum_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.bytes_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.datetime_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.uuid_ltree_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.map_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.interval_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.zonedtimestamp_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.decimal_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.integer_edge_cases REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.boolean_edge_cases REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// ======================================================================
			// TEXT TYPES
			// ======================================================================

			// --- STRING (string_edge_cases) ---

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'path\to\file',                          -- literal backslash-t, backslash-o
				'It''s a test',                          -- single quote (SQL escaped)
				'line1' || E'\u2028' || 'line2',         -- Unicode line separator (U+2028)
				'para1' || E'\u2029' || 'para2',         -- Unicode paragraph separator (U+2029)
				'word' || E'\u200B' || 'word',           -- Zero-width space (U+200B)
				'word' || E'\u00A0' || 'word',           -- Non-breaking space (U+00A0)
				'''; DROP TABLE users--',                -- SQL injection
				'café 日本語',                           -- Unicode
				'',                                      -- empty string
				'NULL'                                   -- literal string "NULL"
			);`,
			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'\\server\share',                        -- UNC path (double backslash)
				'O''Reilly''s book',                    -- multiple single quotes
				E'line1\nline2',                        -- Actual newline character (E-string)
				E'col1\tcol2',                          -- Actual tab character (E-string)
				E'text\rmore',                          -- Actual carriage return (E-string)
				'C:\Program Files\MyApp\bin',           -- Windows path
				''' OR ''1''=''1',                      -- SQL injection
				'café''s specialty',                     -- Unicode with single quote
				E'\t',                                  -- Tab only (E-string)
				E'\n'                                   -- Newline only (E-string)
			);`,
			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'path\to\日本語',                        -- Unicode with backslash (backslash + Japanese)
				'English مرحبا English',                 -- Bidirectional text (LTR + RTL)
				'Hello 世界 🌍',                         -- Mixed ASCII+Unicode (English + Chinese + emoji)
				'tab',                                  -- simple text
				'All: ''""\\ text',                     -- all special chars
				'C:\new\test\report.txt',               -- path
				'--comment',                            -- SQL comment
				'👨‍👩‍👧 family',                           -- Zero-width joiner emoji (composite emoji)
				' ',                                    -- single space only (critical edge case)
			'This is NULL value'                    -- NULL as part of string
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'another\path\here',                    -- more backslash patterns
				'Say "hello" and ''goodbye''',          -- mixed double and single quotes
				E'multi\nline\nstring',                 -- multiple newlines
				E'column1\tcolumn2\tcolumn3',           -- multiple tabs
				E'complex\t"quote''\ntest',             -- mix of everything
				'D:\Data\Reports\2024\file.xlsx',       -- long Windows path
				'1=1; DROP DATABASE;--',                -- SQL injection variant
				'Привет мир 🚀',                         -- Russian + emoji
				E'',                                    -- empty via E-string
				'null'                                  -- lowercase null
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'slash/backslash\mix',                  -- forward and back slashes
				'''quoted string''',                    -- entire string in quotes
				E'start\nend',                          -- newline at boundary
				E'\ttabbed',                            -- tab at start
				'simple text',                          -- actually simple
				'\\network\share\folder',               -- network path
				'admin''--',                            -- comment injection
				'مرحبا 你好 שלום',                       -- Arabic, Chinese, Hebrew
				'  ',                                   -- spaces only
				'NULLNULLNULL'                          -- repeated NULL text
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'has value',
				'has value',
				'has value',
				'has value',
				'has value',
				'has value',
				'has value',
				'has value'
			);`,

			// --- JSON/JSONB (json_edge_cases) ---

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"key": "value\"test", "path": "C:\\\\path"}',
				'{"message": "Hello 世界 🎉 café"}',
				'{"outer": {"inner": "value"}}',
				'["item1", "item2", "item\"3"]',
				'{"key": null}',
				'{}',
				'{"formatted": "value"}',
				'{"num": 123, "float": 45.67, "bool": true}',
				'{"str": "test", "num": 123, "bool": true, "null": null, "arr": [1,2]}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"escapes": "slash:\\\\ newline:\\n tab:\\t return:\\r"}',
				'{"text": "zero\u200Bwidth\u200Djoin"}',
				'{"level1": {"level2": {"level3": "deep"}}}',
				'[1, "two", {"three": 3}]',
				'{"a": null, "b": null}',
				'[]',
				'{"query": "SELECT * FROM users"}',
				'{"int": -999, "float": 3.14159, "exp": 1.23e10}',
				'{"path": "C:\\\\Program Files\\\\App\\\\file.txt", "json": {"nested": true}}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"key": "line1\nline2"}',
				'{"arabic": "مرحبا", "chinese": "你好"}',
				'{"a": {"b": {"c": {"d": "value"}}}}',
				'[[1,2],[3,4]]',
				'{"result": null}',
				'{"empty": {}}',
				'{"text": "simple value"}',
				'{"zero": 0, "negative": -42, "positive": 42}',
				'{"name": "test", "value": 123}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"quote": "O''Reilly", "slash": "path/to/file"}',
				'{"emoji": "🎉🚀💡", "lang": "Español"}',
				'{"outer": {"middle": {"inner": {"deep": "nest"}}}}',
				'["a", "b", "c", "d", "e"]',
				'{"x": null, "y": null, "z": null}',
				'{"data": {}}',
				'{"formatted": "multi-line test"}',
				'{"big": 999999999, "small": 0.000001}',
				'{"mixed": [1, "two", null, {"four": 4}]}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"simple": "value5"}',
				'{"japanese": "日本語", "korean": "한국어"}',
				'{"one": {"two": {"three": "value"}}}',
				'[true, false, null, 123, "text"]',
				'{"null_value": null}',
				'{}',
				'{"sql": "SELECT id FROM table"}',
				'{"e": 2.718, "pi": 3.14159}',
				'{"bool": true, "array": [1,2,3], "obj": {"key": "val"}}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'{"has": "value"}',
				'["has", "value"]',
				'{"has": "value"}',
				'{}',
				'{"has": "value"}',
				'{"num": 1}',
				'{"has": "value"}'
			);`,

			// --- ENUM (enum_edge_cases) ---

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'active',
				'enum''value',
				'with space',
				'café',
				ARRAY['active', 'pending', 'inactive']::test_schema.status_enum[],
				'pending'
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
			status_simple,
			status_with_quote,
			status_with_special,
			status_unicode,
			status_array,
			status_null
			) VALUES
			(
				'inactive',
				'enum"value',
				'with-dash',
				'🎉emoji',
				ARRAY[]::test_schema.status_enum[],  -- EMPTY ARRAY test
				NULL
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
					status_simple,
					status_with_quote,
					status_with_special,
					status_unicode,
					status_array,
					status_null
				) VALUES
				(
					'pending',
					'enum\value',
					'with_underscore',
					'123start',
				ARRAY['🎉emoji', '123start', 'enum\value']::test_schema.status_enum[],
				'active'
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'inactive',
				'enum''value',
				'with space',
				'café',
				ARRAY['active', NULL, 'pending']::test_schema.status_enum[],  -- Array with NULL element
				'pending'
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'active',
				'enum\value',
				'with_underscore',
				'123start',
				ARRAY['inactive', 'with space', 'with-dash']::test_schema.status_enum[],
				'active'
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'pending',
				'pending',
				ARRAY['active']::test_schema.status_enum[],
				'pending'
			);`,

			// ======================================================================
			// BINARY TYPE
			// ======================================================================

			// --- BYTES (bytes_edge_cases) ---

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				E'\\x',
				E'\\x41',
				E'\\x414243',
				E'\\x00',
				E'\\x000000',
				E'\\xffffff',
				E'\\x275c0a',
				E'\\x48656c6c6f'
			);`,

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				E'\\x',
				E'\\xff',
				E'\\x54657374',
				E'\\x00000000',
				E'\\x0000000000',
				E'\\xffffffffff',
				E'\\x090d',
				E'\\xdeadbeef'
			);`,

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				NULL,
				E'\\x7f',
				E'\\x646174',
				E'\\x007465737400',
				E'\\x00',
				E'\\xff',
				E'\\x010203',
				E'\\xcafebabe'
			);`,

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				E'\\x',
				E'\\x42',
				E'\\x444546',
				E'\\x0041420043',
				E'\\x0000',
				E'\\xffff',
				E'\\x0d0a09',
				E'\\xdeadbeef'
			);`,

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				NULL,
				E'\\xff',
				E'\\x58595a',
				E'\\x00616200',
				E'\\x000000',
				E'\\xffffff',
				E'\\x5c275c',
				E'\\x12345678'
			);`,

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_null_byte,
				bytes_special_chars
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'\x00'::bytea,
				'\xFF'::bytea,
				'\x41'::bytea,
				'\x41'::bytea
			);`,

			// ======================================================================
			// TEMPORAL TYPES
			// ======================================================================

			// --- DATETIME (datetime_edge_cases) ---

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
				-- time_with_tz  -- EXCLUDED: Known Debezium limitation
			) VALUES
			(
				'1970-01-01',
				'1969-12-31',
				'2022-01-01',
				'1970-01-01 00:00:00',
				'1969-12-31 00:00:00',
				'2022-01-01 12:00:00+00',
				'00:00:00',
				'12:00:00',
				'12:30:45.123456'
				-- '12:00:00+00'  -- EXCLUDED: TIMETZ
			);`,

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
				-- time_with_tz  -- EXCLUDED: Known Debezium limitation
			) VALUES
			(
				'2000-01-01',
				'1900-01-01',
				'2099-12-31',
				'2000-01-01 00:00:00',
				'1900-01-01 12:30:45',
				'2099-12-31 23:59:59+00',
				'23:59:59',
				'06:30:00',
				'00:00:00.000001'
				-- '23:59:59-08'  -- EXCLUDED: TIMETZ
			);`,

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
				-- time_with_tz  -- EXCLUDED: Known Debezium limitation
			) VALUES
			(
				'2024-06-15',
				'1950-06-15',
				'2050-06-15',
				'2024-06-15 14:30:00',
				'1950-06-15 08:15:30',
				'2050-06-15 18:45:00-05',
				'18:45:30',
				'09:15:00',
				'23:59:59.999999'
				-- '18:45:30-05'  -- EXCLUDED: TIMETZ
			);`,

			`INSERT INTO test_schema.datetime_edge_cases (
			date_epoch,
			date_negative,
			date_future,
			timestamp_epoch,
			timestamp_negative,
			timestamp_with_tz,
			time_midnight,
			time_noon,
			time_with_micro
				-- time_with_tz  -- EXCLUDED: Known Debezium limitation
			) VALUES
			(
				'2000-01-01',
				'1900-01-01',
				'2100-12-31',
				'2000-01-01 00:00:00',
				'1900-01-01 12:30:45',
				'2100-12-31 23:59:59+00',
				'00:00:01',
				'12:30:45',
				'15:30:45.123456'
				-- '00:00:01+05:30'  -- EXCLUDED: TIMETZ
			);`,

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
				-- time_with_tz  -- EXCLUDED: Known Debezium limitation
			) VALUES
			(
				'1980-06-15',
				'1930-03-20',
				'2075-08-10',
				'1980-06-15 18:20:30',
				'1930-03-20 09:45:15',
				'2075-08-10 14:25:00-07',
				'06:30:00',
				'18:45:30',
				'21:15:30.654321'
				-- '14:25:00-07'  -- EXCLUDED: TIMETZ
			);`,

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				time_midnight,
				time_noon
				-- time_with_tz  -- EXCLUDED: Known Debezium limitation
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'2024-01-01',
				'2024-01-01 00:00:00',
				'2000-01-01 00:00:00',
					'00:00:00',
					'12:00:00'
					-- '12:00:00+00'  -- EXCLUDED: TIMETZ
			);`,

			// --- ZONEDTIMESTAMP (zonedtimestamp_edge_cases) ---

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2024-01-01 00:00:00+00'::timestamptz,
				'2024-06-15 12:30:45+05:30'::timestamptz,
				'2024-12-25 18:00:00-08:00'::timestamptz,
				'1970-01-01 00:00:00+00'::timestamptz,
				'2050-12-31 23:59:59+00'::timestamptz,
				'2024-01-01 00:00:00+00'::timestamptz
			);`,

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2023-07-04 12:00:00+00'::timestamptz,
				'2023-03-15 08:30:00+01:00'::timestamptz,
				'2023-11-11 22:45:30-05:00'::timestamptz,
				'1969-12-31 23:59:59+00'::timestamptz,
				'2100-01-01 00:00:00+00'::timestamptz,
				'2023-06-21 00:00:00+00'::timestamptz
			);`,

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2025-01-01 06:00:00+00'::timestamptz,
				'2025-05-20 14:15:30+09:00'::timestamptz,
				'2025-08-10 10:20:40-07:00'::timestamptz,
				'1970-01-01 12:00:00+00'::timestamptz,
				'2075-06-15 18:30:00+00'::timestamptz,
				'2025-12-31 00:00:00+00'::timestamptz
			);`,

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2023-03-15 08:30:00+00'::timestamptz,
				'2023-04-20 16:45:00+05:30'::timestamptz,
				'2023-05-25 20:15:00-08:00'::timestamptz,
				'1970-01-01 06:00:00+00'::timestamptz,
				'2080-09-10 14:20:00+00'::timestamptz,
				'2023-07-04 00:00:00+00'::timestamptz
			);`,

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2022-11-30 18:00:00+00'::timestamptz,
				'2022-12-10 22:30:00+10:00'::timestamptz,
				'2022-09-05 14:45:00-06:00'::timestamptz,
				'1970-01-02 12:00:00+00'::timestamptz,
				'2090-03-20 10:15:00+00'::timestamptz,
				'2022-01-01 00:00:00+00'::timestamptz
			);`,

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'2024-01-01 00:00:00-05',
				'1970-01-01 00:00:00+00',
				'2099-12-31 23:59:59+00'
			);`,

			// --- INTERVAL (interval_edge_cases) ---

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'1 year 2 months 3 days'::interval,
				'-1 year -2 months'::interval,
				'00:00:00'::interval,
				'5 years'::interval,
				'100 days'::interval,
				'12:30:45'::interval,
				'1 year 6 months 15 days 8 hours 30 minutes'::interval
			);`,

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'3 months 7 days'::interval,
				'-5 days -3 hours'::interval,
				'0 seconds'::interval,
				'10 years'::interval,
				'365 days'::interval,
				'23:59:59'::interval,
				'2 years 3 months 10 days 5 hours'::interval
			);`,

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'6 months'::interval,
				'-1 month -1 day'::interval,
				'0'::interval,
				'1 year'::interval,
				'1 day'::interval,
				'1:00:00'::interval,
				'1 month 1 day 1 hour 1 minute 1 second'::interval
			);`,

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'8 months 15 days'::interval,
				'-10 days'::interval,
				'0 hours'::interval,
				'5 years'::interval,
				'100 days'::interval,
				'18:30:45'::interval,
				'4 years 6 months 20 days 12 hours'::interval
			);`,

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'2 years'::interval,
				'-2 months -5 days'::interval,
				'0 minutes'::interval,
				'20 years'::interval,
				'500 days'::interval,
				'6:15:30'::interval,
				'3 years 4 months 5 days 6 hours 7 minutes'::interval
			);`,

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'0 seconds',
				'1 year',
				'1 day',
				'1 hour',
				'1 year 1 day'
			);`,

			// ======================================================================
			// IDENTIFIER TYPES
			// ======================================================================

			// --- UUID/LTREE (uuid_ltree_edge_cases) ---

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
				'00000000-0000-0000-0000-000000000000',
				'ffffffff-ffff-ffff-ffff-ffffffffffff',
				'f47ac10b-58cc-4372-a567-0e02b2c3d479',
				'Top.Science.Astronomy',
				'Top.ScienceFiction.Books',
				'Top.Science.Astronomy.Stars.Sun',
				'Top'
			);`,

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'550e8400-e29b-41d4-a716-446655440000',
				'00000000-0000-0000-0000-000000000001',
				'fffffffe-ffff-ffff-ffff-ffffffffffff',
				'6ba7b810-9dad-11d1-80b4-00c04fd430c8',
				'Animals.Mammals.Primates',
				'Products.HomeAppliances.Kitchen',
				'Geography.Continents.Europe.Countries.France.Cities.Paris',
				'Root'
			);`,

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'123e4567-e89b-12d3-a456-426614174000',
				'10000000-0000-0000-0000-000000000000',
				'efffffff-ffff-ffff-ffff-ffffffffffff',
				'00000000-0000-0000-0000-000000000000',
				'Data.Users.Profiles',
				'Items.SpecialCharacters.Test',
				'A.B.C.D.E.F.G.H.I.J',
				'Leaf'
			);`,

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'550e8400-e29b-41d4-a716-446655440000',
				'00000000-0000-0000-0000-000000000001',
				'fffffffe-ffff-ffff-ffff-ffffffffffff',
				'c73bcdcc-2669-4bf6-81d3-e4ae73fb11fd',
				'Root.Branch.Leaf',
				'Category.SubCategory.Item',
				'Level1.Level2.Level3.Level4.Level5',
				'Single'
			);`,

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'6ba7b810-9dad-11d1-80b4-00c04fd430c8',
				'00000000-0000-0000-0000-000000000002',
				'fffffffd-ffff-ffff-ffff-ffffffffffff',
				'9b3e4d5c-1a2b-3c4d-5e6f-7a8b9c0d1e2f',
				'Company.Department.Team',
				'Product.Feature.Component',
				'Path.To.A.Very.Deep.Node.In.Tree',
				'Root'
			);`,

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				ltree_simple,
				ltree_deep,
				ltree_quoted
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'Top.Science',
				'Top.Science.Astronomy',
				'Top.Collections.Art'
			);`,

			// ======================================================================
			// MAP TYPE
			// ======================================================================

			// --- HSTORE (map_edge_cases) ---

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"key1" => "value1"',
				'"key=>val" => "test"',
				'"key" => "it''s"',
				'"" => "value"',
				'"a" => "1", "b" => "2", "c" => "3"',
				'"special" => "test@email.com"'
			);`,

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"name" => "John"',
				'"key" => "val=>test"',
				'"name" => "O''Reilly"',
				'"key" => ""',
				'"x" => "10", "y" => "20", "z" => "30"',
				'"path" => "C:\\Users\\test"'
			);`,

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"status" => "active"',
				'"arrow" => "=>"',
				'"text" => "It''s a test"',
				'"empty" => ""',
				'"one" => "1", "two" => "2"',
				'"data" => "value"'
			);`,

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"id" => "123"',
				'"formula" => "a=>b"',
				'"name" => "John''s"',
				'"blank" => ""',
				'"r" => "red", "g" => "green", "b" => "blue"',
				'"email" => "test@domain.com"'
			);`,

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"type" => "test"',
				'"map" => "key=>value"',
				'"title" => "Test''s Title"',
				'"null" => ""',
				'"first" => "1st", "second" => "2nd", "third" => "3rd"',
				'"url" => "http://example.com"'
			);`,

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				'"a"=>"b"',
				'"a"=>""',
				'"a"=>"1", "b"=>"2"',
				'"x"=>"y"'
			);`,

			// ======================================================================
			// NUMERIC TYPES
			// ======================================================================

			// --- INTEGER (integer_edge_cases) ---

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				2147483647,                             -- INT4 MAX
				-2147483648,                            -- INT4 MIN
				0,
				-1,
				9223372036854775807,                    -- BIGINT MAX
				-9223372036854775808,                   -- BIGINT MIN
				0
			);`,

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				2147483646,                             -- INT4 MAX - 1
				-2147483647,                            -- INT4 MIN + 1
				1,
				-2,
				9223372036854775806,                    -- BIGINT MAX - 1
				-9223372036854775807,                   -- BIGINT MIN + 1
				1
			);`,

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				1000000,
				-1000000,
				100,
				-100,
				1000000000000,
				-1000000000000,
				100
			);`,

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				10,
				-10,
				5,
				-5,
				1000,
				-1000,
				5
			);`,

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				1048576,                                -- 2^20
				-1048576,
				2,
				-2,
				1152921504606846976,                    -- 2^60
				-1152921504606846976,
				2
			);`,

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				0,
				-1,
				123456789,
				-123456789,
				0
			);`,

			// --- DECIMAL (decimal_edge_cases) ---

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				123456789.123456789,
				-123.456,
				0.00,
				123.456789012345,
				202020.292920000,
				99.99
			);`,

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				987654321.987654321,
				-999.999,
				0,
				999.999999999999999,
				2.3232323,
				-50.25
			);`,

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				1.000000001,
				-0.001,
				0.0,
				0.000000000000001,
				99999999999.999,
				12.34
			);`,

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				555666777.888999,
				-789.012,
				0.000,
				456.789012345678,
				101010.101010,
				55.55
			);`,

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				987654321.123456,
				-999.999,
				0,
				0.123456789012345,
				888888.888,
				77.77
			);`,

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				NULL,                                   -- will be set to non-NULL then back to NULL
				NULL,                                   -- will be set to non-NULL then back to NULL
				0,
				0.000000000000001,
				999999.999,
				99.99
			);`,

			// --- BOOLEAN (boolean_edge_cases) ---

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (TRUE, FALSE);`,

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (FALSE, TRUE);`,

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (TRUE, NULL);`,

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (FALSE, NULL);`,

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (NULL, TRUE);`,

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (NULL, NULL);`,
		},
		SourceDeltaSQL: []string{
			// ======================================================================
			// TEXT TYPES
			// ======================================================================

			// --- STRING (string_edge_cases) ---

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'streaming\path',                       -- backslash path
				'streaming''s test',                    -- single quote
				'first' || E'\u2028' || 'second',       -- Unicode line separator in streaming
				'word' || E'\u200B' || 'word',          -- Zero-width space in streaming
				'text' || E'\u00A0' || 'text',          -- Non-breaking space in streaming
				'D:\streaming\path',                    -- Windows path
				'''; DELETE FROM test',                 -- SQL injection
				'streaming 数据',                        -- Chinese
				'',                                     -- empty
				'NULL'                                  -- NULL literal
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES
			(
				'another\path\to\文件',                  -- Unicode with backslash (Chinese)
				'café''s specialty Ñoño',               -- Unicode with single quote
				E'first\nsecond\nthird',                -- Multiple actual newlines (E-string)
				E'a\tb\tc\td',                          -- Multiple actual tabs (E-string)
				E'mix: ''"\\\n\t\r',                    -- All special chars with actual control chars
				'C:\path',                              -- Windows path
				'--sql',                                -- SQL comment
				'مرحبا Hello مرحبا',                     -- Bidirectional text
				E'\n',                                  -- Newline only
				'NULL'                                  -- NULL literal
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES (
				'literal\nstring',          -- Two chars: backslash + n
				'test',
				E'actual\nbyte',            -- Actual newline byte (0x0A)
				'test',
				'literal\nmixed',           -- Mix with literal \n
				'C:\new\test',              -- Path with literal \n
				'test',
				'test',
				'',
				'NULL'
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES (
				NULL,                       -- INSERT with NULL during streaming
				'value with quote',
				NULL,                       -- INSERT with NULL during streaming
				E'has\ttab',
				NULL,
				'D:\path\test',
				NULL,
				NULL,
				'',
				'not null string'
			);`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'updated: \x\y\z',
			    text_with_quote = 'updated''s value'
			WHERE id = 1;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_quote = 'O''''Reilly',
			    text_sql_injection = '''; DROP TABLE users--'
			WHERE id = 2;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'updated\''s test',
			    text_unicode = '🎉 emoji test 日本',
			    text_empty = ' '
			WHERE id = 3;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'updated\path\数据',
			    text_with_quote = 'Hello مرحبا world',
			    text_with_newline = 'Mixed 世界 test 🌏',
			    text_unicode = '👨‍👩‍👧‍👦 emoji family'
			WHERE id = 2;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_newline = 'updated' || E'\u2028' || 'line',
			    text_with_tab = 'updated' || E'\u2029' || 'para',
			    text_with_mixed = 'zero' || E'\u200B' || 'width',
			    text_windows_path = 'nbsp' || E'\u00A0' || 'here'
			WHERE id = 1;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_newline = E'new\nline\ntest',
			    text_with_tab = E'new\ttab\ttest',
			    text_with_mixed = E'It''s "test" with \n\t\r',
			    text_empty = E' \t\n\r '
			WHERE id = 2;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_quote = NULL,
				text_empty = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'set from NULL',
				text_with_quote = 'also set from NULL'
			WHERE id = 6 AND text_with_backslash IS NULL;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'updated\nliteral',     -- Two chars: backslash + n
				text_windows_path = 'E:\new\updated\path',    -- More literal \n in path
				text_with_mixed = 'literal\nupdated\nvalue'   -- Multiple literal \n
			WHERE id = 4;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_newline = E'updated\nactual\nnewline',  -- Actual newline bytes (0x0A)
				text_with_tab = E'and\nsome\ntabs\there'           -- Mix actual newlines and tabs
			WHERE id = 4;`,

			`DELETE FROM test_schema.string_edge_cases WHERE id = 3;`,

			// --- JSON/JSONB (json_edge_cases) ---

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES
			(
				'{"streaming": "value with backslash\\\\ test"}',
				'{"stream": "数据流 🚀"}',
				'{"new": {"nested": "stream"}}',
				'["stream1", "stream2"]',
				'{"stream": null}',
				'{}',
				'{"stream": "formatted"}',
				'{"count": 999}',
				'{"streaming": true, "data": "test"}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES (
				'{"author": "O''Reilly", "title": "It''s a book"}',
				'{"company": "O''Neill", "slogan": "We''re the best"}',
				'{"person": "O''Brien", "nested": {"quote": "It''s nested"}}',
				'["It''s working", "O''Reilly''s guide", "We''re here"]',
				'{"name": "O''Connor", "value": null}',
				'{}',
				'{"quote": "She said ''hello''"}',
				'{"count": 123}',
				'{"author": "O''Reilly", "books": ["It''s great", "We''re learning"]}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES (
				NULL,                                     -- INSERT with NULL during streaming
				'{"lang": "español"}',
				NULL,                                     -- INSERT with NULL during streaming
				'["item1", "item2"]',
				NULL,
				'{}',
				NULL,
				'{"num": 777}',
				NULL
			);`,

			`UPDATE test_schema.json_edge_cases
			SET json_with_escaped_chars = '{"updated": "simple value"}',
			    json_with_unicode = '{"updated": "café 世界 🎉"}',
			    json_nested = '{"updated": {"deep": {"nesting": "value"}}}'
			WHERE id = 1;`,

			`UPDATE test_schema.json_edge_cases
			SET json_empty = '{"now": "not_empty"}',
			    json_with_null = '{"was": null, "now": "value"}',
			    json_array = '[1, 2, 3, 4, 5]'
			WHERE id = 2;`,

			`UPDATE test_schema.json_edge_cases
			SET json_with_unicode = NULL,
				json_nested = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.json_edge_cases
			SET json_with_escaped_chars = '{"from": "NULL"}',
				json_with_unicode = '{"also": "from NULL"}'
			WHERE id = 6 AND json_with_escaped_chars IS NULL;`,

			`UPDATE test_schema.json_edge_cases
			SET json_with_escaped_chars = '{"updated": "O''Sullivan", "note": "It''s updated"}',
				json_array = '["Updated''s test", "O''Reilly''s updated", "We''re updating"]',
				json_complex = '{"author": "O''Brien", "items": ["She''s here", "It''s working"]}'
			WHERE id = 4;`,

			`DELETE FROM test_schema.json_edge_cases WHERE id = 3;`,

			// --- ENUM (enum_edge_cases) ---

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'active',
				'enum''value',
				'with-dash',
				'🎉emoji',
				ARRAY[]::test_schema.status_enum[],  -- INSERT with EMPTY array
				NULL
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				'pending',
				'enum\value',
				'with_underscore',
				'123start',
				ARRAY[NULL, 'active', NULL]::test_schema.status_enum[],  -- INSERT with NULL elements in array
				'active'
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES
			(
				NULL,                                                      -- INSERT with NULL during streaming
				'enum"value',
				NULL,                                                      -- INSERT with NULL during streaming
				'café',
				ARRAY['active', 'pending']::test_schema.status_enum[],
				NULL
			);`,

			`UPDATE test_schema.enum_edge_cases
			SET status_simple = 'pending',
			    status_with_quote = 'enum"value',
			    status_unicode = '123start'
			WHERE id = 1;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY['enum\value', 'with_underscore', 'with space']::test_schema.status_enum[],
				status_null = 'inactive'
			WHERE id = 2;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY[]::test_schema.status_enum[]
			WHERE id = 1;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY['pending', NULL, NULL, 'inactive']::test_schema.status_enum[]
			WHERE id = 4;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY['inactive', 'with space', 'with-dash', 'pending', 'active', 'café']::test_schema.status_enum[]
			WHERE id = 5;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_simple = NULL,
				status_with_quote = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY['pending']::test_schema.status_enum[]
			WHERE id = 1 AND status_array = ARRAY[]::test_schema.status_enum[];`,

			`UPDATE test_schema.enum_edge_cases
			SET status_simple = 'active',
				status_with_quote = 'inactive'
			WHERE id = 6 AND status_simple IS NULL;`,

			`DELETE FROM test_schema.enum_edge_cases WHERE id = 3;`,

			// ======================================================================
			// BINARY TYPE
			// ======================================================================

			// --- BYTES (bytes_edge_cases) ---

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				E'\\x',
				E'\\x42',
				E'\\x53747265616d',
				E'\\x0000',
				E'\\x00000000',
				E'\\xffffffff',
				E'\\x5c27',
				E'\\x0123456789abcdef'
			);`,

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES
			(
				NULL,                    -- INSERT with NULL during streaming
				E'\\xff',
				NULL,                    -- INSERT with NULL during streaming
				E'\\x0041',
				E'\\x0000',
				NULL,
				NULL,
				E'\\xabcdef01'
			);`,

			`UPDATE test_schema.bytes_edge_cases
			SET bytes_single = E'\\xaa',
			    bytes_ascii = E'\\x557064617465',
			    bytes_mixed = E'\\xfeedface'
			WHERE id = 1;`,

			`UPDATE test_schema.bytes_edge_cases
			SET bytes_null_byte = E'\\x00ff00ff',
			    bytes_all_zeros = E'\\x0000',
			    bytes_all_ff = E'\\xffff'
			WHERE id = 2;`,

			`UPDATE test_schema.bytes_edge_cases
			SET bytes_ascii = NULL,
				bytes_mixed = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.bytes_edge_cases
			SET bytes_empty = '\x42'::bytea,
				bytes_single = '\x43'::bytea
			WHERE id = 6 AND bytes_empty IS NULL;`,

			`DELETE FROM test_schema.bytes_edge_cases WHERE id = 3;`,

			// ======================================================================
			// TEMPORAL TYPES
			// ======================================================================

			// --- DATETIME (datetime_edge_cases) ---

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
				-- time_with_tz  -- EXCLUDED: Known Debezium limitation
			) VALUES
			(
				'2023-01-15',
				'1980-03-20',
				'2030-08-10',
				'2023-01-15 10:20:30',
				'1980-03-20 15:45:00',
				'2030-08-10 20:00:00+02',
				'10:20:30',
				'15:45:00',
				'08:15:30.654321'
				-- '20:00:00+02'  -- EXCLUDED: TIMETZ
			);`,

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
			) VALUES
			(
				NULL,                              -- INSERT with NULL during streaming
				'1995-06-15',
				NULL,                              -- INSERT with NULL during streaming
				'2020-01-01 00:00:00',
				NULL,
				'2025-06-15 12:00:00+00',
				'06:00:00',
				NULL,
				'18:30:45.123456'
			);`,

			`UPDATE test_schema.datetime_edge_cases
			SET date_epoch = '2025-12-25',
			    timestamp_epoch = '2025-12-25 18:30:00',
			    time_midnight = '01:02:03'
			    -- time_with_tz = '01:02:03+01'  -- EXCLUDED: Known Debezium limitation
			WHERE id = 1;`,

			`UPDATE test_schema.datetime_edge_cases
			SET date_future = '2099-01-01',
			    timestamp_with_tz = '2099-01-01 00:00:00-08',
			    time_with_micro = '12:34:56.789012'
			    -- time_with_tz = '00:00:00-08'  -- EXCLUDED: Known Debezium limitation
			WHERE id = 2;`,

			`UPDATE test_schema.datetime_edge_cases
			SET timestamp_epoch = NULL,
				timestamp_with_tz = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.datetime_edge_cases
			SET date_epoch = '2025-01-01',
				date_negative = '2000-01-01'
			WHERE id = 6 AND date_epoch IS NULL;`,

			`DELETE FROM test_schema.datetime_edge_cases WHERE id = 3;`,

			// --- ZONEDTIMESTAMP (zonedtimestamp_edge_cases) ---

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				'2024-08-20 15:45:30+00'::timestamptz,
				'2024-09-10 09:15:00+03:00'::timestamptz,
				'2024-10-05 20:30:15-06:00'::timestamptz,
				'1970-01-02 00:00:00+00'::timestamptz,
				'2060-05-15 12:00:00+00'::timestamptz,
				'2024-07-01 00:00:00+00'::timestamptz
			);`,

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES
			(
				NULL,                                      -- INSERT with NULL during streaming
				'2026-01-15 08:00:00+02:00'::timestamptz,
				NULL,                                      -- INSERT with NULL during streaming
				'1970-01-01 00:00:00+00'::timestamptz,
				NULL,
				'2026-01-01 00:00:00+00'::timestamptz
			);`,

			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_utc = '2024-02-14 10:30:00+00'::timestamptz,
			    ts_positive_offset = '2024-03-20 16:45:00+08:00'::timestamptz
			WHERE id = 1;`,

			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_negative_offset = '2023-09-30 11:11:11-04:00'::timestamptz,
			    ts_future = '2090-12-31 23:59:59+00'::timestamptz
			WHERE id = 2;`,

			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_negative_offset = NULL,
				ts_future = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_utc = '2025-06-15 12:00:00+00',
				ts_positive_offset = '2025-06-15 12:00:00+05'
			WHERE id = 6 AND ts_utc IS NULL;`,

			`DELETE FROM test_schema.zonedtimestamp_edge_cases WHERE id = 3;`,

			// --- INTERVAL (interval_edge_cases) ---

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				'2 years 5 months'::interval,
				'-10 days'::interval,
				'0 minutes'::interval,
				'50 years'::interval,
				'7 days'::interval,
				'6:15:30'::interval,
				'3 years 2 months 20 days 10 hours'::interval
			);`,

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES
			(
				NULL,                             -- INSERT with NULL during streaming
				'-5 days'::interval,
				NULL,                             -- INSERT with NULL during streaming
				'10 years'::interval,
				NULL,
				'12:00:00'::interval,
				'1 year 6 months 15 days'::interval
			);`,

			`UPDATE test_schema.interval_edge_cases
			SET interval_positive = '8 months 15 days'::interval,
			    interval_years = '25 years'::interval
			WHERE id = 1;`,

			`UPDATE test_schema.interval_edge_cases
			SET interval_negative = '-3 months -7 days'::interval,
			    interval_mixed = '5 months 10 days 2 hours 30 minutes'::interval
			WHERE id = 2;`,

			`UPDATE test_schema.interval_edge_cases
			SET interval_zero = NULL,
				interval_days = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.interval_edge_cases
			SET interval_positive = '5 years',
				interval_negative = '-10 days'
			WHERE id = 6 AND interval_positive IS NULL;`,

			`DELETE FROM test_schema.interval_edge_cases WHERE id = 3;`,

			// ======================================================================
			// IDENTIFIER TYPES
			// ======================================================================

			// --- UUID/LTREE (uuid_ltree_edge_cases) ---

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				'7c9e6679-7425-40de-944b-e07fc1f90ae7',
				'00000000-0000-0000-0000-000000000002',
				'fffffffd-ffff-ffff-ffff-ffffffffffff',
				'9b2c8f5d-1234-5678-9abc-def012345678',
				'Stream.Data.Live',
				'Test.StreamingPath.Values',
				'Deep.Path.To.Stream.Data.Node',
				'Stream'
			);`,

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES
			(
				NULL,                                     -- INSERT with NULL during streaming
				'00000000-0000-0000-0000-000000000005',
				NULL,                                     -- INSERT with NULL during streaming
				'3f7a8b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c',
				'Test.Null.Insert',
				NULL,
				'Path.With.Null.Values',
				NULL
			);`,

			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_standard = 'c2a9c8d0-1234-5678-9abc-def123456789',
			    ltree_simple = 'Updated.Path.Node'
			WHERE id = 1;`,

			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_all_zeros = '00000000-0000-0000-0000-000000000003',
			    ltree_deep = 'Very.Deep.Path.With.Many.Levels.To.Test'
			WHERE id = 2;`,

			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_random = NULL,
				ltree_quoted = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_standard = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
				uuid_all_zeros = '00000000-0000-0000-0000-000000000000'
			WHERE id = 6 AND uuid_standard IS NULL;`,

			`DELETE FROM test_schema.uuid_ltree_edge_cases WHERE id = 3;`,

			// ======================================================================
			// MAP TYPE
			// ======================================================================

			// --- HSTORE (map_edge_cases) ---

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				'"stream" => "data"',
				'"test=>key" => "value"',
				'"quote" => "test''s"',
				'"" => "empty"',
				'"s1" => "v1", "s2" => "v2"',
				'"special" => "data"'
			);`,

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES
			(
				NULL,                                  -- INSERT with NULL during streaming
				'"key=>val" => "test"',
				NULL,                                  -- INSERT with NULL during streaming
				'"empty" => ""',
				NULL,
				'"test" => "value"'
			);`,

			`UPDATE test_schema.map_edge_cases
			SET map_simple = '"updated" => "value"',
			    map_with_arrow = '"arrow=>test" => "updated"'
			WHERE id = 1;`,

			`UPDATE test_schema.map_edge_cases
			SET map_with_quotes = '"name" => "O''Brien"',
			    map_multiple_pairs = '"x" => "100", "y" => "200"'
			WHERE id = 2;`,

			`UPDATE test_schema.map_edge_cases
			SET map_empty_values = NULL,
				map_multiple_pairs = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.map_edge_cases
			SET map_simple = 'from_null=>yes',
				map_with_arrow = 'also=>from_null'
			WHERE id = 6 AND map_simple IS NULL;`,

			`DELETE FROM test_schema.map_edge_cases WHERE id = 3;`,

			// ======================================================================
			// NUMERIC TYPES
			// ======================================================================

			// --- INTEGER (integer_edge_cases) ---

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				2147483646,                         -- Near INT4 MAX
				-2147483647,                        -- Near INT4 MIN
				42,
				-42,
				9223372036854775806,                -- Near BIGINT MAX
				-9223372036854775807,               -- Near BIGINT MIN
				42
			);`,

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				NULL,                               -- INSERT with NULL during streaming
				-1000000,
				NULL,                               -- INSERT with NULL during streaming
				-50,
				5000000000000,
				NULL,
				0
			);`,

			`UPDATE test_schema.integer_edge_cases
			SET int_max = 2147483645,               -- INT4 MAX - 2
				bigint_max = 9223372036854775805    -- BIGINT MAX - 2
			WHERE id = 1;`,

			`UPDATE test_schema.integer_edge_cases
			SET int_zero = -1,
				int_negative_one = 1,
				bigint_zero = -999
			WHERE id = 2;`,

			`UPDATE test_schema.integer_edge_cases
			SET bigint_max = NULL,
				bigint_min = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.integer_edge_cases
			SET int_max = 999999,
				int_min = -999999
			WHERE id = 6 AND int_max IS NULL;`,

			`DELETE FROM test_schema.integer_edge_cases WHERE id = 3;`,

			// --- DECIMAL (decimal_edge_cases) ---

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				555555555.555555555,
				-777.777,
				0.000,
				888.888888888888888,
				100.500000,
				75.50
			);`,

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES
			(
				NULL,                               -- INSERT with NULL during streaming
				-999.999,
				NULL,                               -- INSERT with NULL during streaming
				0.000000000000001,
				NULL,
				50.00
			);`,

			`UPDATE test_schema.decimal_edge_cases
			SET decimal_large = 999999999.999999999,
			    decimal_negative = -1000.001
			WHERE id = 1;`,

			`UPDATE test_schema.decimal_edge_cases
			SET decimal_high_precision = 0.123456789012345,
			    decimal_scientific = 3.1415926000
			WHERE id = 2;`,

			`UPDATE test_schema.decimal_edge_cases
			SET decimal_negative = NULL,
				decimal_small = NULL
			WHERE id = 5;`,

			`UPDATE test_schema.decimal_edge_cases
			SET decimal_large = 111111111.111111111,
				decimal_negative = -111.111
			WHERE id = 6 AND decimal_large IS NULL;`,

			`DELETE FROM test_schema.decimal_edge_cases WHERE id = 3;`,

			// --- BOOLEAN (boolean_edge_cases) ---

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (TRUE, TRUE);`,

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (NULL, TRUE);`,

			`UPDATE test_schema.boolean_edge_cases SET bool_simple = FALSE, bool_nullable = TRUE WHERE id = 1;`,

			`UPDATE test_schema.boolean_edge_cases SET bool_simple = TRUE, bool_nullable = FALSE WHERE id = 2;`,

			`UPDATE test_schema.boolean_edge_cases SET bool_simple = NULL, bool_nullable = NULL WHERE id = 5;`,

			`UPDATE test_schema.boolean_edge_cases SET bool_simple = TRUE, bool_nullable = TRUE WHERE id = 6 AND bool_simple IS NULL;`,

			`DELETE FROM test_schema.boolean_edge_cases WHERE id = 3;`,
		},
		TargetDeltaSQL: []string{
			// ======================================================================
			// TEXT TYPES
			// ======================================================================

			// --- STRING (string_edge_cases) ---

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES (
				'fb\path\to\文件',
				'café''s fb Ñoño',
				E'fb\nline\nstream',
				E'fb\ttab\tstream',
				E'fb: ''"\\\n\t\r',
				'C:\fb',
				'--fb',
				'مرحبا fb مرحبا',
				E'\n',
				'NULL'
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES (
				'fallback_literal\ntest',   -- Two chars: backslash + n
				'fallback',
				E'fallback_actual\ntest',   -- Actual newline byte (0x0A)
				'fallback',
				'fallback_literal\nmixed',  -- Mix with literal \n
				'D:\new\path',              -- Path with literal \n
				'fallback',
				'fallback',
				'',
				'NULL'
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES (
				'MARKER_LITERAL_N',         -- Unique marker for UPDATE matching
				'test',
				'test',
				'test',
				'test',
				'test',
				'test',
				'test',
				'',
				'NULL'
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES (
				'test',
				'test',
				'MARKER_ACTUAL_NEWLINE',    -- Unique marker for UPDATE matching
				'test',
				'test',
				'test',
				'test',
				'test',
				'',
				'NULL'
			);`,

			`INSERT INTO test_schema.string_edge_cases (
				text_with_backslash,
				text_with_quote,
				text_with_newline,
				text_with_tab,
				text_with_mixed,
				text_windows_path,
				text_sql_injection,
				text_unicode,
				text_empty,
				text_null_string
			) VALUES (
				NULL,                       -- INSERT with NULL during streaming (fallback)
				'value with quote',
				NULL,                       -- INSERT with NULL during streaming (fallback)
				E'has\ttab',
				NULL,
				'D:\path\test',
				NULL,
				NULL,
				'',
				'not null string'
			);`,

			`UPDATE test_schema.string_edge_cases 
			SET text_with_backslash = '\\network\fb',
				text_with_quote = 'café''s updated Ñoño',
				text_unicode = 'مرحبا fb 世界'
			WHERE id = 1;`,

			`UPDATE test_schema.string_edge_cases 
			SET text_with_newline = E'fb\nnew\nlines',
				text_with_tab = E'fb\nnew\ttabs',
				text_with_mixed = E'fb: ''"\\\n\t\r'
			WHERE id = 2;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_quote = 'fallback from NULL',
				text_empty = 'fallback also from NULL'
			WHERE id = 5 AND text_with_quote IS NULL;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = NULL,
				text_with_quote = NULL
			WHERE id = 6;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_newline = 'fb' || E'\u2028' || 'line',   -- Unicode line separator (U+2028)
				text_with_tab = 'fb' || E'\u2029' || 'para',       -- Unicode paragraph separator (U+2029)
				text_with_mixed = 'fb' || E'\u200B' || 'zero',     -- Zero-width space (U+200B)
				text_windows_path = 'fb' || E'\u00A0' || 'nbsp'    -- Non-breaking space (U+00A0)
			WHERE id = 1;`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_backslash = 'fb_updated\nliteral',
				text_windows_path = 'F:\fallback\new\path',
				text_with_mixed = 'fb_literal\nupdated'
			WHERE text_with_backslash = 'MARKER_LITERAL_N';`,

			`UPDATE test_schema.string_edge_cases
			SET text_with_newline = E'fb_updated\nactual\nbyte',
				text_with_tab = E'fb\nnewlines\ttabs',
				text_with_quote = E'fb_actual\nnewline\ntest'
			WHERE text_with_newline = 'MARKER_ACTUAL_NEWLINE';`,

			`DELETE FROM test_schema.string_edge_cases WHERE id = 4;`,

			// --- JSON/JSONB (json_edge_cases) ---

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES (
				'{"fallback": "test value", "quotes": "O''Reilly''s"}',
				'{"fallback": "مرحبا", "emoji": "🔄", "chinese": "你好"}',
				'{"fallback": {"nested": {"deep": {"value": "test"}}}}',
				'[["fallback"], ["nested", "array"]]',
				'{"fallback": null, "also_null": null}',
				'{"empty": {}}',
				'{"fallback": "formatted value", "number": 123}',
				'{"neg": -999, "zero": 0, "pos": 999, "decimal": 123.456}',
				'{"unicode": "café ñoño", "escaped": "test", "data": "fallback"}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES (
				'{"fallback": "O''Malley", "text": "It''s fallback"}',
				'{"name": "O''Donnell", "message": "We''re testing"}',
				'{"person": "O''Brien", "quote": "He said ''hello''"}',
				'["Fallback''s test", "O''Reilly''s book"]',
				'{"author": "O''Neil", "value": null}',
				'{}',
				'{"fallback": "She''s here"}',
				'{"num": 456}',
				'{"fallback": "O''Connor", "items": ["It''s good", "We''re done"]}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES (
				'{"marker": "JSON_SINGLE_QUOTES"}',  -- Unique marker for UPDATE matching
				'{"test": "value"}',
				'{"test": "nested"}',
				'["test"]',
				'{"test": null}',
				'{}',
				'{"test": "formatted"}',
				'{"num": 1}',
				'{"test": "complex"}'
			);`,

			`INSERT INTO test_schema.json_edge_cases (
				json_with_escaped_chars,
				json_with_unicode,
				json_nested,
				json_array,
				json_with_null,
				json_empty,
				json_formatted,
				json_with_numbers,
				json_complex
			) VALUES (
				NULL,                                     -- INSERT with NULL during streaming (fallback)
				'{"lang": "español"}',
				NULL,                                     -- INSERT with NULL during streaming (fallback)
				'["item1", "item2"]',
				NULL,
				'{}',
				NULL,
				'{"num": 777}',
				NULL
			);`,

			`UPDATE test_schema.json_edge_cases
			SET json_with_escaped_chars = '{"updated": "test value"}',
				json_with_unicode = '{"updated": "日本語🎉", "korean": "한글"}',
				json_nested = '{"updated": {"level": 2, "nested": true}}'
			WHERE id = 1;`,

			`UPDATE test_schema.json_edge_cases
			SET json_array = '["updated", "array", "values"]',
				json_complex = '{"updated": true, "number": 42}'
			WHERE id = 2;`,

			`UPDATE test_schema.json_edge_cases
			SET json_with_unicode = '{"fallback": "from NULL", "unicode": "restored"}',
				json_nested = '{"fallback": "also from NULL", "restored": true}'
			WHERE id = 5 AND json_with_unicode IS NULL;`,

			`UPDATE test_schema.json_edge_cases
			SET json_with_escaped_chars = NULL,
				json_with_unicode = NULL
			WHERE id = 6;`,

			`UPDATE test_schema.json_edge_cases
			SET json_with_escaped_chars = '{"fb_updated": "O''Connell", "note": "It''s fallback"}',
				json_array = '["Fallback''s updated", "O''Reilly''s fallback"]',
				json_complex = '{"fb_author": "O''Sullivan", "items": ["She''s testing", "It''s done"]}'
			WHERE json_with_escaped_chars::text = '{"marker": "JSON_SINGLE_QUOTES"}';`,

			`DELETE FROM test_schema.json_edge_cases WHERE id = 4;`,

			// --- ENUM (enum_edge_cases) ---

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES (
				'pending',
				'enum"value',
				'with space',
				'café',
				ARRAY[]::test_schema.status_enum[],  -- FALLBACK: INSERT with EMPTY array
				'active'
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES (
				'inactive',
				'enum''value',
				'with-dash',
				'🎉emoji',
				ARRAY[NULL, 'pending', NULL, 'inactive']::test_schema.status_enum[],  -- FALLBACK: INSERT with NULL elements
				NULL
			);`,

			`INSERT INTO test_schema.enum_edge_cases (
				status_simple,
				status_with_quote,
				status_with_special,
				status_unicode,
				status_array,
				status_null
			) VALUES (
				NULL,                                                      -- INSERT with NULL during streaming (fallback)
				'enum"value',
				NULL,                                                      -- INSERT with NULL during streaming (fallback)
				'café',
				ARRAY['active', 'pending']::test_schema.status_enum[],
				NULL
			);`,

			`UPDATE test_schema.enum_edge_cases
			SET status_simple = 'inactive',
				status_with_quote = 'enum''value',
				status_with_special = 'with_underscore',
				status_array = ARRAY[]::test_schema.status_enum[]  -- FALLBACK: Set to EMPTY array
			WHERE id = 1;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_unicode = '🎉emoji',
				status_array = ARRAY['with-dash', 'enum\value']::test_schema.status_enum[]  -- FALLBACK: Set EMPTY to NON-EMPTY
			WHERE id = 2;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY[NULL, 'active', NULL, 'pending', NULL]::test_schema.status_enum[]  -- FALLBACK: Add multiple NULLs
			WHERE id = 4;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY['active', 'inactive', 'pending', 'with space', '🎉emoji']::test_schema.status_enum[]  -- FALLBACK: Add elements to array (expand)
			WHERE id = 5;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_simple = 'with-dash',
				status_with_quote = 'with space'
			WHERE id = 5 AND status_simple IS NULL;`,

			`UPDATE test_schema.enum_edge_cases
			SET status_array = ARRAY['inactive']::test_schema.status_enum[]  -- FALLBACK: Remove elements from array (shrink to single element)
			WHERE id = 1 AND status_array = ARRAY[]::test_schema.status_enum[];`,

			`UPDATE test_schema.enum_edge_cases
			SET status_simple = NULL,
				status_with_quote = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.enum_edge_cases WHERE id = 4;`,

			// ======================================================================
			// BINARY TYPE
			// ======================================================================

			// --- BYTES (bytes_edge_cases) ---

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES (
				E'\\x',
				E'\\xFB',
				E'\\x46616C6C6261636B',
				E'\\xFF00',
				E'\\x0000',
				E'\\xFFFF',
				E'\\x5c5c',
				E'\\xABCDEF123456'
			);`,

			`INSERT INTO test_schema.bytes_edge_cases (
				bytes_empty,
				bytes_single,
				bytes_ascii,
				bytes_null_byte,
				bytes_all_zeros,
				bytes_all_ff,
				bytes_special_chars,
				bytes_mixed
			) VALUES (
				NULL,                    -- INSERT with NULL during streaming (fallback)
				E'\\xff',
				NULL,                    -- INSERT with NULL during streaming (fallback)
				E'\\x0041',
				E'\\x0000',
				NULL,
				NULL,
				E'\\xabcdef01'
			);`,

			`UPDATE test_schema.bytes_edge_cases
			SET bytes_single = E'\\xBB',
				bytes_ascii = E'\\x75706461746564',
				bytes_mixed = E'\\xDEADC0DE'
			WHERE id = 1;`,

			`UPDATE test_schema.bytes_edge_cases
			SET bytes_null_byte = E'\\xFF00FF00',
				bytes_all_zeros = E'\\x00',
				bytes_all_ff = E'\\xFF'
			WHERE id = 2;`,

			`UPDATE test_schema.bytes_edge_cases
			SET bytes_ascii = E'\\x6661696C6261636B',
				bytes_mixed = E'\\xCAFEBABE'
			WHERE id = 5 AND bytes_ascii IS NULL;`,

			`UPDATE test_schema.bytes_edge_cases
			SET bytes_empty = NULL,
				bytes_single = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.bytes_edge_cases WHERE id = 4;`,

			// ======================================================================
			// TEMPORAL TYPES
			// ======================================================================

			// --- DATETIME (datetime_edge_cases) ---

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
				-- time_with_tz  -- EXCLUDED: Known Debezium limitation
			) VALUES (
				'2024-06-15',
				'1975-05-20',
				'2035-09-25',
				'2024-06-15 14:30:45',
				'1975-05-20 08:15:30',
				'2035-09-25 16:45:00+03',
				'14:30:45',
				'08:15:30',
				'16:45:30.123456'
				-- '16:45:00+03'  -- EXCLUDED: TIMETZ
			);`,

			`INSERT INTO test_schema.datetime_edge_cases (
				date_epoch,
				date_negative,
				date_future,
				timestamp_epoch,
				timestamp_negative,
				timestamp_with_tz,
				time_midnight,
				time_noon,
				time_with_micro
			) VALUES (
				NULL,                              -- INSERT with NULL during streaming (fallback)
				'1995-06-15',
				NULL,                              -- INSERT with NULL during streaming (fallback)
				'2020-01-01 00:00:00',
				NULL,
				'2025-06-15 12:00:00+00',
				'06:00:00',
				NULL,
				'18:30:45.123456'
			);`,

			`UPDATE test_schema.datetime_edge_cases
			SET date_epoch = '2026-11-20',
				timestamp_epoch = '2026-11-20 09:15:45',
				time_midnight = '02:03:04'
				-- time_with_tz = '02:03:04-05'  -- EXCLUDED: Known Debezium limitation
			WHERE id = 1;`,

			`UPDATE test_schema.datetime_edge_cases
			SET date_future = '2098-06-15',
				timestamp_with_tz = '2098-06-15 12:00:00-06',
				time_with_micro = '18:45:30.654321'
				-- time_with_tz = '12:00:00-06'  -- EXCLUDED: Known Debezium limitation
			WHERE id = 2;`,

			`UPDATE test_schema.datetime_edge_cases
			SET timestamp_epoch = '2026-06-01 10:20:30',
				timestamp_with_tz = '2026-06-01 10:20:30-05'
			WHERE id = 5 AND timestamp_epoch IS NULL;`,

			`UPDATE test_schema.datetime_edge_cases
			SET date_epoch = NULL,
				date_negative = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.datetime_edge_cases WHERE id = 4;`,

			// --- ZONEDTIMESTAMP (zonedtimestamp_edge_cases) ---

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES (
				'2025-05-15 10:20:30+00'::timestamptz,
				'2025-06-20 14:30:00+04:00'::timestamptz,
				'2025-07-10 18:45:15-07:00'::timestamptz,
				'1970-01-03 00:00:00+00'::timestamptz,
				'2065-08-20 18:00:00+00'::timestamptz,
				'2025-08-01 00:00:00+00'::timestamptz
			);`,

			`INSERT INTO test_schema.zonedtimestamp_edge_cases (
				ts_utc,
				ts_positive_offset,
				ts_negative_offset,
				ts_epoch,
				ts_future,
				ts_midnight
			) VALUES (
				NULL,                                      -- INSERT with NULL during streaming (fallback)
				'2026-01-15 08:00:00+02:00'::timestamptz,
				NULL,                                      -- INSERT with NULL during streaming (fallback)
				'1970-01-01 00:00:00+00'::timestamptz,
				NULL,
				'2026-01-01 00:00:00+00'::timestamptz
			);`,

			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_utc = '2025-01-01 12:00:00+00'::timestamptz,
				ts_positive_offset = '2025-02-14 06:30:00+05:30'::timestamptz
			WHERE id = 1;`,

			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_negative_offset = '2025-03-15 18:45:00-07:00'::timestamptz,
				ts_future = '2070-12-31 23:59:59.999999+00'::timestamptz
			WHERE id = 2;`,

			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_negative_offset = '2026-05-01 14:30:00-03:00'::timestamptz,
				ts_future = '2065-11-15 18:20:40+00'::timestamptz
			WHERE id = 5 AND ts_negative_offset IS NULL;`,

			`UPDATE test_schema.zonedtimestamp_edge_cases
			SET ts_utc = NULL,
				ts_positive_offset = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.zonedtimestamp_edge_cases WHERE id = 4;`,

			// --- INTERVAL (interval_edge_cases) ---

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES (
				'3 years 6 months'::interval,
				'-15 days'::interval,
				'0 seconds'::interval,
				'75 years'::interval,
				'14 days'::interval,
				'8:30:45'::interval,
				'4 years 3 months 25 days 15 hours'::interval
			);`,

			`INSERT INTO test_schema.interval_edge_cases (
				interval_positive,
				interval_negative,
				interval_zero,
				interval_years,
				interval_days,
				interval_hours,
				interval_mixed
			) VALUES (
				NULL,                             -- INSERT with NULL during streaming (fallback)
				'-5 days'::interval,
				NULL,                             -- INSERT with NULL during streaming (fallback)
				'10 years'::interval,
				NULL,
				'12:00:00'::interval,
				'1 year 6 months 15 days'::interval
			);`,

			`UPDATE test_schema.interval_edge_cases
			SET interval_positive = '9 months 20 days'::interval,
				interval_years = '30 years'::interval
			WHERE id = 1;`,

			`UPDATE test_schema.interval_edge_cases
			SET interval_negative = '-4 months -10 days'::interval,
				interval_mixed = '6 months 15 days 3 hours 45 minutes'::interval
			WHERE id = 2;`,

			`UPDATE test_schema.interval_edge_cases
			SET interval_zero = '0'::interval,
				interval_days = '45 days'::interval
			WHERE id = 5 AND interval_zero IS NULL;`,

			`UPDATE test_schema.interval_edge_cases
			SET interval_positive = NULL,
				interval_negative = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.interval_edge_cases WHERE id = 4;`,

			// ======================================================================
			// IDENTIFIER TYPES
			// ======================================================================

			// --- UUID/LTREE (uuid_ltree_edge_cases) ---

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES (
				'fb123456-7890-abcd-ef12-345678901234',
				'00000000-0000-0000-0000-000000000099',
				'fffffffe-ffff-ffff-ffff-ffffffffffff',
				'abcdef12-3456-7890-abcd-ef1234567890',
				'Fallback.Data.Test',
				'FB.TestPath.Values',
				'Fallback.Deep.Path.To.Data.Node',
				'FB'
			);`,

			`INSERT INTO test_schema.uuid_ltree_edge_cases (
				uuid_standard,
				uuid_all_zeros,
				uuid_all_fs,
				uuid_random,
				ltree_simple,
				ltree_quoted,
				ltree_deep,
				ltree_single
			) VALUES (
				NULL,                                     -- INSERT with NULL during streaming (fallback)
				'00000000-0000-0000-0000-000000000005',
				NULL,                                     -- INSERT with NULL during streaming (fallback)
				'3f7a8b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c',
				'Test.Null.Insert',
				NULL,
				'Path.With.Null.Values',
				NULL
			);`,

			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_standard = 'fb654321-0987-fedc-ba21-098765432109',
				ltree_simple = 'FB.Updated.Path'
			WHERE id = 1;`,

			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_all_zeros = '00000000-0000-0000-0000-000000000088',
				ltree_deep = 'FB.Very.Deep.Path.With.Many.Levels'
			WHERE id = 2;`,

			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_random = 'fb123456-7890-abcd-ef01-234567890abc',
				ltree_quoted = 'FB.Quoted.Path'
			WHERE id = 5 AND uuid_random IS NULL;`,

			`UPDATE test_schema.uuid_ltree_edge_cases
			SET uuid_standard = NULL,
				uuid_all_zeros = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.uuid_ltree_edge_cases WHERE id = 4;`,

			// ======================================================================
			// MAP TYPE
			// ======================================================================

			// --- HSTORE (map_edge_cases) ---

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES (
				'"fallback" => "data"',
				'"fb=>key" => "testval"',
				'"fb" => "O''Reilly"',
				'"" => "fb"',
				'"fb1" => "v1", "fb2" => "v2", "fb3" => "v3"',
				'"special" => "test@fb.com"'
			);`,

			`INSERT INTO test_schema.map_edge_cases (
				map_simple,
				map_with_arrow,
				map_with_quotes,
				map_empty_values,
				map_multiple_pairs,
				map_special_chars
			) VALUES (
				NULL,                                  -- INSERT with NULL during streaming (fallback)
				'"key=>val" => "test"',
				NULL,                                  -- INSERT with NULL during streaming (fallback)
				'"empty" => ""',
				NULL,
				'"test" => "value"'
			);`,

			`UPDATE test_schema.map_edge_cases
			SET map_simple = '"updated" => "fb"',
				map_with_arrow = '"update=>key" => "fb"'
			WHERE id = 1;`,

			`UPDATE test_schema.map_edge_cases
			SET map_with_quotes = '"fb" => "O''Brien"',
				map_multiple_pairs = '"x" => "99", "y" => "88"'
			WHERE id = 2;`,

			`UPDATE test_schema.map_edge_cases
			SET map_empty_values = '"fallback" => "value"',
				map_multiple_pairs = '"a" => "1", "b" => "2"'
			WHERE id = 5 AND map_empty_values IS NULL;`,

			`UPDATE test_schema.map_edge_cases
			SET map_simple = NULL,
				map_with_arrow = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.map_edge_cases WHERE id = 4;`,

			// ======================================================================
			// NUMERIC TYPES
			// ======================================================================

			// --- INTEGER (integer_edge_cases) ---

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				2147483644,                         -- Different from forward
				-2147483646,
				99,
				-99,
				9223372036854775804,                -- Different from forward
				-9223372036854775806,
				99
			);`,

			`INSERT INTO test_schema.integer_edge_cases (
				int_max,
				int_min,
				int_zero,
				int_negative_one,
				bigint_max,
				bigint_min,
				bigint_zero
			) VALUES
			(
				NULL,                               -- INSERT with NULL during streaming (fallback)
				-1000000,
				NULL,                               -- INSERT with NULL during streaming (fallback)
				-50,
				5000000000000,
				NULL,
				0
			);`,

			`UPDATE test_schema.integer_edge_cases
			SET int_max = 100000,
				bigint_max = 100000000000
			WHERE id = 2;`,

			`UPDATE test_schema.integer_edge_cases
			SET int_zero = 999,
				bigint_zero = 999999
			WHERE id = 1;`,

			`UPDATE test_schema.integer_edge_cases
			SET bigint_max = 777777777777,
				bigint_min = -777777777777
			WHERE id = 5 AND bigint_max IS NULL;`,

			`UPDATE test_schema.integer_edge_cases
			SET int_max = NULL,
				int_min = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.integer_edge_cases WHERE id = 4;`,

			// --- DECIMAL (decimal_edge_cases) ---

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES (
				111222333.444555666,
				-999.888,
				0.000,
				777.777777777777777,
				200.600000,
				88.88
			);`,

			`INSERT INTO test_schema.decimal_edge_cases (
				decimal_large,
				decimal_negative,
				decimal_zero,
				decimal_high_precision,
				decimal_scientific,
				decimal_small
			) VALUES (
				NULL,                               -- INSERT with NULL during streaming (fallback)
				-999.999,
				NULL,                               -- INSERT with NULL during streaming (fallback)
				0.000000000000001,
				NULL,
				50.00
			);`,

			`UPDATE test_schema.decimal_edge_cases
			SET decimal_large = 999999999.999999999,
				decimal_negative = -1000.001
			WHERE id = 1;`,

			`UPDATE test_schema.decimal_edge_cases
			SET decimal_high_precision = 0.123456789012345,
				decimal_scientific = 3.1415926000
			WHERE id = 2;`,

			`UPDATE test_schema.decimal_edge_cases
			SET decimal_negative = -333.333,
				decimal_small = 333.33
			WHERE id = 5 AND decimal_negative IS NULL;`,

			`UPDATE test_schema.decimal_edge_cases
			SET decimal_large = NULL,
				decimal_negative = NULL
			WHERE id = 6;`,

			`DELETE FROM test_schema.decimal_edge_cases WHERE id = 4;`,

			// --- BOOLEAN (boolean_edge_cases) ---

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (FALSE, FALSE);`,

			`INSERT INTO test_schema.boolean_edge_cases (bool_simple, bool_nullable) VALUES (NULL, TRUE);`,

			`UPDATE test_schema.boolean_edge_cases SET bool_simple = TRUE, bool_nullable = FALSE WHERE id = 1;`,

			`UPDATE test_schema.boolean_edge_cases SET bool_simple = FALSE, bool_nullable = TRUE WHERE id = 2;`,

			`UPDATE test_schema.boolean_edge_cases SET bool_simple = FALSE, bool_nullable = FALSE WHERE id = 5 AND bool_simple IS NULL;`,

			`UPDATE test_schema.boolean_edge_cases SET bool_simple = NULL, bool_nullable = NULL WHERE id = 6;`,

			`DELETE FROM test_schema.boolean_edge_cases WHERE id = 4;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	}
}

func TestLiveMigrationWithDatatypeEdgeCases(t *testing.T) {
	lm := NewLiveMigrationTest(t, getDatatypeEdgeCasesTestConfig())

	defer lm.Cleanup()

	t.Log("=== Setting up containers ===")
	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	t.Log("=== Setting up schema ===")
	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	t.Log("=== Starting export data ===")
	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	t.Log("=== Starting import data ===")
	err = lm.StartImportData(true, map[string]string{
		"--log-level": "debug",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	t.Log("=== Waiting for snapshot complete (12 datatypes × 6 rows = 72 rows) ===")
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."string_edge_cases"`:         6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."json_edge_cases"`:           6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."enum_edge_cases"`:           6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."bytes_edge_cases"`:          6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."datetime_edge_cases"`:       6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."uuid_ltree_edge_cases"`:     6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."map_edge_cases"`:            6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."interval_edge_cases"`:       6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."zonedtimestamp_edge_cases"`: 6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."decimal_edge_cases"`:        6, // 6 rows (5 edge cases + 1 with NULL for transitions)
		`test_schema."integer_edge_cases"`:        6, // 6 rows (INT/BIGINT boundaries + NULL transitions)
		`test_schema."boolean_edge_cases"`:        6, // 6 rows (TRUE/FALSE patterns + NULL transitions)
	}, 240) // Increased timeout for 72 rows
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	t.Log("=== Validating snapshot data ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."decimal_edge_cases"`, `test_schema."integer_edge_cases"`, `test_schema."boolean_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate snapshot data consistency")

	t.Log("=== Executing source delta (streaming operations) ===")
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	t.Log("=== Waiting for streaming complete (ALL 12 datatypes with NULL transitions!) ===")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."string_edge_cases"`: {
			Inserts: 4,  // 4 INSERT operations: basic + actual control chars + literal \n test + 1 with NULLs
			Updates: 10, // 10 UPDATE operations: 6 regular + 2 literal \n/actual newline + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,  // 1 DELETE operation: delete row 3
		},
		`test_schema."json_edge_cases"`: {
			Inserts: 3, // 3 INSERT operations: 1 basic + 1 with single quotes in JSON + 1 with NULLs
			Updates: 5, // 5 UPDATE operations: 2 regular + 1 single quotes + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete JSON row 3
		},
		`test_schema."enum_edge_cases"`: {
			Inserts: 3, // 3 INSERT operations: EMPTY array + array with NULL elements + 1 with NULLs
			Updates: 8, // 8 UPDATE operations: 6 regular (empty array tests + NULL elements + add/remove elements) + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete ENUM row 3
		},
		`test_schema."bytes_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: BYTES with special patterns + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete BYTES row 3
		},
		`test_schema."datetime_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: DATETIME with various dates/times + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete DATETIME row 3
		},
		`test_schema."uuid_ltree_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: UUID/LTREE with edge cases + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete UUID/LTREE row 3
		},
		`test_schema."map_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: MAP/HSTORE with arrow operator and quotes + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete MAP row 3
		},
		`test_schema."interval_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: INTERVAL with positive/negative/zero + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete INTERVAL row 3
		},
		`test_schema."zonedtimestamp_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: TIMESTAMPTZ with various timezones + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete TIMESTAMPTZ row 3
		},
		`test_schema."decimal_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: DECIMAL with large, negative, high precision + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete DECIMAL row 3
		},
		`test_schema."integer_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: INT/BIGINT with boundary values + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete INTEGER row 3
		},
		`test_schema."boolean_edge_cases"`: {
			Inserts: 2, // 2 INSERT operations: TRUE/TRUE + 1 with NULLs
			Updates: 4, // 4 UPDATE operations: 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1, // 1 DELETE operation: delete BOOLEAN row 3
		},
	}, 120, 1) // 1 minute timeout, 1 second poll interval
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	t.Log("=== Validating streaming data ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."decimal_edge_cases"`, `test_schema."integer_edge_cases"`, `test_schema."boolean_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	t.Log("=== Initiating cutover ===")
	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	t.Log("=== Waiting for cutover complete ===")
	err = lm.WaitForCutoverComplete(60)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	t.Log("=== Final validation ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."decimal_edge_cases"`, `test_schema."integer_edge_cases"`, `test_schema."boolean_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed final data consistency check")
}

func TestLiveMigrationWithDatatypeEdgeCasesAndFallback(t *testing.T) {
	lm := NewLiveMigrationTest(t, getDatatypeEdgeCasesTestConfig())
	defer lm.Cleanup()

	t.Log("=== Setting up containers ===")
	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	t.Log("=== Setting up schema ===")
	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	t.Log("=== Starting export data ===")
	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	t.Log("=== Starting import data ===")
	err = lm.StartImportData(true, map[string]string{
		"--log-level": "debug",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	t.Log("=== Waiting for snapshot complete (12 datatypes × 6 rows = 72 rows) ===")
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`test_schema."string_edge_cases"`:         6,
		`test_schema."json_edge_cases"`:           6,
		`test_schema."enum_edge_cases"`:           6,
		`test_schema."bytes_edge_cases"`:          6,
		`test_schema."datetime_edge_cases"`:       6,
		`test_schema."uuid_ltree_edge_cases"`:     6,
		`test_schema."map_edge_cases"`:            6,
		`test_schema."interval_edge_cases"`:       6,
		`test_schema."zonedtimestamp_edge_cases"`: 6,
		`test_schema."decimal_edge_cases"`:        6,
		`test_schema."integer_edge_cases"`:        6,
		`test_schema."boolean_edge_cases"`:        6,
	}, 240)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	t.Log("=== Validating snapshot data ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."decimal_edge_cases"`, `test_schema."integer_edge_cases"`, `test_schema."boolean_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate snapshot data consistency")

	t.Log("=== Executing source delta (forward streaming) ===")
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	t.Log("=== Waiting for forward streaming complete ===")
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`test_schema."string_edge_cases"`: {
			Inserts: 4,  // basic + actual control chars + literal \n test + 1 with NULLs
			Updates: 10, // 6 regular + 2 literal \n/actual newline + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."json_edge_cases"`: {
			Inserts: 3, // 1 basic + 1 with single quotes in JSON + 1 with NULLs
			Updates: 5, // 5 UPDATE operations: 2 regular + 1 single quotes + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."enum_edge_cases"`: {
			Inserts: 3, // EMPTY array + array with NULL elements + 1 with NULLs
			Updates: 8, // 6 regular (empty array tests + NULL elements + add/remove elements) + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."bytes_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."datetime_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."uuid_ltree_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."map_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."interval_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."zonedtimestamp_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."decimal_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."integer_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
		`test_schema."boolean_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (value→NULL) + 1 row 6 (NULL→value)
			Deletes: 1,
		},
	}, 180, 2)
	testutils.FatalIfError(t, err, "failed to wait for forward streaming complete")

	t.Log("=== Validating forward streaming data ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."decimal_edge_cases"`, `test_schema."integer_edge_cases"`, `test_schema."boolean_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate streaming data consistency")

	t.Log("=== Initiating cutover to target ===")
	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	t.Log("=== Waiting for cutover complete ===")
	err = lm.WaitForCutoverComplete(90)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	t.Log("=== Executing target delta ===")
	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	t.Log("=== Waiting for CDC to capture changes ===")
	time.Sleep(30 * time.Second)

	t.Log("=== Waiting for fallback streaming complete (ALL 12 datatypes with BOTH NULL transition paths!) ===")
	t.Log("   TESTING: ALL 12 datatypes with FULL edge cases! (Final Step)")
	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`test_schema."string_edge_cases"`: {
			Inserts: 5, // 1 basic + 1 literal \n test + 2 marker rows for UPDATE tests (literal \n, actual newline) + 1 with NULLs
			Updates: 7, // 2 regular + 1 Unicode separators + 2 marker row UPDATEs + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."decimal_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."json_edge_cases"`: {
			Inserts: 4, // 1 basic + 1 with single quotes in JSON + 1 marker row for UPDATE test + 1 with NULLs
			Updates: 5, // 5 UPDATE operations: 2 regular + 1 marker row UPDATE + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."enum_edge_cases"`: {
			Inserts: 3, // 1 EMPTY array + 1 with NULL elements + 1 with NULLs
			Updates: 7, // 5 regular (empty array tests + NULL elements + add/remove elements) + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."bytes_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."datetime_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."uuid_ltree_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."map_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."interval_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."zonedtimestamp_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."integer_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
		`test_schema."boolean_edge_cases"`: {
			Inserts: 2, // 1 basic + 1 with NULLs
			Updates: 4, // 2 regular + 1 row 5 (NULL→value) + 1 row 6 (value→NULL)
			Deletes: 1,
		},
	}, 180, 2)
	testutils.FatalIfError(t, err, "failed to wait for fallback streaming complete")

	t.Log("=== Validating fallback streaming data ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."decimal_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."integer_edge_cases"`, `test_schema."boolean_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate fallback streaming data consistency")

	t.Log("=== Initiating cutover to source ===")
	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	t.Log("=== Waiting for cutover to source complete ===")
	err = lm.WaitForCutoverSourceComplete(150)
	testutils.FatalIfError(t, err, "failed to wait for cutover to source complete")

	t.Log("=== Final validation after complete round-trip ===")
	err = lm.ValidateDataConsistency([]string{`test_schema."string_edge_cases"`, `test_schema."decimal_edge_cases"`, `test_schema."json_edge_cases"`, `test_schema."enum_edge_cases"`, `test_schema."bytes_edge_cases"`, `test_schema."datetime_edge_cases"`, `test_schema."uuid_ltree_edge_cases"`, `test_schema."map_edge_cases"`, `test_schema."interval_edge_cases"`, `test_schema."zonedtimestamp_edge_cases"`, `test_schema."integer_edge_cases"`, `test_schema."boolean_edge_cases"`}, "id")
	testutils.FatalIfError(t, err, "failed final data consistency check after fallback")
}

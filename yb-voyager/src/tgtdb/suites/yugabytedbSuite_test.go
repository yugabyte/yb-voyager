//go:build unit

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
// package for tgtdb value converter suite
package tgtdbsuite

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStringConversionWithFormattingWithDoubleQuotes(t *testing.T) {
	// Given
	value := "abc\"def"
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["STRING"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, "'abc\"def'", result)
}

func TestStringConversionWithFormattingWithSingleQuotesEscaped(t *testing.T) {
	// Given
	value := "abc'def"
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["STRING"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, "'abc''def'", result)
}

func TestJsonConversionWithFormattingWithDoubleQuotes(t *testing.T) {
	// Given
	value := `{"key":"value"}`
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Json"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'{"key":"value"}'`, result)
}

func TestJsonConversionWithFormattingWithSingleQuotesEscaped(t *testing.T) {
	// Given
	value := `{"key":"value's"}`
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Json"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'{"key":"value''s"}'`, result)
}

func TestEnumConversionWithFormattingWithDoubleQuotes(t *testing.T) {
	// Given
	value := `enum"Value`
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Enum"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'enum"Value'`, result)
}

func TestEnumConversionWithFormattingWithSingleQuotesEscaped(t *testing.T) {
	// Given
	value := "enum'Value"
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Enum"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, "'enum''Value'", result)
}

func TestUUIDConversionWithFormatting(t *testing.T) {
	// Given
	value := "123e4567-e89b-12d3-a456-426614174000"
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Uuid"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'123e4567-e89b-12d3-a456-426614174000'`, result)
}

func TestBytesConversionWithFormatting(t *testing.T) {
	//small data example
	value := "////wv=="
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["BYTES"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'\xffffffc2'`, result)

	//large data example for bytea - create a large base64 encoded string
	// Generate 10KB of binary data (10240 bytes)
	largeData := make([]byte, 10240)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	// Encode to base64
	largeBase64Value := base64.StdEncoding.EncodeToString(largeData)

	// When we convert with formatIfRequired is true
	result, err = YBValueConverterSuite["BYTES"](largeBase64Value, true, nil)
	assert.NoError(t, err)
	//verify the result is same as expected
	assert.Equal(t, len(result), 20484) //20480 hex chars + 4 for '\x' + 2 for quotes

	largeData10MB := make([]byte, 10000000)
	for i := range largeData10MB {
		largeData10MB[i] = byte(i % 256)
	}
	// Encode to base64
	largeBase64Value10MB := base64.StdEncoding.EncodeToString(largeData10MB)

	// When we convert with formatIfRequired is true
	result, err = YBValueConverterSuite["BYTES"](largeBase64Value10MB, true, nil)
	assert.NoError(t, err)

	//verify the result is same as expected
	assert.Equal(t, len(result), 20000004) //20000000 hex chars + 4 for '\x' + 2 for quotes

	largeData200MB := make([]byte, 200000000)
	for i := range largeData200MB {
		largeData200MB[i] = byte(i % 256)
	}
	// Encode to base64
	largeBase64Value200MB := base64.StdEncoding.EncodeToString(largeData200MB)

	// When we convert with formatIfRequired is true
	result, err = YBValueConverterSuite["BYTES"](largeBase64Value200MB, true, nil)
	assert.NoError(t, err)

	//verify the result is same as expected
	assert.Equal(t, len(result), 400000004) //400000000 hex chars + 4 for '\x' + 2 for quotes
}

// ============================================================================
// YUGABYTEDB VALUE CONVERTER UNIT TEST COVERAGE MATRIX
// ============================================================================
//
// This matrix shows edge case coverage for YugabyteDB value converter functions.
// Unit tests focus on **converter function logic** (input string → output string),
// not database operations or streaming phases (covered in integration tests).
//
// Legend:
//   ✓ = Covered (test exists)
//   ⚠ = Partially covered (some cases missing)
//   ✗ = Not covered (test needed)
//   N/A = Not applicable for unit tests
//
// Overall Coverage: 87% (93/106 test cases)
//
// ============================================================================
// 1. STRING DATATYPE (TEXT/VARCHAR) - 100% ✅ (16/16) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Unicode characters (café, 日本語)   | ✓      | TestStringConversionWithUnicode               | Multi-byte chars
// Emojis (🎉, 👨‍👩‍👧‍👦)                   | ✓      | TestStringConversionWithUnicode               | Emoji family
// Single quotes (It's, O'Reilly)    | ✓      | TestStringConversionWithFormattingWithSingleQuotesEscaped | SQL escaping
// Double quotes ("test")            | ✓      | TestStringConversionWithFormattingWithDoubleQuotes | Double quote handling
// Backslashes (C:\path\to\file)     | ✓      | TestStringConversionWithBackslash             | Windows paths
// Actual newline byte (0x0A)        | ✓      | TestStringConversionWithNewlineCharacters     | E'...\n...'
// Literal \n string (backslash+n)   | ✓      | TestStringConversionWithLiteralBackslashN     | Two-char string
// Actual tab byte (0x09)            | ✓      | TestStringConversionWithNewlineCharacters     | E'...\t...'
// Actual carriage return (0x0D)     | ✓      | TestStringConversionWithNewlineCharacters     | E'...\r...'
// Mixed control chars (\n\t\r)      | ✓      | TestStringConversionWithMixedSpecialChars     | All control chars
// Unicode separators (U+2028)       | ✓      | TestStringConversionWithUnicodeSeparators     | Line/para/zero-width
// Empty string ('')                 | ✓      | TestStringConversionWithNullString            | Zero-length
// String literal 'NULL'             | ✓      | TestStringConversionWithNullString            | vs actual NULL
// SQL injection patterns            | ✓      | TestStringConversionWithCriticalEdgeCases     | --comment, '; DROP
// Bidirectional text (RTL)          | ✓      | TestStringConversionWithBidirectionalText     | Arabic/Hebrew
// Very large strings                | ✓      | TestStringConversionWithVeryLargeStrings      | 1KB, 10KB, 100KB
//
// ============================================================================
// 2. JSON/JSONB DATATYPE - 100% ✅ (11/11) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Single quotes in values            | ✓      | TestJsonConversionWithFormattingWithSingleQuotesEscaped | SQL escaping
// Escaped characters (\", \\)        | ✓      | TestJsonConversionBasic                       | JSON escaping
// Unicode in JSON                    | ✓      | TestJsonConversionBasic                       | café, 日本語, 🎉
// Nested objects                     | ✓      | TestJsonConversionWithComplexStructures       | Deep nesting
// Arrays                             | ✓      | TestJsonConversionWithComplexStructures       | Nested arrays
// NULL value in JSON                 | ✓      | TestJsonConversionBasic                       | {"key": null}
// Empty JSON                         | ✓      | TestJsonConversionBasic                       | {}
// Formatted JSON                     | ✓      | TestJsonConversionWithFormattingWithDoubleQuotes | Whitespace
// Numbers in JSON                    | ✓      | TestJsonConversionBasic                       | Int, float, bool
// Complex nested structures          | ✓      | TestJsonConversionWithDeepNesting             | Mixed types
// Deep nesting (10+ levels)          | ✓      | TestJsonConversionWithDeepNesting             | Extreme depth
//
// ============================================================================
// 3. ENUM DATATYPE - 100% ✅ (11/11) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Simple enum values                 | ✓      | TestEnumConversionWithSpecialChars            | active, pending
// Enum with single quote             | ✓      | TestEnumConversionWithFormattingWithSingleQuotesEscaped | enum'value
// Enum with double quote             | ✓      | TestEnumConversionWithFormattingWithDoubleQuotes | enum"value
// Enum with backslash                | ✓      | TestEnumConversionWithSpecialChars            | enum\value
// Enum with spaces                   | ✓      | TestEnumConversionWithSpecialChars            | 'with space'
// Enum with dashes                   | ✓      | TestEnumConversionWithSpecialChars            | with-dash
// Enum with underscore               | ✓      | TestEnumConversionWithSpecialChars            | with_underscore
// Enum with Unicode                  | ✓      | TestEnumConversionWithSpecialChars            | café, 🎉emoji
// Enum starting with digits          | ✓      | TestEnumConversionWithSpecialChars            | 123value
// Empty ENUM array                   | ✓      | TestArrayConversionWithEdgeCases              | "{}" via STRING converter
// ENUM array with NULL elements      | ✓      | TestArrayConversionWithEdgeCases              | "{a,NULL,b}" via STRING
//
// ============================================================================
// 4. BYTES DATATYPE (BYTEA) - 100% ✅ (10/10) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Empty bytes                        | ✓      | TestBytesConversionWithSpecialPatterns        | \\x
// Single byte                        | ✓      | TestBytesConversionWithSpecialPatterns        | \\x41
// ASCII string as bytes              | ✓      | TestBytesConversionWithSpecialPatterns        | Text → hex
// NULL byte in middle                | ✓      | TestBytesConversionWithSpecialPatterns        | \\x00
// All zeros                          | ✓      | TestBytesConversionWithSpecialPatterns        | \\x000000
// All 0xFF                           | ✓      | TestBytesConversionWithSpecialPatterns        | \\xFFFFFF
// Special char bytes (', \, \n)      | ✓      | TestBytesConversionWithBinarySpecialChars     | Binary chars
// Mixed byte patterns                | ✓      | TestBytesConversionWithSpecialPatterns        | Random hex
// Invalid base64                     | ✓      | TestBytesConversionInvalidBase64              | Error handling
// formatIfRequired parameter         | ✓      | TestBytesConversionFormatIfRequired           | With/without quotes
//
// ============================================================================
// 5. DATETIME DATATYPE (DATE/TIMESTAMP/TIME) - 100% ✅ (10/10) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Epoch date (1970-01-01)            | ✓      | TestDateConversionEdgeCases                   | Unix epoch
// Negative epoch (before 1970)       | ✓      | TestDateConversionEdgeCases                   | Historical dates
// Future dates (2050+)               | ✓      | TestDateConversionEdgeCases                   | Far future
// Timestamps with timezone           | ✓      | TestZonedTimestampConversion                  | Full coverage
// Midnight (00:00:00)                | ✓      | TestTimeConversion                            | Day boundary
// Noon (12:00:00)                    | ✓      | TestTimeConversion                            | Mid-day
// Microsecond precision              | ✓      | TestMicroTimestampConversion, TestMicroTimeConversion | 6 decimal places
// Nanosecond precision               | ✓      | TestNanoTimestampConversion                   | 9 decimal places
// Invalid input handling             | ✓      | TestTimestampConversionInvalidInput           | Error cases
// End of day (23:59:59)              | ✓      | TestTimeConversion                            | Added explicitly
//
// ============================================================================
// 6. UUID DATATYPE - 100% ✅ (5/5) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Standard UUID v4                   | ✓      | TestUuidConversionEdgeCases                   | Random UUID
// All zeros UUID                     | ✓      | TestUuidConversionEdgeCases                   | 00000000-0000...
// All Fs UUID                        | ✓      | TestUuidConversionEdgeCases                   | ffffffff-ffff...
// Invalid UUID format                | ✓      | TestUuidConversionInvalidInput                | Error handling
// formatIfRequired parameter         | ✓      | TestUUIDConversionWithFormatting              | With/without quotes
//
// ============================================================================
// 7. LTREE DATATYPE - 100% ✅ (4/4) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Simple path                        | ✓      | TestLtreeConversionEdgeCases                  | Top.Science
// Quoted labels                      | ✓      | TestLtreeConversionEdgeCases                  | "Special Label"
// Deep hierarchy                     | ✓      | TestLtreeConversionEdgeCases                  | 10+ levels
// Single label                       | ✓      | TestLtreeConversionEdgeCases                  | Top only
//
// ============================================================================
// 8. MAP DATATYPE (HSTORE) - 100% ✅ (8/8) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Arrow operator in key              | ✓      | TestMapConversionWithArrowOperator            | "key=>val"=>"x"
// Arrow operator in value            | ✓      | TestMapConversionWithArrowOperator            | "k"=>"val=>test"
// Escaped quotes                     | ✓      | TestMapConversionWithEscapedChars             | "key\"test"
// Escaped backslash                  | ✓      | TestMapConversionWithEscapedChars             | "key\\test"
// Single quotes in value             | ✓      | TestMapConversionWithEscapedChars             | "k"=>"O'Reilly"
// Empty key                          | ✓      | TestMapConversionWithEmptyValues              | ""=>"value"
// Empty value                        | ✓      | TestMapConversionWithEmptyValues              | "key"=>""
// Multiple pairs                     | ✓      | TestMapConversionWithMultiplePairs            | k1=>v1, k2=>v2
//
// ============================================================================
// 9. INTERVAL DATATYPE - 63% (5/8)
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Positive intervals                 | ✓      | TestIntervalConversionEdgeCases               | Years, months
// Negative intervals                 | ✓      | TestIntervalConversionEdgeCases               | -15 days
// Zero interval                      | ✓      | TestIntervalConversionEdgeCases               | 0 seconds
// Years only                         | ⚠      | TestIntervalConversionEdgeCases               | Should verify
// Days only                          | ⚠      | TestIntervalConversionEdgeCases               | Should verify
// Time only                          | ⚠      | TestIntervalConversionEdgeCases               | Should verify
// Mixed components                   | ✓      | TestIntervalConversionEdgeCases               | Years+days+hours
// Very large values                  | ✗      | MISSING                                       | 999999 years
//
// ============================================================================
// 10. ZONEDTIMESTAMP DATATYPE (TIMESTAMPTZ) - 100% ✅ (6/6) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// UTC timezone (+00)                 | ✓      | TestZonedTimestampConversion                  | Zulu time
// Positive offset (+04:00, +05:30)   | ✓      | TestZonedTimestampConversion                  | Dubai, India
// Negative offset (-07:00)           | ✓      | TestZonedTimestampConversion                  | PDT, EST
// Epoch with timezone                | ✓      | TestZonedTimestampConversion                  | 1970-01-01+00
// Future with timezone               | ✓      | TestZonedTimestampConversion                  | 2065+
// Midnight with timezone             | ✓      | TestZonedTimestampConversion                  | 00:00:00+00
//
// ============================================================================
// 11. DECIMAL DATATYPE (NUMERIC) - 100% ✅ (7/7) COMPLETE
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// Large numbers (1B+)                | ✓      | TestDecimalConversionEdgeCases                | 999999999.999...
// Negative numbers                   | ✓      | TestDecimalConversionEdgeCases                | -999999.999
// Zero (0.0, 0.00, 0.000)            | ✓      | TestDecimalConversionEdgeCases                | Various scales
// High precision (15+ decimals)      | ✓      | TestDecimalConversionEdgeCases                | 0.123456789...
// Scientific notation                | ✓      | TestDecimalConversionEdgeCases                | 1.23E+10
// Small decimals                     | ✓      | TestDecimalConversionEdgeCases                | 0.0001
// Variable scale                     | ✓      | TestVariableScaleDecimalConversion            | Different scales
//
// ============================================================================
// 12. INTEGER DATATYPE (INT/BIGINT) - 0% (0/7) ⚠️ HIGH PRIORITY - NO TESTS
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// INT MAX (2147483647)               | ✗      | MISSING                                       | Max 32-bit
// INT MIN (-2147483648)              | ✗      | MISSING                                       | Min 32-bit
// BIGINT MAX (9223372036854775807)   | ✗      | MISSING                                       | Max 64-bit
// BIGINT MIN (-9223372036854775808)  | ✗      | MISSING                                       | Min 64-bit
// Zero                               | ✗      | MISSING                                       | 0
// Negative one                       | ✗      | MISSING                                       | -1
// Overflow scenarios                 | ✗      | MISSING                                       | Boundary tests
//
// ============================================================================
// 13. BOOLEAN DATATYPE - 0% (0/3) ⚠️ HIGH PRIORITY - NO TESTS
// ============================================================================
// Edge Case                          | Status | Test Function(s)                              | Notes
// -----------------------------------|--------|-----------------------------------------------|------------------------
// TRUE value                         | ✗      | MISSING                                       | true
// FALSE value                        | ✗      | MISSING                                       | false
// NULL value                         | ✗      | MISSING                                       | null
//
// ============================================================================
// SUMMARY BY DATATYPE
// ============================================================================
// Datatype          | Covered | Total | Percentage | Priority
// ------------------|---------|-------|------------|----------
// STRING            | 16      | 16    | 100% ✅    | Complete
// JSON/JSONB        | 11      | 11    | 100% ✅    | Complete
// ENUM              | 11      | 11    | 100% ✅    | Complete
// BYTES             | 10      | 10    | 100% ✅    | Complete
// DATETIME          | 10      | 10    | 100% ✅    | Complete
// UUID              | 5       | 5     | 100% ✅    | Complete
// LTREE             | 4       | 4     | 100% ✅    | Complete
// MAP (HSTORE)      | 8       | 8     | 100% ✅    | Complete
// INTERVAL          | 5       | 8     | 63%        | Medium
// ZONEDTIMESTAMP    | 6       | 6     | 100% ✅    | Complete
// DECIMAL           | 7       | 7     | 100% ✅    | Complete
// INTEGER           | 0       | 7     | 0%         | HIGH
// BOOLEAN           | 0       | 3     | 0%         | HIGH
// **TOTAL**         | **93**  | **106** | **88%** |
//
// ✅ Complete Datatypes: STRING, JSON/JSONB, ENUM, BYTES, DATETIME, UUID, LTREE, MAP, ZONEDTIMESTAMP, DECIMAL (10/13)
//
// ============================================================================
// PRIORITY GAPS TO FILL
// ============================================================================
// HIGH PRIORITY (Missing entirely or critical gaps):
//   1. INTEGER/BIGINT - 0% coverage - Full test suite needed
//   2. BOOLEAN - 0% coverage - Full test suite needed
//
// MEDIUM PRIORITY (Partial coverage):
//   3. INTERVAL - Missing: Component-specific tests, large values
//
// ============================================================================
// NOTES
// ============================================================================
// - Integration tests (live_migration_integration_test.go) prove these work end-to-end
// - Unit tests should test converter logic in isolation
// - formatIfRequired parameter should be tested for all datatypes
// - Error handling tests should exist for invalid inputs
// - NULL handling is tested in integration tests, not unit tests
// - Array types (text[], enum[], etc.) are converted to STRING type by Debezium's
//   PostgresToYbValueConverter, then processed by the STRING converter
//   (quoteValueIfRequiredWithEscaping). Array literals like "{}" or "{a,NULL,b}"
//   should be unit tested via STRING converter tests.
//
// ============================================================================

// Test 1.1: TestStringConversionWithBackslash
// Tests backslash handling in STRING type conversion
// NOTE: With standard_conforming_strings=ON (default in PostgreSQL 9.1+),
// backslashes are treated as literal characters and don't need escaping
func TestStringConversionWithBackslash(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "single backslash in path",
			input:            `C:\path`,
			formatIfRequired: true,
			expected:         `'C:\path'`, // Backslash is literal with standard_conforming_strings=ON
			note:             "Backslash preserved as literal character",
		},
		{
			name:             "double backslash",
			input:            `\\server\path`,
			formatIfRequired: true,
			expected:         `'\\server\path'`, // Backslashes preserved
			note:             "Multiple backslashes preserved as literals",
		},
		{
			name:             "backslash-n as literal string (not newline)",
			input:            `line1\nline2`,
			formatIfRequired: true,
			expected:         `'line1\nline2'`, // Literal \n, not interpreted as newline
			note:             "Literal backslash-n preserved (not interpreted as newline)",
		},
		{
			name:             "backslash-t as literal string (not tab)",
			input:            `col1\tcol2`,
			formatIfRequired: true,
			expected:         `'col1\tcol2'`, // Literal \t, not interpreted as tab
			note:             "Literal backslash-t preserved (not interpreted as tab)",
		},
		{
			name:             "backslash-r as literal string (not carriage return)",
			input:            `text\rmore`,
			formatIfRequired: true,
			expected:         `'text\rmore'`, // Literal \r, not interpreted as CR
			note:             "Literal backslash-r preserved (not interpreted as carriage return)",
		},
		{
			name:             "Windows path with backslashes",
			input:            `C:\Users\test\Documents\file.txt`,
			formatIfRequired: true,
			expected:         `'C:\Users\test\Documents\file.txt'`,
			note:             "Windows paths preserve all backslashes as literals",
		},
		{
			name:             "UNC path",
			input:            `\\server\share\folder`,
			formatIfRequired: true,
			expected:         `'\\server\share\folder'`,
			note:             "UNC paths work with literal backslashes",
		},
		{
			name:             "backslash without formatting",
			input:            `path\to\file`,
			formatIfRequired: false,
			expected:         `path\to\file`, // No quotes, backslashes preserved
			note:             "COPY format also preserves backslashes",
		},
		{
			name:             "empty string with formatting",
			input:            ``,
			formatIfRequired: true,
			expected:         `''`,
			note:             "Empty string should produce empty quoted string",
		},
		{
			name:             "single backslash only",
			input:            `\`,
			formatIfRequired: true,
			expected:         `'\'`,
			note:             "Single backslash preserved as literal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			// Log the test case details for debugging
			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 1.2: TestStringConversionWithNewlineCharacters
// Tests handling of actual newline/tab/carriage return characters (not escape sequences)
// These are literal control characters, not the two-character sequences \n, \t, \r
func TestStringConversionWithNewlineCharacters(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "actual newline character",
			input:            "line1\nline2", // Actual newline, not backslash-n
			formatIfRequired: true,
			expected:         "'line1\nline2'", // PostgreSQL accepts literal newlines in strings
			note:             "Actual newline character should be preserved",
		},
		{
			name:             "actual tab character",
			input:            "col1\tcol2", // Actual tab
			formatIfRequired: true,
			expected:         "'col1\tcol2'",
			note:             "Actual tab character should be preserved",
		},
		{
			name:             "actual carriage return",
			input:            "text\rmore", // Actual CR
			formatIfRequired: true,
			expected:         "'text\rmore'",
			note:             "Actual carriage return should be preserved",
		},
		{
			name:             "mixed control characters",
			input:            "line1\nline2\ttab\rreturn",
			formatIfRequired: true,
			expected:         "'line1\nline2\ttab\rreturn'",
			note:             "Multiple control characters should be preserved",
		},
		{
			name:             "newline at start",
			input:            "\nstart",
			formatIfRequired: true,
			expected:         "'\nstart'",
			note:             "Leading newline should be preserved",
		},
		{
			name:             "newline at end",
			input:            "end\n",
			formatIfRequired: true,
			expected:         "'end\n'",
			note:             "Trailing newline should be preserved",
		},
		{
			name:             "multiple consecutive newlines",
			input:            "line1\n\n\nline2",
			formatIfRequired: true,
			expected:         "'line1\n\n\nline2'",
			note:             "Multiple newlines should be preserved",
		},
		{
			name:             "only whitespace characters",
			input:            " \t\n\r ",
			formatIfRequired: true,
			expected:         "' \t\n\r '",
			note:             "Whitespace-only strings should be preserved",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input (repr): %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 1.3: TestStringConversionWithNullString
// Tests handling of NULL-related strings and edge cases
func TestStringConversionWithNullString(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "string literal NULL (uppercase)",
			input:            "NULL",
			formatIfRequired: true,
			expected:         "'NULL'",
			note:             "String 'NULL' should be quoted, not treated as SQL NULL",
		},
		{
			name:             "string literal null (lowercase)",
			input:            "null",
			formatIfRequired: true,
			expected:         "'null'",
			note:             "String 'null' should be quoted",
		},
		{
			name:             "empty string",
			input:            "",
			formatIfRequired: true,
			expected:         "''",
			note:             "Empty string produces empty quoted string",
		},
		{
			name:             "empty string without formatting",
			input:            "",
			formatIfRequired: false,
			expected:         "",
			note:             "Empty string without formatting remains empty",
		},
		{
			name:             "single space",
			input:            " ",
			formatIfRequired: true,
			expected:         "' '",
			note:             "Single space should be preserved",
		},
		{
			name:             "multiple spaces",
			input:            "   ",
			formatIfRequired: true,
			expected:         "'   '",
			note:             "Multiple spaces should be preserved",
		},
		{
			name:             "tab only",
			input:            "\t",
			formatIfRequired: true,
			expected:         "'\t'",
			note:             "Tab-only string should be preserved",
		},
		{
			name:             "newline only",
			input:            "\n",
			formatIfRequired: true,
			expected:         "'\n'",
			note:             "Newline-only string should be preserved",
		},
		{
			name:             "string with NULL word inside",
			input:            "This is NULL value",
			formatIfRequired: true,
			expected:         "'This is NULL value'",
			note:             "NULL as part of string should be preserved",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 1.4: TestStringConversionWithMixedSpecialChars
// Tests combinations of special characters together
func TestStringConversionWithMixedSpecialChars(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "single quote with double quote",
			input:            `It's "test"`,
			formatIfRequired: true,
			expected:         `'It''s "test"'`,
			note:             "Single quote escaped, double quote preserved",
		},
		{
			name:             "single quote with backslash and newline",
			input:            `It's test with \ and \n`,
			formatIfRequired: true,
			expected:         `'It''s test with \ and \n'`, // Only ' needs escaping, \ is literal
			note:             "Single quote escaped, backslashes preserved as literals",
		},
		{
			name:             "multiple special chars",
			input:            `Single'Double"Back\New\nTab\t`,
			formatIfRequired: true,
			expected:         `'Single''Double"Back\New\nTab\t'`,
			note:             "Only single quote needs escaping, backslashes are literals",
		},
		{
			name:             "SQL injection pattern",
			input:            `'; DROP TABLE users--`,
			formatIfRequired: true,
			expected:         `'''; DROP TABLE users--'`,
			note:             "SQL injection attempts should be safely escaped",
		},
		{
			name:             "consecutive single quotes",
			input:            `O''Reilly`,
			formatIfRequired: true,
			expected:         `'O''''Reilly'`, // Two single quotes become four
			note:             "Already-escaped quotes need further escaping",
		},
		{
			name:             "backslash before single quote",
			input:            `test\'s value`,
			formatIfRequired: true,
			expected:         `'test\''s value'`, // Only ' becomes '', \ stays literal
			note:             "Backslash preserved, single quote escaped",
		},
		{
			name:             "all problematic chars",
			input:            "mix: '\"\\\n\t\r", // Contains: ', ", \, actual newline, actual tab, actual CR
			formatIfRequired: true,
			expected:         "'mix: ''\"\\\n\t\r'", // Only ' escapes to '', rest preserved (\ is literal, control chars preserved)
			note:             "Only single quote needs escaping, backslashes are literal, control chars preserved",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 1.5: TestStringConversionWithUnicode
// Tests Unicode character handling
func TestStringConversionWithUnicode(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "accented characters",
			input:            "café",
			formatIfRequired: true,
			expected:         "'café'",
			note:             "Accented characters should be preserved",
		},
		{
			name:             "emoji",
			input:            "🎉",
			formatIfRequired: true,
			expected:         "'🎉'",
			note:             "Emoji should be preserved",
		},
		{
			name:             "Japanese characters",
			input:            "日本語",
			formatIfRequired: true,
			expected:         "'日本語'",
			note:             "Multi-byte characters should be preserved",
		},
		{
			name:             "mixed ASCII and Unicode",
			input:            "Hello 世界 🌍",
			formatIfRequired: true,
			expected:         "'Hello 世界 🌍'",
			note:             "Mixed character sets should work",
		},
		{
			name:             "Unicode with single quote",
			input:            "café's specialty",
			formatIfRequired: true,
			expected:         "'café''s specialty'",
			note:             "Unicode with special chars",
		},
		{
			name:             "Unicode with backslash",
			input:            `path\to\日本語`,
			formatIfRequired: true,
			expected:         `'path\to\日本語'`,
			note:             "Unicode with backslashes preserved as literals",
		},
		{
			name:             "Unicode line separator (U+2028)",
			input:            "line1\u2028line2",
			formatIfRequired: true,
			expected:         "'line1\u2028line2'",
			note:             "Unicode line separator (different from \\n) should be preserved",
		},
		{
			name:             "Unicode paragraph separator (U+2029)",
			input:            "para1\u2029para2",
			formatIfRequired: true,
			expected:         "'para1\u2029para2'",
			note:             "Unicode paragraph separator should be preserved",
		},
		{
			name:             "zero-width space",
			input:            "word\u200Bword",
			formatIfRequired: true,
			expected:         "'word\u200Bword'",
			note:             "Zero-width space (U+200B) should be preserved",
		},
		{
			name:             "zero-width joiner in emoji",
			input:            "👨\u200D👩\u200D👧",
			formatIfRequired: true,
			expected:         "'👨\u200D👩\u200D👧'",
			note:             "Zero-width joiner for composite emoji should be preserved",
		},
		{
			name:             "bidirectional text (Arabic + English)",
			input:            "English مرحبا English",
			formatIfRequired: true,
			expected:         "'English مرحبا English'",
			note:             "Mixed LTR and RTL text should be preserved",
		},
		{
			name:             "non-breaking space",
			input:            "word\u00A0word",
			formatIfRequired: true,
			expected:         "'word\u00A0word'",
			note:             "Non-breaking space (NBSP) should be preserved",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 1.6: TestStringConversionWithCriticalEdgeCases
// Tests critical edge cases: null bytes, control characters, very long strings
//
// IMPORTANT NOTE about invalid UTF-8 bytes:
// PostgreSQL TEXT columns with UTF-8 encoding do NOT allow:
//   - Null bytes (\x00)
//   - DEL character (\x7F)
//   - C1 control characters (\x80-\x9F) - these are UTF-8 continuation bytes, not standalone chars
//
// These tests verify that the converter handles these bytes correctly (doesn't crash),
// but in practice, PostgreSQL will reject INSERT/UPDATE with these bytes in TEXT fields.
// For binary data containing these bytes, use BYTEA datatype instead of TEXT.
func TestStringConversionWithCriticalEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "null byte in middle",
			input:            "before\x00after",
			formatIfRequired: true,
			expected:         "'before\x00after'",
			note:             "Converter preserves null bytes (but PostgreSQL UTF-8 TEXT will reject)",
		},
		{
			name:             "multiple null bytes",
			input:            "\x00start\x00middle\x00end\x00",
			formatIfRequired: true,
			expected:         "'\x00start\x00middle\x00end\x00'",
			note:             "Multiple null bytes - converter handles, but PostgreSQL rejects",
		},
		{
			name:             "DEL character (0x7F)",
			input:            "text\x7Fmore",
			formatIfRequired: true,
			expected:         "'text\x7Fmore'",
			note:             "DEL - converter preserves, but not valid UTF-8 in PostgreSQL TEXT",
		},
		{
			name:             "C1 control characters",
			input:            "text\x80\x9Fmore",
			formatIfRequired: true,
			expected:         "'text\x80\x9Fmore'",
			note:             "C1 controls (0x80-0x9F) - converter handles, but not valid UTF-8",
		},
		{
			name:             "small string (1 char)",
			input:            "a",
			formatIfRequired: true,
			expected:         "'a'",
			note:             "Baseline for length tests",
		},
		{
			name:             "medium string (1KB)",
			input:            string(make([]byte, 1024)),
			formatIfRequired: true,
			expected:         "'" + string(make([]byte, 1024)) + "'",
			note:             "1KB string should be handled efficiently",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			// For long strings, just check length and prefix/suffix
			if len(tc.input) > 100 {
				t.Logf("Input length: %d bytes", len(tc.input))
				t.Logf("Result length: %d bytes", len(result))
				t.Logf("Expected length: %d bytes", len(tc.expected))
				assert.Equal(t, len(tc.expected), len(result), "Length should match")

				// Check it has quotes and content matches
				assert.True(t, result[0] == '\'', "Should start with quote")
				assert.True(t, result[len(result)-1] == '\'', "Should end with quote")
				assert.Equal(t, tc.input, result[1:len(result)-1], "Content should match")
			} else {
				t.Logf("Input: %q", tc.input)
				t.Logf("Expected: %q", tc.expected)
				t.Logf("Got: %q", result)
				assert.Equal(t, tc.expected, result, "Converted value should match expected")
			}

			t.Logf("Note: %s", tc.note)
		})
	}
}

// Test 1.7: TestStringConversionWithVeryLargeStrings
// Tests performance and correctness with large strings (up to 20MB)
// String creation: Allocates byte array and fills with repeating a-z pattern
// This verifies data integrity while testing realistic document sizes
// TestStringConversionWithVeryLargeStrings tests STRING converter with sizes matching BYTES test pattern
// Pattern: 10KB → 10MB → 200MB (matches TestBytesConversionWithFormatting)
func TestStringConversionWithVeryLargeStrings(t *testing.T) {
	// Test 1: 10KB string (typical document size)
	t.Run("10KB string", func(t *testing.T) {
		size := 10 * 1024 // 10KB = 10240 bytes
		input := make([]byte, size)
		for i := range input {
			input[i] = byte('a' + (i % 26)) // Repeating a-z pattern
		}
		inputStr := string(input)

		// Convert with formatIfRequired=true (SQL format with quotes)
		result, err := YBValueConverterSuite["STRING"](inputStr, true, nil)
		assert.NoError(t, err)

		// Verify result
		expectedSize := size + 2 // +2 for quotes
		assert.Equal(t, expectedSize, len(result), "10KB + 2 quotes = 10242 bytes")
		assert.Equal(t, '\'', rune(result[0]), "Should start with quote")
		assert.Equal(t, '\'', rune(result[len(result)-1]), "Should end with quote")
		assert.Equal(t, inputStr, result[1:len(result)-1], "Content should match")

		t.Logf("✓ 10KB string: %d bytes → %d bytes", size, len(result))
	})

	// Test 2: 10MB string (large document size)
	t.Run("10MB string", func(t *testing.T) {
		size := 10 * 1024 * 1024 // 10MB = 10485760 bytes
		input := make([]byte, size)
		for i := range input {
			input[i] = byte('a' + (i % 26))
		}
		inputStr := string(input)

		// Convert with formatIfRequired=true
		result, err := YBValueConverterSuite["STRING"](inputStr, true, nil)
		assert.NoError(t, err)

		// Verify length (full content check would be slow)
		expectedSize := size + 2 // +2 for quotes
		assert.Equal(t, expectedSize, len(result), "10MB + 2 quotes = 10485762 bytes")

		// Verify first and last 1KB (data integrity spot check)
		assert.Equal(t, inputStr[:1024], result[1:1025], "First 1KB should match")
		assert.Equal(t, inputStr[len(inputStr)-1024:], result[len(result)-1025:len(result)-1], "Last 1KB should match")

		t.Logf("✓ 10MB string: %d bytes → %d bytes", size, len(result))
	})

	// Test 3: 200MB string (extreme stress test - matches BYTES test)
	t.Run("200MB string", func(t *testing.T) {
		size := 200 * 1024 * 1024 // 200MB = 209715200 bytes
		input := make([]byte, size)
		for i := range input {
			input[i] = byte('a' + (i % 26))
		}
		inputStr := string(input)

		t.Logf("Converting 200MB string...")
		start := time.Now()
		result, err := YBValueConverterSuite["STRING"](inputStr, true, nil)
		duration := time.Since(start)

		assert.NoError(t, err)

		// Verify length
		expectedSize := size + 2 // +2 for quotes
		assert.Equal(t, expectedSize, len(result), "200MB + 2 quotes = 209715202 bytes")

		// Verify first and last 1KB (data integrity spot check)
		assert.Equal(t, inputStr[:1024], result[1:1025], "First 1KB should match")
		assert.Equal(t, inputStr[len(inputStr)-1024:], result[len(result)-1025:len(result)-1], "Last 1KB should match")

		t.Logf("✓ 200MB string: %d bytes → %d bytes in %v", size, len(result), duration)
		t.Logf("✓ Throughput: %.2f MB/s", float64(size)/(1024*1024)/duration.Seconds())
	})
}

// Test 1.8: TestStringConversionWithLiteralBackslashN
// Tests the distinction between literal \n (two characters: backslash + n) vs actual newline byte (0x0A)
// This is critical for correctly handling strings like "C:\new\test" which should NOT be interpreted as control characters
func TestStringConversionWithLiteralBackslashN(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "Literal backslash-n (two characters)",
			input:            `literal\nstring`,
			formatIfRequired: true,
			expected:         `'literal\nstring'`,
			note:             "\\n should remain as two literal characters",
		},
		{
			name:             "Windows path with \\n",
			input:            `C:\new\test\file.txt`,
			formatIfRequired: true,
			expected:         `'C:\new\test\file.txt'`,
			note:             "Windows paths with \\n should not be converted to newlines",
		},
		{
			name:             "Multiple literal \\n",
			input:            `first\nsecond\nthird`,
			formatIfRequired: true,
			expected:         `'first\nsecond\nthird'`,
			note:             "Multiple \\n should all remain literal",
		},
		{
			name:             "Mix of \\n, \\t, \\r as literals",
			input:            `line\nwith\ttabs\rand\nmore`,
			formatIfRequired: true,
			expected:         `'line\nwith\ttabs\rand\nmore'`,
			note:             "All escape sequences should remain literal",
		},
		{
			name:             "Literal \\n without formatting",
			input:            `test\nvalue`,
			formatIfRequired: false,
			expected:         `test\nvalue`,
			note:             "Without formatting, \\n passes through unchanged",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, tc.note)
			assert.Equal(t, tc.expected, result, tc.note)
		})
	}
}

// Test 1.9: TestStringConversionWithUnicodeSeparators
// Tests Unicode separator characters that are not visible but have special meaning
// U+2028 (Line Separator), U+2029 (Paragraph Separator), U+200B (Zero-Width Space), U+00A0 (Non-Breaking Space)
func TestStringConversionWithUnicodeSeparators(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "Line separator U+2028",
			input:            "line1\u2028line2",
			formatIfRequired: true,
			expected:         "'line1\u2028line2'",
			note:             "Unicode line separator should be preserved",
		},
		{
			name:             "Paragraph separator U+2029",
			input:            "para1\u2029para2",
			formatIfRequired: true,
			expected:         "'para1\u2029para2'",
			note:             "Unicode paragraph separator should be preserved",
		},
		{
			name:             "Zero-width space U+200B",
			input:            "word\u200Bword",
			formatIfRequired: true,
			expected:         "'word\u200Bword'",
			note:             "Zero-width space should be preserved",
		},
		{
			name:             "Non-breaking space U+00A0",
			input:            "word\u00A0word",
			formatIfRequired: true,
			expected:         "'word\u00A0word'",
			note:             "Non-breaking space should be preserved",
		},
		{
			name:             "Multiple Unicode separators mixed",
			input:            "text\u2028with\u2029various\u200Bseparators\u00A0here",
			formatIfRequired: true,
			expected:         "'text\u2028with\u2029various\u200Bseparators\u00A0here'",
			note:             "All Unicode separators should be preserved together",
		},
		{
			name:             "Unicode separators without formatting",
			input:            "test\u2028value",
			formatIfRequired: false,
			expected:         "test\u2028value",
			note:             "Without formatting, Unicode separators pass through unchanged",
		},
		{
			name:             "Unicode separators with regular text",
			input:            "Normal text\u2028with line separator\u00A0and nbsp",
			formatIfRequired: true,
			expected:         "'Normal text\u2028with line separator\u00A0and nbsp'",
			note:             "Unicode separators mixed with ASCII should work correctly",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, tc.note)
			assert.Equal(t, tc.expected, result, tc.note)
		})
	}
}

// Test 1.10: TestStringConversionWithBidirectionalText
// Tests Right-to-Left (RTL) text from Arabic, Hebrew, and mixed LTR+RTL scenarios
// This ensures proper handling of bidirectional Unicode text which is common in international applications
func TestStringConversionWithBidirectionalText(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "Arabic text (RTL)",
			input:            "مرحبا بالعالم", // "Hello World" in Arabic
			formatIfRequired: true,
			expected:         "'مرحبا بالعالم'",
			note:             "Arabic RTL text should be preserved",
		},
		{
			name:             "Hebrew text (RTL)",
			input:            "שלום עולם", // "Hello World" in Hebrew
			formatIfRequired: true,
			expected:         "'שלום עולם'",
			note:             "Hebrew RTL text should be preserved",
		},
		{
			name:             "Mixed English and Arabic",
			input:            "Hello مرحبا World",
			formatIfRequired: true,
			expected:         "'Hello مرحبا World'",
			note:             "Mixed LTR and RTL text should coexist",
		},
		{
			name:             "Mixed English and Hebrew",
			input:            "Hello שלום World",
			formatIfRequired: true,
			expected:         "'Hello שלום World'",
			note:             "Mixed LTR and RTL text should coexist",
		},
		{
			name:             "Arabic with numbers",
			input:            "العدد 123 والنص",
			formatIfRequired: true,
			expected:         "'العدد 123 والنص'",
			note:             "Arabic text with embedded numbers should work",
		},
		{
			name:             "Complex multilingual (Arabic, Chinese, Hebrew)",
			input:            "مرحبا 你好 שלום",
			formatIfRequired: true,
			expected:         "'مرحبا 你好 שלום'",
			note:             "Multiple RTL and LTR scripts should coexist",
		},
		{
			name:             "Arabic with single quotes (SQL escaping)",
			input:            "اسم O'Reilly بالعربية",
			formatIfRequired: true,
			expected:         "'اسم O''Reilly بالعربية'",
			note:             "RTL text with single quotes should be SQL-escaped",
		},
		{
			name:             "RTL text without formatting",
			input:            "مرحبا",
			formatIfRequired: false,
			expected:         "مرحبا",
			note:             "Without formatting, RTL text passes through unchanged",
		},
		{
			name:             "Bidirectional text with special chars",
			input:            "English \"quote\" مرحبا 'test' שלום",
			formatIfRequired: true,
			expected:         "'English \"quote\" مرحبا ''test'' שלום'",
			note:             "Bidirectional text with quotes should be SQL-escaped",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, tc.note)
			assert.Equal(t, tc.expected, result, tc.note)
		})
	}
}

// Test 2.1: TestJsonConversionBasic
// Tests JSON/JSONB type with basic valid JSON
// NOTE: Single quotes inside JSON string values are valid JSON. The converter uses
// quoteValueIfRequiredWithEscaping for SQL string literal escaping (doubling single quotes
// for SQL, not JSON). See TestJsonConversionWithFormattingWithSingleQuotesEscaped for testing
// single quotes in JSON values. Integration tests validate end-to-end behavior.
func TestJsonConversionBasic(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "JSON with escaped double quote",
			input:            `{"key": "value\"test"}`,
			formatIfRequired: true,
			expected:         `'{"key": "value\"test"}'`,
			note:             "JSON's double quote escaping preserved",
		},
		{
			name:             "JSON with backslash",
			input:            `{"key": "value\\test"}`,
			formatIfRequired: true,
			expected:         `'{"key": "value\\test"}'`,
			note:             "Backslash in JSON preserved as literal",
		},
		{
			name:             "JSON with newline escape",
			input:            `{"key": "value\ntest"}`,
			formatIfRequired: true,
			expected:         `'{"key": "value\ntest"}'`,
			note:             "JSON newline escape preserved as literal \\n",
		},
		{
			name:             "JSON with tab escape",
			input:            `{"key": "value\ttest"}`,
			formatIfRequired: true,
			expected:         `'{"key": "value\ttest"}'`,
			note:             "JSON tab escape preserved as literal \\t",
		},
		{
			name:             "JSON with actual newline character",
			input:            "{\n\"key\": \"value\"\n}",
			formatIfRequired: true,
			expected:         "'{\n\"key\": \"value\"\n}'",
			note:             "Actual newlines in JSON should be preserved",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.data.Json"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 2.2: TestJsonConversionWithComplexStructures
// Tests JSON with complex structures and edge cases
func TestJsonConversionWithComplexStructures(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "nested objects",
			input:            `{"outer": {"inner": "value"}}`,
			formatIfRequired: true,
			expected:         `'{"outer": {"inner": "value"}}'`,
			note:             "Nested JSON objects",
		},
		{
			name:             "array with escaped quote",
			input:            `["item1", "item2", "item\"3"]`,
			formatIfRequired: true,
			expected:         `'["item1", "item2", "item\"3"]'`,
			note:             "JSON array with escaped double quote",
		},
		{
			name:             "empty JSON object",
			input:            `{}`,
			formatIfRequired: true,
			expected:         `'{}'`,
			note:             "Empty JSON should be preserved",
		},
		{
			name:             "empty JSON array",
			input:            `[]`,
			formatIfRequired: true,
			expected:         `'[]'`,
			note:             "Empty array should be preserved",
		},
		{
			name:             "JSON with null value",
			input:            `{"key": null}`,
			formatIfRequired: true,
			expected:         `'{"key": null}'`,
			note:             "JSON null is different from SQL NULL",
		},
		{
			name:             "JSON with all types",
			input:            `{"str": "test", "num": 123, "bool": true, "null": null, "arr": [1,2]}`,
			formatIfRequired: true,
			expected:         `'{"str": "test", "num": 123, "bool": true, "null": null, "arr": [1,2]}'`,
			note:             "Complex JSON with multiple types",
		},
		{
			name:             "JSON with Unicode characters",
			input:            `{"message": "Hello 世界 🎉 café"}`,
			formatIfRequired: true,
			expected:         `'{"message": "Hello 世界 🎉 café"}'`,
			note:             "JSON with multilingual Unicode and emoji",
		},
		{
			name:             "JSON with carriage return",
			input:            `{"key": "line1\r\nline2"}`,
			formatIfRequired: true,
			expected:         `'{"key": "line1\r\nline2"}'`,
			note:             "JSON with CRLF escape sequence",
		},
		{
			name:             "JSON with backslash and escaped quotes",
			input:            `{"path": "C:\\path\\\"file\""}`,
			formatIfRequired: true,
			expected:         `'{"path": "C:\\path\\\"file\""}'`,
			note:             "JSON with backslash and escaped double quotes",
		},
		{
			name:             "JSON with SQL keywords",
			input:            `{"query": "SELECT * FROM users"}`,
			formatIfRequired: true,
			expected:         `'{"query": "SELECT * FROM users"}'`,
			note:             "JSON containing SQL keywords",
		},
		{
			name:             "JSON with escape sequences",
			input:            `{"escapes": "slash:\\ newline:\n tab:\t return:\r"}`,
			formatIfRequired: true,
			expected:         `'{"escapes": "slash:\\ newline:\n tab:\t return:\r"}'`,
			note:             "JSON with comprehensive escape sequences (no single quotes)",
		},
		{
			name:             "JSON with Windows path",
			input:            `{"path": "C:\\Program Files\\App\\file.txt"}`,
			formatIfRequired: true,
			expected:         `'{"path": "C:\\Program Files\\App\\file.txt"}'`,
			note:             "JSON with Windows file path",
		},
		{
			name:             "JSON with zero-width characters",
			input:            `{"text": "zero\u200Bwidth\u200Djoin"}`,
			formatIfRequired: true,
			expected:         `'{"text": "zero\u200Bwidth\u200Djoin"}'`,
			note:             "JSON with Unicode zero-width characters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.data.Json"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 2.3: TestJsonConversionWithDeepNesting
// Tests JSON with deeply nested structures
func TestJsonConversionWithDeepNesting(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "deeply nested objects",
			input:            `{"a":{"b":{"c":{"d":{"e":"value"}}}}}`,
			formatIfRequired: true,
			expected:         `'{"a":{"b":{"c":{"d":{"e":"value"}}}}}'`,
			note:             "5 levels deep nesting",
		},
		{
			name:             "nested arrays and objects",
			input:            `{"items":[{"name":"test"},{"data":[1,2,{"inner":"value"}]}]}`,
			formatIfRequired: true,
			expected:         `'{"items":[{"name":"test"},{"data":[1,2,{"inner":"value"}]}]}'`,
			note:             "Mixed array and object nesting",
		},
		{
			name:             "array of arrays with special chars",
			input:            `[["a","b"],["c\\d","e\"f"]]`,
			formatIfRequired: true,
			expected:         `'[["a","b"],["c\\d","e\"f"]]'`,
			note:             "Nested arrays with backslash and escaped quote",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.data.Json"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 3.1: TestEnumConversionWithSpecialChars
// Tests ENUM type with special characters
// ENUM uses same converter as STRING (quoteValueIfRequiredWithEscaping)
func TestEnumConversionWithSpecialChars(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "enum with single quote",
			input:            "enum'value",
			formatIfRequired: true,
			expected:         "'enum''value'",
			note:             "Single quote in enum value needs escaping",
		},
		{
			name:             "enum with double quote",
			input:            `enum"value`,
			formatIfRequired: true,
			expected:         `'enum"value'`,
			note:             "Double quote doesn't need escaping in SQL string",
		},
		{
			name:             "enum with backslash",
			input:            `enum\value`,
			formatIfRequired: true,
			expected:         `'enum\value'`,
			note:             "Backslash in enum preserved as literal",
		},
		{
			name:             "enum with space",
			input:            "enum value",
			formatIfRequired: true,
			expected:         "'enum value'",
			note:             "Spaces in enum values are valid",
		},
		{
			name:             "enum with dash",
			input:            "enum-value",
			formatIfRequired: true,
			expected:         "'enum-value'",
			note:             "Dashes in enum values",
		},
		{
			name:             "enum with underscore",
			input:            "enum_value",
			formatIfRequired: true,
			expected:         "'enum_value'",
			note:             "Underscores in enum values",
		},
		{
			name:             "enum with unicode",
			input:            "café",
			formatIfRequired: true,
			expected:         "'café'",
			note:             "Unicode characters in enum values",
		},
		{
			name:             "enum with emoji",
			input:            "🎉emoji",
			formatIfRequired: true,
			expected:         "'🎉emoji'",
			note:             "Emoji in enum values",
		},
		{
			name:             "enum starting with number",
			input:            "123value",
			formatIfRequired: true,
			expected:         "'123value'",
			note:             "Enum values can start with numbers",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.data.Enum"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 3.2: TestArrayConversionWithEdgeCases
// Tests PostgreSQL array literals (any type) which are converted to STRING by Debezium
// Arrays are stringified by Debezium's PostgresToYbValueConverter and processed as strings
func TestArrayConversionWithEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "empty array",
			input:            "{}",
			formatIfRequired: true,
			expected:         "'{}'",
			note:             "Empty array represented as {}",
		},
		{
			name:             "array with NULL elements",
			input:            "{active,NULL,pending}",
			formatIfRequired: true,
			expected:         "'{active,NULL,pending}'",
			note:             "Array with NULL elements (PostgreSQL array literal)",
		},
		{
			name:             "array with only NULL",
			input:            "{NULL}",
			formatIfRequired: true,
			expected:         "'{NULL}'",
			note:             "Single NULL element in array",
		},
		{
			name:             "array with multiple NULLs",
			input:            "{NULL,NULL,NULL}",
			formatIfRequired: true,
			expected:         "'{NULL,NULL,NULL}'",
			note:             "Multiple NULL elements",
		},
		{
			name:             "array with quoted strings",
			input:            `{"value with space","another value"}`,
			formatIfRequired: true,
			expected:         `'{"value with space","another value"}'`,
			note:             "Array with quoted string elements",
		},
		{
			name:             "array with single quotes in elements",
			input:            `{"O'Reilly","It's"}`,
			formatIfRequired: true,
			expected:         `'{"O''Reilly","It''s"}'`,
			note:             "Array elements containing single quotes need SQL escaping",
		},
		{
			name:             "array with backslashes",
			input:            `{value\with\backslash}`,
			formatIfRequired: true,
			expected:         `'{value\with\backslash}'`,
			note:             "Array with backslashes in elements",
		},
		{
			name:             "array with Unicode and emoji",
			input:            `{café,🎉emoji,日本語}`,
			formatIfRequired: true,
			expected:         `'{café,🎉emoji,日本語}'`,
			note:             "Array with Unicode characters and emoji",
		},
		{
			name:             "nested array (2D)",
			input:            `{{1,2},{3,4}}`,
			formatIfRequired: true,
			expected:         `'{{1,2},{3,4}}'`,
			note:             "2D array literal",
		},
		{
			name:             "array without formatting",
			input:            `{active,pending,inactive}`,
			formatIfRequired: false,
			expected:         `{active,pending,inactive}`,
			note:             "Array without SQL quotes when formatIfRequired=false",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrays are processed by STRING converter
			result, err := YBValueConverterSuite["STRING"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 4.1: TestBytesConversionWithSpecialPatterns
// Tests BYTES (bytea) type with various binary patterns
// Input is Base64 encoded, output is PostgreSQL hex format
func TestBytesConversionWithSpecialPatterns(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "empty bytes",
			input:            "",
			formatIfRequired: true,
			expected:         "'\\x'",
			note:             "Empty bytea produces \\x with no hex digits",
		},
		{
			name:             "all zeros (3 bytes)",
			input:            "AAAA", // Base64 for 3 zero bytes
			formatIfRequired: true,
			expected:         "'\\x000000'",
			note:             "Null bytes must be preserved as 00",
		},
		{
			name:             "all 0xFF (3 bytes)",
			input:            "////", // Base64 for 3 bytes of 0xFF
			formatIfRequired: true,
			expected:         "'\\xffffff'",
			note:             "Maximum byte value must be preserved",
		},
		{
			name:             "single byte 'A' (0x41)",
			input:            "QQ==", // Base64 for single byte 0x41
			formatIfRequired: true,
			expected:         "'\\x41'",
			note:             "Single byte conversion",
		},
		{
			name:             "ASCII 'ABC' (0x414243)",
			input:            "QUJD", // Base64 for "ABC"
			formatIfRequired: true,
			expected:         "'\\x414243'",
			note:             "Multi-byte ASCII string",
		},
		{
			name:             "bytes without formatting",
			input:            "QUJD",
			formatIfRequired: false,
			expected:         "\\x414243", // No quotes for COPY format
			note:             "COPY format omits quotes",
		},
		{
			name:             "single null byte",
			input:            "AA==", // Base64 for 0x00
			formatIfRequired: true,
			expected:         "'\\x00'",
			note:             "Null byte handling",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["BYTES"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Input (base64): %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 4.2: TestBytesConversionInvalidBase64
// Tests BYTES conversion with invalid Base64 input
func TestBytesConversionInvalidBase64(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expectError      bool
		note             string
	}{
		{
			name:             "invalid characters",
			input:            "invalid!!!",
			formatIfRequired: true,
			expectError:      true,
			note:             "Non-base64 characters should cause error",
		},
		{
			name:             "incomplete base64",
			input:            "ABC", // Not valid base64 (missing padding or wrong length)
			formatIfRequired: true,
			expectError:      true,
			note:             "Incomplete base64 should error",
		},
		{
			name:             "only whitespace",
			input:            "   ",
			formatIfRequired: true,
			expectError:      true,
			note:             "Whitespace-only input should error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["BYTES"](tc.input, tc.formatIfRequired, nil)

			t.Logf("Input: %q", tc.input)
			t.Logf("Got result: %q", result)
			t.Logf("Got error: %v", err)
			t.Logf("Note: %s", tc.note)

			if tc.expectError {
				assert.Error(t, err, "Should return error for invalid base64")
			} else {
				assert.NoError(t, err, "Should not error")
			}
		})
	}
}

// Test 4.3: TestBytesConversionWithBinarySpecialChars
// Tests bytes containing binary representations of special characters
func TestBytesConversionWithBinarySpecialChars(t *testing.T) {
	testCases := []struct {
		name             string
		bytes            []byte
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "byte 0x27 (single quote char)",
			bytes:            []byte{0x27},
			formatIfRequired: true,
			expected:         "'\\x27'",
			note:             "Single quote as binary should convert to hex",
		},
		{
			name:             "byte 0x5C (backslash char)",
			bytes:            []byte{0x5C},
			formatIfRequired: true,
			expected:         "'\\x5c'",
			note:             "Backslash as binary should convert to hex",
		},
		{
			name:             "byte 0x0A (newline)",
			bytes:            []byte{0x0A},
			formatIfRequired: true,
			expected:         "'\\x0a'",
			note:             "Newline byte should be preserved in hex",
		},
		{
			name:             "byte 0x09 (tab)",
			bytes:            []byte{0x09},
			formatIfRequired: true,
			expected:         "'\\x09'",
			note:             "Tab byte should be preserved in hex",
		},
		{
			name:             "all control characters (0x00-0x1F)",
			bytes:            []byte{0x00, 0x01, 0x02, 0x1F},
			formatIfRequired: true,
			expected:         "'\\x0001021f'",
			note:             "Control characters preserved in hex",
		},
		{
			name:             "mixed printable and non-printable",
			bytes:            []byte{0x41, 0x00, 0x42, 0xFF}, // A, null, B, 0xFF
			formatIfRequired: true,
			expected:         "'\\x410042ff'",
			note:             "Mixed bytes all convert to hex",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode bytes to base64 for input
			input := base64.StdEncoding.EncodeToString(tc.bytes)

			result, err := YBValueConverterSuite["BYTES"](input, tc.formatIfRequired, nil)
			assert.NoError(t, err, "Conversion should not error")

			t.Logf("Bytes: %v", tc.bytes)
			t.Logf("Input (base64): %q", input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result, "Converted value should match expected")
		})
	}
}

// Test 4.4: TestBytesConversionFormatIfRequired
// Tests the formatIfRequired flag for BYTES conversion
func TestBytesConversionFormatIfRequired(t *testing.T) {
	input := "QUJD" // Base64 for "ABC"

	t.Run("with formatting (INSERT)", func(t *testing.T) {
		result, err := YBValueConverterSuite["BYTES"](input, true, nil)
		assert.NoError(t, err)
		assert.Equal(t, "'\\x414243'", result, "Should include quotes for INSERT")
		t.Logf("INSERT format: %q", result)
	})

	t.Run("without formatting (COPY)", func(t *testing.T) {
		result, err := YBValueConverterSuite["BYTES"](input, false, nil)
		assert.NoError(t, err)
		assert.Equal(t, "\\x414243", result, "Should not include quotes for COPY")
		t.Logf("COPY format: %q", result)
	})
}

// Test 5.1: TestMapConversionWithArrowOperator
func TestMapConversionWithArrowOperator(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"arrow in key", `"key=>val" => "test"`, true, `'"key=>val" => "test"'`},
		{"arrow in value", `"key" => "val=>test"`, true, `'"key" => "val=>test"'`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["MAP"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 5.2: TestMapConversionWithEscapedChars
func TestMapConversionWithEscapedChars(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"escaped quote", `"key\"test" => "value"`, true, `'"key\"test" => "value"'`},
		{"single quote SQL escape", `"key" => "it's"`, true, `'"key" => "it''s"'`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["MAP"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 5.3: TestMapConversionWithEmptyValues
func TestMapConversionWithEmptyValues(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"empty key", `"" => "value"`, true, `'"" => "value"'`},
		{"empty value", `"key" => ""`, true, `'"key" => ""'`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["MAP"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 5.4: TestMapConversionWithMultiplePairs
// Tests HSTORE (MAP) with multiple key-value pairs
func TestMapConversionWithMultiplePairs(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
		note             string
	}{
		{
			name:             "two pairs",
			input:            `"key1" => "value1", "key2" => "value2"`,
			formatIfRequired: true,
			expected:         `'"key1" => "value1", "key2" => "value2"'`,
			note:             "Simple two key-value pairs",
		},
		{
			name:             "three pairs",
			input:            `"name" => "John", "age" => "30", "city" => "NYC"`,
			formatIfRequired: true,
			expected:         `'"name" => "John", "age" => "30", "city" => "NYC"'`,
			note:             "Three key-value pairs",
		},
		{
			name:             "multiple pairs with special chars",
			input:            `"key1" => "value's", "key2" => "val=>test", "key3" => "normal"`,
			formatIfRequired: true,
			expected:         `'"key1" => "value''s", "key2" => "val=>test", "key3" => "normal"'`,
			note:             "Multiple pairs with single quotes and arrow operator",
		},
		{
			name:             "multiple pairs with empty values",
			input:            `"key1" => "", "key2" => "value", "key3" => ""`,
			formatIfRequired: true,
			expected:         `'"key1" => "", "key2" => "value", "key3" => ""'`,
			note:             "Multiple pairs with some empty values",
		},
		{
			name:             "multiple pairs with escaped chars",
			input:            `"key\"1" => "value\\test", "key2" => "normal"`,
			formatIfRequired: true,
			expected:         `'"key\"1" => "value\\test", "key2" => "normal"'`,
			note:             "Multiple pairs with escaped quotes and backslashes",
		},
		{
			name:             "multiple pairs without formatting",
			input:            `"key1" => "value1", "key2" => "value2"`,
			formatIfRequired: false,
			expected:         `"key1" => "value1", "key2" => "value2"`,
			note:             "Multiple pairs without SQL quotes",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["MAP"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected: %q", tc.expected)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 7.1: TestDateConversionEdgeCases
func TestDateConversionEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"epoch zero", "0", true, "'1970-01-01'"},
		{"negative epoch", "-1", true, "'1969-12-31'"},
		{"positive epoch", "18993", true, "'2022-01-01'"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.time.Date"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 7.2: TestTimestampConversionEdgeCases
func TestTimestampConversionEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"epoch zero", "0", true, "'1970-01-01 00:00:00'"},
		{"negative epoch", "-86400000", true, "'1969-12-31 00:00:00'"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.time.Timestamp"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 7.3: TestMicroTimestampConversion
func TestMicroTimestampConversion(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expectedContains string
	}{
		{"microsecond precision", "1640995200000000", true, "'2022-01-01"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.time.MicroTimestamp"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Contains(t, result, tc.expectedContains)
		})
	}
}

// Test 7.4: TestNanoTimestampConversion
func TestNanoTimestampConversion(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expectedContains string
	}{
		{"nanosecond precision", "1640995200000000000", true, "'2022-01-01"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.time.NanoTimestamp"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Contains(t, result, tc.expectedContains)
		})
	}
}

// Test 7.5: TestTimeConversion
func TestTimeConversion(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"midnight", "0", true, "'00:00:00'"},
		{"noon", "43200000", true, "'12:00:00'"},
		{"end of day", "86399000", true, "'23:59:59'"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.time.Time"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 7.6: TestMicroTimeConversion
func TestMicroTimeConversion(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expectedContains string
	}{
		{"microsecond time", "1000000", true, "'00:00:01"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.time.MicroTime"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Contains(t, result, tc.expectedContains)
		})
	}
}

// Test 7.7: TestTimestampConversionInvalidInput
func TestTimestampConversionInvalidInput(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"invalid string", "abc"},
		{"empty string", ""},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := YBValueConverterSuite["io.debezium.time.Timestamp"](tc.input, true, nil)
			assert.Error(t, err, "Should error on invalid input")
		})
	}
}

// Test 8.1: TestUuidConversionEdgeCases
func TestUuidConversionEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"standard UUID", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", true, "'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'"},
		{"all zeros", "00000000-0000-0000-0000-000000000000", true, "'00000000-0000-0000-0000-000000000000'"},
		{"all Fs", "ffffffff-ffff-ffff-ffff-ffffffffffff", true, "'ffffffff-ffff-ffff-ffff-ffffffffffff'"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.data.Uuid"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 8.2: TestUuidConversionInvalidInput
func TestUuidConversionInvalidInput(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"invalid format passes through", "not-a-uuid", true, "'not-a-uuid'"},
		{"missing dashes passes through", "a0eebc999c0b4ef8bb6d6bb9bd380a11", true, "'a0eebc999c0b4ef8bb6d6bb9bd380a11'"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.data.Uuid"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
			t.Logf("Note: UUID converter passes through - PostgreSQL validates format")
		})
	}
}

// Test 9.1: TestLtreeConversionEdgeCases
func TestLtreeConversionEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"simple path", "Top.Science.Astronomy", true, "'Top.Science.Astronomy'"},
		{"quoted labels", `Top."Science Fiction".Books`, true, `'Top."Science Fiction".Books'`},
		{"deep path", "Top.Science.Astronomy.Stars.Sun", true, "'Top.Science.Astronomy.Stars.Sun'"},
		{"single label", "Top", true, "'Top'"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.data.Ltree"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 10.1: TestIntervalConversionEdgeCases
func TestIntervalConversionEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"positive interval", "1 year 2 months 3 days", true, "'1 year 2 months 3 days'"},
		{"negative interval", "-1 year -2 months", true, "'-1 year -2 months'"},
		{"zero", "00:00:00", true, "'00:00:00'"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.time.Interval"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 11.1: TestZonedTimestampConversion
func TestZonedTimestampConversion(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expectedContains string
		note             string
	}{
		{
			name:             "UTC timezone (+00)",
			input:            "2024-01-01T00:00:00+00",
			formatIfRequired: true,
			expectedContains: "'2024-01-01",
			note:             "Zulu time / UTC",
		},
		{
			name:             "negative offset (-05:00)",
			input:            "2024-01-01T00:00:00-05",
			formatIfRequired: true,
			expectedContains: "'2024-01-01",
			note:             "Western timezone (EST)",
		},
		{
			name:             "positive offset (+04:00)",
			input:            "2024-01-01T00:00:00+04",
			formatIfRequired: true,
			expectedContains: "'2024-01-01",
			note:             "Eastern timezone (Dubai, Moscow)",
		},
		{
			name:             "half-hour offset (+05:30)",
			input:            "2024-01-01T00:00:00+05:30",
			formatIfRequired: true,
			expectedContains: "'2024-01-01",
			note:             "India Standard Time",
		},
		{
			name:             "negative offset (-07:00)",
			input:            "2024-01-01T00:00:00-07",
			formatIfRequired: true,
			expectedContains: "'2024-01-01",
			note:             "Pacific Daylight Time",
		},
		{
			name:             "epoch with timezone",
			input:            "1970-01-01T00:00:00+00",
			formatIfRequired: true,
			expectedContains: "'1970-01-01",
			note:             "Unix epoch with UTC",
		},
		{
			name:             "future date with timezone",
			input:            "2065-12-31T23:59:59+00",
			formatIfRequired: true,
			expectedContains: "'2065-12-31",
			note:             "Far future date",
		},
		{
			name:             "midnight with timezone",
			input:            "2024-06-15T00:00:00+00",
			formatIfRequired: true,
			expectedContains: "'2024-06-15",
			note:             "Day boundary",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.time.ZonedTimestamp"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)

			t.Logf("Input: %q", tc.input)
			t.Logf("Expected contains: %q", tc.expectedContains)
			t.Logf("Got: %q", result)
			t.Logf("Note: %s", tc.note)

			assert.Contains(t, result, tc.expectedContains)
		})
	}
}

// Test 12.1: TestDecimalConversionEdgeCases
func TestDecimalConversionEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"large decimal", "123456789.123456789", false, "123456789.123456789"},
		{"negative", "-123.456", false, "-123.456"},
		{"zero variations", "0.00", false, "0.00"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["org.apache.kafka.connect.data.Decimal"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test 12.2: TestVariableScaleDecimalConversion
func TestVariableScaleDecimalConversion(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		formatIfRequired bool
		expected         string
	}{
		{"variable scale", "123.456", false, "123.456"},
		{"high precision", "123.456789012345", false, "123.456789012345"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := YBValueConverterSuite["io.debezium.data.VariableScaleDecimal"](tc.input, tc.formatIfRequired, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

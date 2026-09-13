# YB Voyager DDL Extractor

This tool extracts anonymized DDL statements from YB Voyager callhome payload JSON files and creates executable SQL files.

## Overview

When YB Voyager performs schema assessment, it can optionally send anonymized DDL statements as part of its callhome telemetry. This tool allows you to extract those DDL statements from the callhome payload and create SQL files that can be executed for testing or analysis purposes.

## Features

- ✅ Extracts `anonymized_ddls` from callhome payload JSON
- ✅ Creates clean, executable SQL files
- ✅ Handles nested JSON structure automatically
- ✅ Simple and lightweight - no external dependencies
- ✅ Robust error handling

## Files

- `extract_anonymized_ddls.py` - Main Python script
- `README_DDL_EXTRACTOR.md` - This documentation

## Prerequisites

- Python 3.6 or higher
- `psql` (optional, for SQL validation)
- Valid YB Voyager callhome payload JSON file

## Usage

```bash
# Extract DDLs to default output file
python3 extract_anonymized_ddls.py callhome_payload.json

# Specify custom output file
python3 extract_anonymized_ddls.py callhome_payload.json my_ddls.sql
```

## Command Line Options

| Option | Description |
|--------|-------------|
| `--help`, `-h` | Show help message |

## Input JSON Structure

The tool expects a YB Voyager callhome payload JSON with this structure:

```json
{
  "migration_phase": "ASSESS_MIGRATION",
  "phase_payload": "{\"payload_version\":\"1.7\",\"anonymized_ddls\":[\"CREATE TABLE table_abc123 (...)\", \"CREATE INDEX index_def456 (...)\"]}"
}
```

The `anonymized_ddls` field contains an array of anonymized DDL statements.

## Output SQL File Format

Generated SQL files include:

```sql
-- YB Voyager Anonymized DDL Statements
-- Extracted on: 2024-01-15 14:30:25
-- Total DDL statements: 42
-- 
-- This file contains anonymized DDL statements from YB Voyager callhome payload.
-- All sensitive information (table names, column names, etc.) has been anonymized.
-- 

-- DDL Statement 1
CREATE TABLE schema_abc123.table_def456 (
    col_ghi789 INTEGER PRIMARY KEY,
    col_jkl012 TEXT NOT NULL
);

-- DDL Statement 2
CREATE INDEX index_mno345 ON schema_abc123.table_def456 (col_ghi789);

-- End of anonymized DDL statements
```

## Examples

### Example 1: Basic Extraction

```bash
# Extract DDLs from callhome payload
python3 extract_anonymized_ddls.py /path/to/callhome_payload.json

# Output: anonymized_ddls_callhome_payload.sql
```

### Example 2: Custom Output with Validation

```bash
# Extract with custom filename and validation
python3 extract_anonymized_ddls.py --verbose --validate callhome_payload.json extracted_schema.sql
```

### Example 3: Statistics Only

```bash
# Just show statistics without creating SQL file
python3 extract_anonymized_ddls.py --stats-only callhome_payload.json
```

Output:
```
=== Extraction Statistics ===
Total DDLs found:     156
Successfully extracted: 152
Empty DDLs skipped:   2
Failed DDLs:          2
Success rate:         97.4%
```

## Running the Extracted SQL

Once you have the SQL file, you can execute it against a PostgreSQL database:

```bash
# Run against local database
psql -d mydb -f anonymized_ddls.sql

# Run against remote database
psql -h hostname -U username -d database -f anonymized_ddls.sql

# With additional options
psql -h hostname -U username -d database \
     --set=ON_ERROR_STOP=1 \
     --echo-queries \
     -f anonymized_ddls.sql
```

## Troubleshooting

### Common Issues

1. **"No 'phase_payload' field found"**
   - The JSON file may not be a valid callhome payload
   - Check that you're using the correct JSON file from YB Voyager

2. **"Invalid JSON in phase_payload"**
   - The nested JSON string is malformed
   - This could indicate a truncated or corrupted payload

3. **"No anonymized_ddls found in payload"**
   - The payload may be from a migration phase that doesn't include DDLs
   - Ensure DDL anonymization was enabled (`SEND_ANONYMIZED_DDLS=true`)
   - Check that the source database is PostgreSQL (DDL anonymization is PostgreSQL-only)

4. **SQL validation failures**
   - Some anonymized DDLs may have parsing issues
   - Use `--verbose` to see which specific DDLs are problematic
   - Consider running without `--validate` and manually reviewing the SQL

### Debug Mode

Enable verbose logging to see detailed information:

```bash
python3 extract_anonymized_ddls.py --verbose callhome_payload.json
```

This will show:
- JSON loading progress
- DDL extraction details
- Processing status for each DDL
- Detailed error messages

## How DDL Anonymization Works

The anonymized DDLs are created by YB Voyager during schema assessment:

1. **Collection**: DDL statements are collected from exported schema files
2. **Parsing**: Each DDL is parsed into an Abstract Syntax Tree (AST)
3. **Anonymization**: All identifiers (table names, column names, etc.) are replaced with anonymized tokens
4. **Hashing**: Anonymized tokens are generated using SHA-256 with a migration-specific salt
5. **Consistency**: The same identifier always produces the same anonymized token within a migration

### Anonymization Prefixes

Different database objects get different prefixes:
- Tables: `table_abc123`
- Columns: `col_def456` 
- Indexes: `index_ghi789`
- Schemas: `schema_jkl012`
- Constraints: `constraint_mno345`

## Security Considerations

- ✅ All sensitive identifiers are anonymized
- ✅ Consistent anonymization within a migration
- ✅ Salt-based hashing prevents reverse engineering
- ✅ No sensitive data is exposed in error messages
- ⚠️ The structure and relationships of your schema are preserved
- ⚠️ Data types and constraints are not anonymized

## Contributing

To improve this tool:

1. Add support for other database types (MySQL, Oracle)
2. Enhance SQL validation with more sophisticated checks
3. Add support for batch processing multiple JSON files
4. Implement DDL filtering and categorization
5. Add integration with schema comparison tools

## License

This tool is part of the YB Voyager project and follows the same licensing terms.

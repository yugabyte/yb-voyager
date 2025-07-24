# MCP Server New - Basic Implementation

A basic Model Context Protocol (MCP) server for YB Voyager, built with a config-first approach.

## Current Status

### What's Implemented
- Basic MCP server infrastructure using `mcp-go` library
- Server can start up and run on stdio transport
- Integration with YB Voyager CLI as `start-mcp-server` command
- **Config validation tool** - validates config files using shared YB Voyager validation logic
- **SQL parsing tool** - parses and validates SQL statements using PostgreSQL parser (pg_query_go)
- **Config schema tool** - provides schema information for YB Voyager configuration sections
- Basic test coverage

### Tools Available
1. **`validate_config`** - Validates that a config file exists, is readable, and has valid YAML content
   - **Parameter**: `config_path` (required) - Path to the config file to validate
   - **Returns**: JSON with validation status and config file information

2. **`parse_sql`** - Parses and validates a SQL statement using PostgreSQL parser
   - **Parameter**: `sql_statement` (required) - SQL statement to parse and validate
   - **Returns**: JSON with parsing status, error details (if invalid), and parse tree information (if valid)

3. **`get_config_schema`** - Get available configuration keys and schema information for YB Voyager config sections
   - **Parameter**: `section` (optional) - Configuration section name (e.g., source, target, export-data). If not provided, returns all available sections.
   - **Returns**: JSON with schema information including all available keys, descriptions, and counts. When no section is provided, returns a list of all available sections with descriptions.

4. **`assess_migration`** - Execute YB Voyager assess-migration command
   - **Parameter**: `config_path` (required) - Path to the config file containing source and assess-migration sections
   - **Parameter**: `additional_args` (optional) - Additional command line arguments
   - **Returns**: JSON with execution results including success status, duration, output, and any errors

5. **`assess_migration_async`** - Execute YB Voyager assess-migration command asynchronously with real-time output
   - **Parameter**: `config_path` (required) - Path to the config file containing source and assess-migration sections
   - **Parameter**: `additional_args` (optional) - Additional command line arguments
   - **Returns**: JSON with execution ID for tracking long-running commands

6. **`get_command_status`** - Get status and progress of a running command
   - **Parameter**: `execution_id` (required) - Execution ID returned by async command
   - **Returns**: JSON with current status, progress lines, and execution details

7. **`analyze_schema`** - Execute YB Voyager analyze-schema command synchronously
   - **Parameter**: `config_path` (required) - Path to the config file containing source and analyze-schema sections
   - **Parameter**: `additional_args` (optional) - Additional command line arguments
   - **Returns**: JSON with execution details

8. **`analyze_schema_async`** - Execute YB Voyager analyze-schema command asynchronously
   - **Parameter**: `config_path` (required) - Path to the config file containing source and analyze-schema sections
   - **Parameter**: `additional_args` (optional) - Additional command line arguments
   - **Returns**: JSON with execution ID for tracking long-running commands

9. **`stop_command`** - Stop/cancel a running command execution
   - **Parameter**: `execution_id` (required) - Execution ID of the running command
   - **Returns**: JSON confirming the command was stopped

### Files
- `server.go` - MCP server implementation with tool registration
- `config_validator.go` - Config file validation logic
- `config_validator_test.go` - Tests for config validation
- `sql_parser.go` - SQL parsing and validation logic
- `sql_parser_test.go` - Tests for SQL parsing
- `config_schema.go` - Config schema information provider
- `config_schema_test.go` - Tests for config schema
- `command_executor.go` - YB Voyager command execution logic
- `command_executor_test.go` - Tests for command execution
- `example-config.yaml` - Example configuration file for testing
- `README.md` - This documentation file

### Usage
```bash
# Build and run
cd yb-voyager
go build -o yb-voyager .

# Start MCP server
./yb-voyager start-mcp-server
```

### Testing
```bash
# Run tests
cd yb-voyager/src/mcp-server-new
go test -v
```

## Config Validation Features
- ✅ Uses `utils.FileOrFolderExists()` to check if config file exists
- ✅ Uses `utils.IsFileEmpty()` to check if file is empty
- ✅ Validates YAML format using viper
- ✅ Extracts config sections and keys
- ✅ Returns detailed file information (size, modification time, etc.)
- ✅ Uses shared YB Voyager validation logic from `utils/config` package
- ✅ Comprehensive validation of all allowed keys and sections
- ✅ Validates mutually exclusive sections
- ✅ No code duplication - reuses existing validation logic

## SQL Parsing Features
- ✅ Uses `pg_query_go` library for PostgreSQL-compatible SQL parsing
- ✅ Validates SQL syntax and returns detailed error messages
- ✅ Returns parse tree information for valid SQL statements
- ✅ Handles multiple SQL statements in a single string
- ✅ Provides statement count and length information
- ✅ Comprehensive error reporting for invalid SQL
- ✅ Supports complex DDL statements (CREATE TABLE, CREATE INDEX, etc.)

## Config Schema Features
- ✅ Provides schema information for all YB Voyager configuration sections
- ✅ Returns required vs optional keys for each section
- ✅ Includes descriptions for each section
- ✅ Handles case-insensitive section names
- ✅ Normalizes section names (spaces to hyphens)
- ✅ Provides helpful error messages with available sections for invalid requests
- ✅ Supports all 20+ configuration sections (source, target, export-data, import-data, etc.)

## Implementation Details
- **Reuses existing utilities**: Uses `utils.FileOrFolderExists()` and `utils.IsFileEmpty()`
- **Shared validation logic**: Uses `utils/config.ValidateConfigFile()` from shared package
- **No import cycles**: Validation logic is in utils package, accessible to both cmd and mcp-server-new
- **Follows YB Voyager patterns**: Uses the same validation approach as existing code
- **Comprehensive validation**: Checks file existence, readability, content, format, and all YB Voyager validation rules
- **SQL parsing integration**: Uses the same `pg_query_go` library as existing YB Voyager code
- **Schema information**: Based on the same validation rules used in YB Voyager

## Architecture
- **Shared validation**: `utils/config/validation.go` contains all validation logic
- **No duplication**: Both `cmd` and `mcp-server-new` use the same validation function
- **Clean separation**: MCP server focuses on MCP protocol, validation logic is shared
- **SQL parsing**: Uses `pg_query_go` library for PostgreSQL-compatible parsing
- **Schema provider**: Centralized schema information for all configuration sections

## Example Usage

### Config Validation
```json
{
  "tool": "validate_config",
  "parameters": {
    "config_path": "example-config.yaml"
  }
}
```

### SQL Parsing
```json
{
  "tool": "parse_sql", 
  "parameters": {
    "sql_statement": "SELECT * FROM users WHERE id = 1;"
  }
}
```

### Config Schema
```json
{
  "tool": "get_config_schema",
  "parameters": {
    "section": "source"
  }
}
```

**Get all available sections:**
```json
{
  "tool": "get_config_schema",
  "parameters": {}
}
```

### Assess Migration
```json
{
  "tool": "assess_migration",
  "parameters": {
    "config_path": "/path/to/config.yaml",
    "additional_args": "--yes --log-level info"
  }
}
```

### Async Assess Migration
```json
{
  "tool": "assess_migration_async",
  "parameters": {
    "config_path": "/path/to/config.yaml",
    "additional_args": "--yes --log-level info"
  }
}
```

### Get Command Status
```json
{
  "tool": "get_command_status",
  "parameters": {
    "execution_id": "exec_1732345678901234567"
  }
}
```

### Analyze Schema
```json
{
  "tool": "analyze_schema",
  "parameters": {
    "config_path": "/path/to/config.yaml",
    "additional_args": "--output-format json --log-level debug"
  }
}
```

### Async Analyze Schema
```json
{
  "tool": "analyze_schema_async",
  "parameters": {
    "config_path": "/path/to/config.yaml",
    "additional_args": "--output-format html --log-level info"
  }
}
```

### Stop Command
```json
{
  "tool": "stop_command",
  "parameters": {
    "execution_id": "exec_1732345678901234567"
  }
}
```

## Next Steps
- Add tools for command execution
- Add comprehensive error handling
- Add more test coverage 

# MCP Server New - Basic Implementation

A basic Model Context Protocol (MCP) server for YB Voyager, built with a config-first approach.

## Current Status

### What's Implemented
- Basic MCP server infrastructure using `mcp-go` library
- Server can start up and run on stdio transport
- Integration with YB Voyager CLI as `start-mcp-server` command
- **Config validation tool** - validates config files using shared YB Voyager validation logic
- **SQL parsing tool** - parses and validates SQL statements using PostgreSQL parser (pg_query_go)
- Basic test coverage

### Tools Available
1. **`validate_config`** - Validates that a config file exists, is readable, and has valid YAML content
   - **Parameter**: `config_path` (required) - Path to the config file to validate
   - **Returns**: JSON with validation status and config file information

2. **`parse_sql`** - Parses and validates a SQL statement using PostgreSQL parser
   - **Parameter**: `sql_statement` (required) - SQL statement to parse and validate
   - **Returns**: JSON with parsing status, error details (if invalid), and parse tree information (if valid)

### Files
- `server.go` - MCP server implementation with tool registration
- `config_validator.go` - Config file validation logic using shared utilities
- `config_validator_test.go` - Config validation tests
- `sql_parser.go` - SQL parsing logic using pg_query_go library
- `sql_parser_test.go` - SQL parsing tests
- `server_test.go` - Basic server tests
- `example-config.yaml` - Example config file for testing
- `README.md` - This documentation

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

## Implementation Details
- **Reuses existing utilities**: Uses `utils.FileOrFolderExists()` and `utils.IsFileEmpty()`
- **Shared validation logic**: Uses `utils/config.ValidateConfigFile()` from shared package
- **No import cycles**: Validation logic is in utils package, accessible to both cmd and mcp-server-new
- **Follows YB Voyager patterns**: Uses the same validation approach as existing code
- **Comprehensive validation**: Checks file existence, readability, content, format, and all YB Voyager validation rules
- **SQL parsing integration**: Uses the same `pg_query_go` library as existing YB Voyager code

## Architecture
- **Shared validation**: `utils/config/validation.go` contains all validation logic
- **No duplication**: Both `cmd` and `mcp-server-new` use the same validation function
- **Clean separation**: MCP server focuses on MCP protocol, validation logic is shared
- **SQL parsing**: Uses `pg_query_go` library for PostgreSQL-compatible parsing

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

## Next Steps
- Add tools for command execution
- Add comprehensive error handling
- Add more test coverage 

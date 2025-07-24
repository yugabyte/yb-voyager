# YB Voyager MCP Server - Cursor Integration

## Overview

This is a new MCP (Model Context Protocol) server implementation for YB Voyager, designed to provide AI agents with tools to interact with YB Voyager commands programmatically. The server follows a "config-first" approach where all operations primarily rely on configuration files.

## Current Status

### ✅ Implemented Tools

1. **`validate_config`** - Validates YB Voyager configuration files
2. **`parse_sql`** - Parses and validates SQL statements using PostgreSQL parser
3. **`get_config_schema`** - Retrieves configuration schema information
4. **`assess_migration`** - Executes YB Voyager assess-migration command synchronously with automatic `--yes` flag
5. **`assess_migration_async`** - Executes YB Voyager assess-migration command asynchronously with real-time output streaming
6. **`get_command_status`** - Gets the status and progress of running commands
7. **`stop_command`** - Stops/cancels a running command execution

### 🔄 In Progress

- None currently

### 📋 Planned Tools

1. **`export_schema`** - Export database schema
2. **`export_data`** - Export data from source database
3. **`import_schema`** - Import schema to target database
4. **`import_data`** - Import data to target database
5. **`analyze_schema`** - Analyze schema for migration
6. **`initiate_cutover`** - Initiate cutover to target
7. **`archive_changes`** - Archive migration changes
8. **`end_migration`** - End the migration process

## Architecture

### Core Components

- **`Server`** - Main MCP server that registers tools and handles requests
- **`CommandExecutor`** - Handles YB Voyager command execution with real-time output streaming
- **`ConfigValidator`** - Validates configuration files and their content
- **`SQLParser`** - Parses and validates SQL statements
- **`ConfigSchema`** - Provides configuration schema information

### Key Features

#### Real-time Output Streaming
- Commands execute asynchronously to prevent blocking the AI agent
- Output is streamed in real-time with timestamps
- Progress updates are captured and stored for later retrieval

#### Interactive Prompt Detection
- **Async Commands**: All async commands automatically include the `--yes` flag to avoid interactive prompts
- **Sync Commands**: Synchronous commands also automatically include the `--yes` flag for consistent behavior
- The system detects interactive prompts in command output (for manual handling if needed)
- Prompts are marked with `INTERACTIVE_PROMPT` in the progress logs
- **Note**: Commands are configured to avoid prompts automatically, but manual prompt handling is available if needed

#### Command Status Tracking
- Each command gets a unique execution ID
- Status updates: `running` → `completed`/`failed`/`cancelled`
- Progress logs with timestamps for debugging
- Commands can run for hours without timeout
- Commands can be manually stopped using `stop_command` tool

#### Config-First Approach
- All commands require a valid configuration file
- Comprehensive validation of config files and their content
- Support for all YB Voyager configuration sections

## File Structure

```
src/mcp-server-new/
├── server.go                 # Main MCP server implementation
├── command_executor.go       # Command execution with real-time streaming
├── command_executor_test.go  # Tests for command execution
├── config_validator.go       # Configuration file validation
├── config_validator_test.go  # Tests for config validation
├── sql_parser.go            # SQL parsing and validation
├── sql_parser_test.go       # Tests for SQL parsing
├── config_schema.go         # Configuration schema information
├── config_schema_test.go    # Tests for config schema
├── example-config.yaml      # Example configuration file
└── CURSOR_INTEGRATION.md   # This documentation
```

## Usage Examples

### 1. Validate Configuration

```json
{
  "name": "validate_config",
  "arguments": {
    "config_path": "/path/to/config.yaml"
  }
}
```

### 2. Parse SQL Statement

```json
{
  "name": "parse_sql",
  "arguments": {
    "sql_statement": "SELECT * FROM users WHERE id = 1;"
  }
}
```

### 3. Get Configuration Schema

```json
{
  "name": "get_config_schema",
  "arguments": {
    "section": "source"
  }
}
```

### 4. Execute Assess Migration (Synchronous - Auto --yes)

```json
{
  "name": "assess_migration",
  "arguments": {
    "config_path": "/path/to/config.yaml",
    "additional_args": "--log-level debug"
  }
}
```

### 5. Execute Assess Migration (Async - Recommended)

```json
{
  "name": "assess_migration_async",
  "arguments": {
    "config_path": "/path/to/config.yaml",
    "additional_args": "--log-level debug"
  }
}
```

### 6. Check Command Status

```json
{
  "name": "get_command_status",
  "arguments": {
    "execution_id": "exec_1234567890"
  }
}
```

### 7. Stop Command

```json
{
  "name": "stop_command",
  "arguments": {
    "execution_id": "exec_1234567890"
  }
}
```

## Interactive Prompt Handling

### How It Works

1. **Detection**: The system detects interactive prompts in command output
2. **Logging**: Prompts are logged with `INTERACTIVE_PROMPT` prefix
3. **User Action**: The AI agent must manually respond using appropriate methods

### For Non-Interactive Commands

Use the `--yes` flag to avoid interactive prompts:

```json
{
  "name": "assess_migration_async",
  "arguments": {
    "config_path": "/path/to/config.yaml",
    "additional_args": "--yes"
  }
}
```

### For Interactive Commands

Commands without `--yes` will wait for user input. The AI agent should:

1. Monitor the command status using `get_command_status`
2. Look for `INTERACTIVE_PROMPT` messages in the progress
3. Provide appropriate responses based on the prompt

## Current Limitations

1. **Automatic --yes Flag**: All commands automatically include `--yes` flag to avoid interactive prompts
2. **Limited Interactive Control**: Manual prompt handling is available but not the default behavior
3. **Database Dependencies**: Commands require valid database connections to complete successfully

## Best Practices

### For AI Agents

1. **Commands auto-include `--yes`** - no need to manually add it
2. **Monitor command status** using `get_command_status` for long-running operations
3. **Check progress logs** for execution details and any errors
4. **Commands can run for hours** - no timeout restrictions
5. **Use async commands** (`assess_migration_async`) for better user experience
6. **Stop long-running commands** using `stop_command` when needed

### For Development

1. **Test with valid configs** - ensure configuration files are properly formatted
2. **Monitor progress logs** - they provide detailed execution information
3. **Handle errors gracefully** - commands may fail due to database connectivity issues
4. **Commands run indefinitely** - no automatic timeout, use `stop_command` to cancel if needed

## Integration with Cursor

### Setup

1. Build the YB Voyager binary with MCP server support
2. Start the MCP server: `./yb-voyager start-mcp-server`
3. Configure Cursor to use the MCP server
4. Test with simple commands first

### Testing

1. Start with `validate_config` to ensure configuration is correct
2. Use `assess_migration_async` with `--yes` for non-interactive testing
3. Monitor command status and progress logs
4. Test interactive scenarios by removing `--yes` flag

## Next Steps

1. Implement remaining command tools (`export_schema`, `export_data`, etc.)
2. Add more comprehensive error handling
3. Improve interactive prompt handling
4. Add support for command cancellation
5. Implement command history and logging

3. **Respond to prompts manually** when you see `INTERACTIVE_PROMPT_DETECTED:`:
   ```json
   {
     "tool": "respond_to_prompt",
     "execution_id": "exec_1234567890",
     "response": "Y"
   }
   ```

## Why Use Async?

- **Real-time streaming**: See command output as it happens
- **Non-blocking**: Commands run in background
- **User-controlled prompts**: You decide how to respond to interactive prompts
- **Better user experience**: No timeouts or blocking

## Tool Descriptions

The async version is clearly marked as "RECOMMENDED" in its description, while the sync version includes a "WARNING" about potential blocking.

## Starting the Server

```bash
./yb-voyager start-mcp-server
```

The server communicates via JSON-RPC over stdio for integration with LLM clients like Cursor. 

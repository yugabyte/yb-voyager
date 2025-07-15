# YB Voyager MCP Server

A Model Context Protocol (MCP) server that enables Large Language Models (LLMs) like Claude to interact with YB Voyager database migration tools through a standardized interface.

## Overview

The YB Voyager MCP server provides LLMs with the ability to:
- Execute YB Voyager migration commands using configuration files
- Manage export directories and migration workspaces
- Query migration status and metadata from MetaDB
- Access migration logs and schema analysis results
- Generate configuration file content from templates

## Features

### Tools (9 available)
1. **`get_export_directory_info`** - Get directory status and metadata
2. **`generate_config_content`** - Generate config file content from templates
3. **`execute_voyager_command`** - Execute YB Voyager commands with smart normalization (recommended)
4. **`execute_voyager_with_config`** - Execute YB Voyager using config files
5. **`execute_voyager_legacy`** - Execute individual YB Voyager commands (legacy)
6. **`query_migration_status`** - Query current migration status from MetaDB
7. **`get_metadb_stats`** - Get migration statistics and progress
8. **`get_assessment_report`** - Get complete migration assessment report as structured JSON
9. **`get_schema_analysis_report`** - Get complete schema analysis report as structured JSON



## Setup Instructions

### Prerequisites
- YB Voyager installed and accessible in PATH
- Go 1.19+ for building the MCP server
- Claude Desktop application

### 1. Build the MCP Server
```bash
# From the yb-voyager directory
go build -o yb-voyager .
```

### 2. Test the MCP Server
```bash
# Test that the MCP server command is available
./yb-voyager mcp-server --help
```

### 3. Configure Claude Desktop

Create or edit the Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
**Linux**: `~/.config/claude/claude_desktop_config.json`

Add the following configuration:

```json
{
  "mcpServers": {
    "yb-voyager": {
      "command": "/full/path/to/yb-voyager/yb-voyager",
      "args": ["mcp-server"],
      "env": {
        "PATH": "/usr/local/bin:/usr/bin:/bin"
      }
    }
  }
}
```

**Important**: Replace `/full/path/to/yb-voyager/yb-voyager` with the actual absolute path to your compiled binary.

### 4. Restart Claude Desktop
Close and reopen Claude Desktop to load the new MCP server configuration.

### 5. Verify Connection
In Claude Desktop, you should see an MCP icon or indicator showing that the YB Voyager server is connected.

## Setup for Cursor IDE

### 1. Build the MCP Server
```bash
# From the yb-voyager directory
go build -o yb-voyager .
```

### 2. Configure Cursor Settings
Open Cursor and go to Settings → Extensions → MCP Servers, or create/edit the MCP configuration file:

**macOS/Linux**: `~/.cursor/mcp_servers.json`
**Windows**: `%APPDATA%\Cursor\mcp_servers.json`

Add the YB Voyager MCP server configuration:

```json
{
  "yb-voyager": {
    "command": "/full/path/to/yb-voyager/yb-voyager",
    "args": ["mcp-server"],
    "env": {
      "PATH": "/usr/local/bin:/usr/bin:/bin"
    }
  }
}
```

**Important**: Replace `/full/path/to/yb-voyager/yb-voyager` with the actual absolute path to your compiled binary.

### 3. Restart Cursor
Close and reopen Cursor to load the new MCP server configuration.

### 4. Verify Connection
In Cursor, you should see the YB Voyager MCP server listed in the MCP panel or available in the AI assistant.

## Usage Examples

### Basic Migration Workflow
1. **Get Export Directory Info**:
   ```
   Can you check the status of my export directory at /path/to/my-migration?
   Can you check the status of migration for config at /path/to/config-file
   ```

2. **Execute Migration**:
   ```
   Execute the export schema command using the config file I created
   Do schema export
   Do migration assessment
   ```

3. **Monitor Progress**:
   ```
   What's the current status of my migration?
   ```

### Advanced Usage
- **Assessment Analysis**: "Show me the complete assessment report for my migration"
- **Schema Analysis**: "Get the detailed schema analysis report for my export directory"
- **Performance**: "What are the migration statistics from my metaDB?"

<!-- 
This is TODO: provide access to config file template so that AI agent can help you build your assessment report also.

## Configuration Templates

The server provides access to 5 built-in configuration templates through the `generate_config_content` tool:
- `live-migration.yaml` - Standard live migration
- `offline-migration.yaml` - Offline migration workflow
- `bulk-data-load.yaml` - Bulk data loading
- `live-migration-with-fall-back.yaml` - Live migration with fallback
- `live-migration-with-fall-forward.yaml` - Live migration with fall-forward
-->

## Architecture

The MCP server follows a config-first approach:
1. **Directory Management** - Check and validate workspaces
2. **Execution** - Run YB Voyager with validated configs
3. **Monitoring** - Fetch progress and status

## Troubleshooting

### Common Issues

1. **"Command not found" error**:
   - Ensure the full absolute path to the binary is used in the config
   - Verify the binary is executable: `chmod +x yb-voyager`

2. **"yb-voyager executable not found in PATH" error**:
   - The MCP server now automatically finds the yb-voyager executable
   - If this fails, ensure yb-voyager is installed and accessible
   - Check with: `which yb-voyager` in your terminal

3. **"Permission denied" error**:
   - Check file permissions on the binary
   - Ensure Claude Desktop has necessary permissions
   - Use the `generate_config_content` tool to get config content without writing files

4. **"Connection failed" error**:
   - Verify the JSON configuration syntax
   - Check the logs in Claude Desktop developer tools
   - Ensure YB Voyager is in the system PATH

5. **"Tool not available" error**:
   - Restart Claude Desktop after configuration changes
   - Verify the MCP server is running: `./yb-voyager mcp-server`

### Debug Mode
Run the MCP server directly to see debug output:
```bash
./yb-voyager mcp-server
```

## Development

### File Structure
- `server.go` - Main MCP server implementation
- `handlers.go` - Tool, resource, and prompt handlers
- `README.md` - This documentation

### Adding New Tools
1. Define the tool in `handlers.go`
2. Implement the handler function
3. Register the tool in `server.go`
4. Update this README
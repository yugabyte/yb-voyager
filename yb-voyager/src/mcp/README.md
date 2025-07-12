# YB Voyager MCP Server

A Model Context Protocol (MCP) server that enables Large Language Models (LLMs) like Claude to interact with YB Voyager database migration tools through a standardized interface.

## Overview

The YB Voyager MCP server provides LLMs with the ability to:
- Execute YB Voyager migration commands using configuration files
- Manage export directories and migration workspaces
- Query migration status and metadata from MetaDB
- Access migration logs and schema analysis results
- Generate and validate configuration files from templates

## Features

### Tools (8 available)
1. **`create_export_directory`** - Create and validate export directories
2. **`get_export_directory_info`** - Get directory status and metadata
3. **`create_config_file`** - Generate config files from templates
4. **`validate_config_file`** - Validate YAML configuration files
5. **`execute_voyager_with_config`** - Execute YB Voyager using config files (recommended)
6. **`execute_voyager_command`** - Execute individual YB Voyager commands
7. **`query_migration_status`** - Query current migration status from MetaDB
8. **`get_metadb_stats`** - Get migration statistics and progress

### Resources (4 available)
1. **`voyager://migration-status`** - Current migration status
2. **`voyager://logs/{export_dir}/recent`** - Recent log files
3. **`voyager://schema-analysis/{export_dir}`** - Schema analysis results
4. **`voyager://config-templates`** - Available configuration templates

### Prompts (3 available)
1. **`troubleshoot_migration`** - Interactive troubleshooting assistance
2. **`optimize_performance`** - Performance optimization recommendations
3. **`generate_config`** - Guided configuration file creation

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

## Usage Examples

### Basic Migration Workflow
1. **Create Export Directory**:
   ```
   Can you create an export directory at /path/to/my-migration?
   ```

2. **Generate Configuration**:
   ```
   Help me create a live migration config file for migrating from PostgreSQL to YugabyteDB
   ```

3. **Execute Migration**:
   ```
   Execute the export schema command using the config file we just created
   ```

4. **Monitor Progress**:
   ```
   What's the current status of my migration?
   ```

### Advanced Usage
- **Troubleshooting**: "I'm having issues with my migration, can you help troubleshoot?"
- **Performance**: "How can I optimize my migration performance?"
- **Analysis**: "Show me the schema analysis results for my export directory"

## Configuration Templates

The server provides access to 5 built-in configuration templates:
- `live-migration.yaml` - Standard live migration
- `offline-migration.yaml` - Offline migration workflow
- `bulk-data-load.yaml` - Bulk data loading
- `live-migration-with-fall-back.yaml` - Live migration with fallback
- `live-migration-with-fall-forward.yaml` - Live migration with fall-forward

## Architecture

The MCP server follows a config-first approach:
1. **Directory Management** - Create and validate workspaces
2. **Template-Based Config** - Generate configs from proven templates
3. **Validation** - Ensure configuration correctness
4. **Execution** - Run YB Voyager with validated configs
5. **Monitoring** - Track progress and status

## Troubleshooting

### Common Issues

1. **"Command not found" error**:
   - Ensure the full absolute path to the binary is used in the config
   - Verify the binary is executable: `chmod +x yb-voyager`

2. **"Permission denied" error**:
   - Check file permissions on the binary
   - Ensure Claude Desktop has necessary permissions

3. **"Connection failed" error**:
   - Verify the JSON configuration syntax
   - Check the logs in Claude Desktop developer tools
   - Ensure YB Voyager is in the system PATH

4. **"Tool not available" error**:
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

## Support

For issues specific to the MCP server integration, check:
1. Claude Desktop logs
2. YB Voyager command-line functionality
3. File permissions and paths
4. Configuration file syntax

For YB Voyager-specific issues, refer to the main YB Voyager documentation. 
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/mcp"
)

var mcpServerCmd = &cobra.Command{
	Use:   "mcp-server",
	Short: "Start MCP server for LLM integration",
	Long: `Start the Model Context Protocol (MCP) server that allows LLMs to:
- Execute yb-voyager commands
- Query migration status and metadata
- Access structured migration data

The server communicates via JSON-RPC over stdio for integration with LLM clients.`,
	Run: func(cmd *cobra.Command, args []string) {
		server := mcp.NewServer()
		if err := server.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running MCP server: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(mcpServerCmd)
}

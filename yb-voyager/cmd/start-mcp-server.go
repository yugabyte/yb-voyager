package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	mcpservernew "github.com/yugabyte/yb-voyager/yb-voyager/src/mcp-server-new"
)

var startMcpServerCmd = &cobra.Command{
	Use:   "start-mcp-server",
	Short: "Start new MCP server for LLM integration",
	Long: `Start the new Model Context Protocol (MCP) server that allows LLMs to:
- Execute migration commands using config files
- Validate configuration files
- Access migration status and metadata

This is a new implementation with config-first approach and no fallbacks.
The server communicates via JSON-RPC over stdio for integration with LLM clients.`,
	Run: func(cmd *cobra.Command, args []string) {
		server := mcpservernew.NewServer()
		if err := server.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running MCP server: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(startMcpServerCmd)
}

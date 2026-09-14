package cmd

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	rhoconfig "github.com/GrayCodeAI/rho/internal/config"
	"github.com/GrayCodeAI/rho/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpConfigWrite bool

func init() {
	mcpConfigCmd.Flags().BoolVar(&mcpConfigWrite, "write", false,
		"also print the well-known client config paths to paste the block into")
	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpConfigCmd)
}

// mcpServeCmd runs rho itself as an MCP server over stdio, exposing rho's
// capabilities (chat, search, memory, review, scan, compress) to MCP clients
// such as Claude Desktop, Cursor, and Windsurf.
var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run rho as an MCP server over stdio",
	Long: "Run rho as a Model Context Protocol server over stdio (JSON-RPC 2.0), " +
		"exposing rho's tools to MCP clients like Claude Desktop, Cursor, and Windsurf.\n\n" +
		"Use `rho mcp config` to print the JSON block that registers this command in a client.",
	RunE: runMCPServe,
}

func runMCPServe(cmd *cobra.Command, _ []string) error {
	settings := rhoconfig.LoadSettings()

	serverVersion := version
	if serverVersion == "" {
		serverVersion = "dev"
	}
	server := mcp.NewMCPServer(mcp.ServerInfo{Name: "rho", Version: serverVersion})

	// Wire rho's tool registry in as the executor so delegating tools run for
	// real; a registry build failure degrades to not-configured rather than
	// aborting (the server still answers initialize/tools/list).
	registry, err := defaultRegistry(settings)
	if err == nil {
		mcp.RegisterDefaultTools(server, registry.Execute)
	} else {
		mcp.RegisterDefaultTools(server, nil)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return server.ServeStdio(ctx)
}

// mcpConfigCmd emits the JSON block that registers rho as an MCP server in a
// client's config file, so users don't hand-edit JSON.
var mcpConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the MCP-server config block to register rho in a client",
	Long: "Print the JSON block that registers rho as an MCP server (pointing at " +
		"`rho mcp serve`) for clients like Claude Desktop, Cursor, and Windsurf.\n\n" +
		"Pipe it to the client's config file, e.g.:\n" +
		"  rho mcp config >> ~/Library/Application Support/Claude/claude_desktop_config.json",
	RunE: runMCPConfig,
}

func runMCPConfig(cmd *cobra.Command, _ []string) error {
	exe := rhoExecutablePath()

	block := map[string]any{
		"mcpServers": map[string]any{
			"rho": map[string]any{
				"command": exe,
				"args":    []string{"mcp", "serve"},
			},
		},
	}
	out, err := json.MarshalIndent(block, "", "  ")
	if err != nil {
		return err
	}

	if mcpConfigWrite {
		cmd.Println("# Add the \"rho\" entry below into the \"mcpServers\" object of your client config:")
		cmd.Println("#   Claude Desktop (macOS): ~/Library/Application Support/Claude/claude_desktop_config.json")
		cmd.Println("#   Cursor:                 ~/.cursor/mcp.json")
		cmd.Println("#   Windsurf:               ~/.codeium/windsurf/mcp_config.json")
		cmd.Println()
	}
	cmd.Println(string(out))
	return nil
}

// rhoExecutablePath returns the absolute path to the running rho binary, or
// the bare name "rho" if it cannot be resolved (e.g. during `go run`), so the
// emitted config is still copy-pasteable.
func rhoExecutablePath() string {
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return "rho"
}

// The rdl-mcp command runs the report tools over MCP stdio.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/h0rn3t/rdl-mcp/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	level := slog.LevelInfo
	switch strings.ToUpper(os.Getenv("RDL_MCP_LOG_LEVEL")) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}
	output := io.Writer(os.Stderr)
	if path := os.Getenv("RDL_MCP_LOG_FILE"); path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open RDL_MCP_LOG_FILE: %v\n", err)
		} else {
			defer func() {
				if err := file.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "close RDL_MCP_LOG_FILE: %v\n", err)
				}
			}()
			output = io.MultiWriter(os.Stderr, file)
		}
	}
	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: level}))
	logger.Info("starting RDL MCP server")
	server, err := mcpserver.New()
	if err != nil {
		return err
	}
	return server.Run(context.Background(), &mcp.StdioTransport{})
}

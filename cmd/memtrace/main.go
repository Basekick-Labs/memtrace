package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

// rootCmd defaults to running the server when invoked with no subcommand,
// preserving compatibility with `docker run memtrace`.
var rootCmd = &cobra.Command{
	Use:   "memtrace",
	Short: "Memtrace — memory layer for AI agents, backed by Arc",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServe(cmd, args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       version,
}

func main() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(keygenCmd)
	rootCmd.AddCommand(orgCmd)
	rootCmd.AddCommand(keyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

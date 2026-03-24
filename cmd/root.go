package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "supa-brain",
	Short: "Semantic memory MCP server for Claude Code",
	RunE: func(_ *cobra.Command, args []string) error {
		return runStdio(args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

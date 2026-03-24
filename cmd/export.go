package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	exportAdapter "supa-brain/adapters/export"
	supabaseAdapter "supa-brain/adapters/supabase"
	"supa-brain/core"
	"supa-brain/internal/config"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export memories to JSON or CSV",
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().String("format", "json", "Output format: json|csv")
	exportCmd.Flags().String("out", "", "Output file path (default: stdout)")
	exportCmd.Flags().String("project", "", "Filter by project (default: all)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString("format")
	outPath, _ := cmd.Flags().GetString("out")
	projectFlag, _ := cmd.Flags().GetString("project")

	cfgPath := filepath.Join(os.Getenv("USERPROFILE"), ".memory-server", "config.env")
	cfg := config.LoadConfig(cfgPath)

	ctx := context.Background()
	store, err := supabaseAdapter.New(ctx, cfg.SupabaseURL, cfg.SupabaseKey, cfg.DBMaxConns, cfg.DBConnectTimeout)
	if err != nil {
		return fmt.Errorf("cannot connect to Supabase: %w", err)
	}

	filter := core.ExportFilter{}
	if projectFlag != "" {
		filter.Project = &projectFlag
	}

	memories, err := store.Export(ctx, filter)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	var exporter core.Exporter
	switch format {
	case "csv":
		exporter = &exportAdapter.CSVExporter{}
	default:
		exporter = &exportAdapter.JSONExporter{}
	}

	out := os.Stdout
	if outPath != "" {
		out, err = os.Create(outPath)
		if err != nil {
			return fmt.Errorf("cannot create output file: %w", err)
		}
		defer out.Close()
	}

	if err := exporter.Export(ctx, out, memories); err != nil {
		return fmt.Errorf("export write failed: %w", err)
	}

	if outPath != "" {
		fmt.Fprintf(os.Stderr, "Exported %d memories to %s\n", len(memories), outPath)
	}
	return nil
}

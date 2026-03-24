package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	ollamaAdapter "supa-brain/adapters/ollama"
	"supa-brain/adapters/migration"
	supabaseAdapter "supa-brain/adapters/supabase"
	"supa-brain/internal/config"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Import memories from an Engram JSON export",
	RunE:  runMigrate,
}

func init() {
	migrateCmd.Flags().String("input", "", "Path to engram-export.json (required)")
	migrateCmd.Flags().Bool("dry-run", false, "Preview without writing")
	migrateCmd.MarkFlagRequired("input")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, _ []string) error {
	input, _ := cmd.Flags().GetString("input")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("cannot read input file: %w", err)
	}

	memories, err := migration.ParseEngram(data)
	if err != nil {
		return err
	}

	if dryRun {
		estimatedSecs := len(memories) * 220 / 1000
		fmt.Printf("Preview migracion:\n")
		fmt.Printf("  Total observaciones: %d\n", len(memories))
		fmt.Printf("  Embeddings a generar: %d\n", len(memories))
		fmt.Printf("  Estimado Ollama: ~%d segundos (%d x ~220ms)\n", estimatedSecs, len(memories))
		fmt.Println("\nCorre sin --dry-run para migrar.")
		return nil
	}

	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = os.Getenv("HOME")
	}
	cfgPath := filepath.Join(userProfile, ".memory-server", "config.env")
	cfg := config.LoadConfig(cfgPath)

	ctx := context.Background()
	ollamaClient := ollamaAdapter.New(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaTimeout)
	store, err := supabaseAdapter.New(ctx, cfg.SupabaseURL, cfg.SupabaseKey, cfg.DBMaxConns, cfg.DBConnectTimeout)
	if err != nil {
		return fmt.Errorf("cannot connect to Supabase: %w", err)
	}

	var failed, skipped, saved int
	total := len(memories)

	for i, m := range memories {
		fmt.Printf("\r  [%d/%d] %s", i+1, total, truncate(m.Title, 50))

		vec, err := ollamaClient.Embed(ctx, m.Title+"\n"+m.Content)
		if err != nil {
			fmt.Printf("\n  WARN: Failed to embed '%s': %v — skipping\n", m.Title, err)
			failed++
			continue
		}
		m.Embedding = vec

		if m.TopicKey != nil {
			_, err = store.Upsert(ctx, m)
		} else {
			_, err = store.Insert(ctx, m)
		}

		if err != nil {
			fmt.Printf("\n  WARN: Failed to save '%s': %v — skipping\n", m.Title, err)
			skipped++
			continue
		}
		saved++

		time.Sleep(10 * time.Millisecond)
	}

	fmt.Printf("\n\nMigracion completada\n")
	fmt.Printf("  Guardadas: %d\n", saved)
	fmt.Printf("  Omitidas:  %d\n", skipped)
	fmt.Printf("  Fallidas:  %d\n", failed)
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"supa-brain/internal/config"
)

var dbMigrateCmd = &cobra.Command{
	Use:   "db:migrate",
	Short: "Print SQL migration and open Supabase SQL editor",
	Long: `Print the SQL migration script and attempt to open the Supabase SQL editor.
You must manually paste and run the SQL in your Supabase dashboard.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := filepath.Join(os.Getenv("USERPROFILE"), ".memory-server", "config.env")
		cfg := config.LoadConfig(cfgPath)

		if cfg.SupabaseURL == "" {
			return fmt.Errorf("SUPABASE_URL not set in %s", cfgPath)
		}

		// Read SQL migration file
		sqlBytes, err := os.ReadFile("migrations/001_init.sql")
		if err != nil {
			return fmt.Errorf("failed to read migration file: %w", err)
		}
		sqlContent := string(sqlBytes)

		ref := extractRef(cfg.SupabaseURL)
		sqlEditorURL := fmt.Sprintf("https://supabase.com/dashboard/project/%s/sql/new", ref)

		fmt.Println("====== SUPABASE MIGRATION ======")
		fmt.Printf("Supabase Project: %s\n", cfg.SupabaseURL)
		fmt.Printf("SQL Editor: %s\n\n", sqlEditorURL)
		fmt.Println("Copy and run this SQL in the Supabase SQL editor:")
		fmt.Println("----------------------------------------")
		fmt.Print(sqlContent)
		fmt.Println("----------------------------------------")
		fmt.Println("Opening browser...")
		openBrowser(sqlEditorURL)
		fmt.Println("(If browser didn't open, visit the SQL Editor URL above)")
		return nil
	},
}

func extractRef(supabaseURL string) string {
	s := supabaseURL
	// Remove protocol
	for _, p := range []string{"https://", "http://"} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimPrefix(s, p)
			break
		}
	}
	// Extract first segment before dot (e.g. "lxjsjebytrqbrcjhmgpi.supabase.co" -> "lxjsjebytrqbrcjhmgpi")
	parts := strings.Split(s, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return s
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Run(); err != nil {
		// Silently ignore browser open failures
	}
}

func init() {
	rootCmd.AddCommand(dbMigrateCmd)
}

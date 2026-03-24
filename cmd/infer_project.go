package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// inferProject returns the project name for the current working directory.
// global overrides CWD inference if non-empty (from config MEMORY_PROJECT).
func inferProject(global string) string {
	// 1. Environment variable
	if v := os.Getenv("MEMORY_PROJECT"); v != "" {
		return v
	}
	// 2. Global config override
	if global != "" {
		return global
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "personal"
	}

	// 3. CLAUDE.md — look for "# ProjectName" heading
	if name := findInCLAUDEMd(cwd); name != "" {
		return name
	}

	// 4. package.json "name" field
	if name := readPackageJSON(cwd); name != "" {
		return name
	}

	// 5. go.mod module name (last segment)
	if name := readGoMod(cwd); name != "" {
		return name
	}

	// 6. Directory name fallback
	return filepath.Base(cwd)
}

func findInCLAUDEMd(dir string) string {
	for {
		data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "# ") {
					name := strings.TrimPrefix(line, "# ")
					return strings.TrimSpace(name)
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func readPackageJSON(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Name
}

func readGoMod(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			module := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			parts := strings.Split(module, "/")
			return parts[len(parts)-1]
		}
	}
	return ""
}

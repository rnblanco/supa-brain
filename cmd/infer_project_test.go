package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferProject_EnvVar(t *testing.T) {
	os.Setenv("MEMORY_PROJECT", "my-project")
	defer os.Unsetenv("MEMORY_PROJECT")

	result := inferProject("")
	assert.Equal(t, "my-project", result)
}

func TestInferProject_GlobalOverride(t *testing.T) {
	os.Unsetenv("MEMORY_PROJECT")
	result := inferProject("global-override")
	assert.Equal(t, "global-override", result)
}

func TestInferProject_PackageJSON(t *testing.T) {
	os.Unsetenv("MEMORY_PROJECT")

	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"my-app"}`), 0644)
	os.Chdir(dir)
	// Chdir back before TempDir cleanup runs (Windows keeps handle open)
	t.Cleanup(func() { os.Chdir(orig) })

	result := inferProject("")
	assert.Equal(t, "my-app", result)
}

func TestInferProject_DirFallback(t *testing.T) {
	os.Unsetenv("MEMORY_PROJECT")

	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	// Chdir back before TempDir cleanup runs (Windows keeps handle open)
	t.Cleanup(func() { os.Chdir(orig) })

	result := inferProject("")
	assert.Equal(t, filepath.Base(dir), result)
}

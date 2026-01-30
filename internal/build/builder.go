package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Builder handles Go cross-compilation
type Builder struct {
	MainPkg string
	Output  string
	GOOS    string
	GOARCH  string
	LDFlags string
	Env     map[string]string
}

// NewBuilder creates a new builder for the target architecture
func NewBuilder(mainPkg, output, goarch string) *Builder {
	return &Builder{
		MainPkg: mainPkg,
		Output:  output,
		GOOS:    "linux",
		GOARCH:  goarch,
		LDFlags: "-s -w",
	}
}

// Build compiles the Go application and returns the path to the binary
func (b *Builder) Build(ctx context.Context) (string, error) {
	// Create temp directory for build output
	tmpDir, err := os.MkdirTemp("", "gotzer-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	outputPath := filepath.Join(tmpDir, b.Output)

	// Build the command
	args := []string{"build"}
	if b.LDFlags != "" {
		args = append(args, fmt.Sprintf("-ldflags=%s", b.LDFlags))
	}
	args = append(args, "-o", outputPath, b.MainPkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment
	env := os.Environ()
	env = append(env, fmt.Sprintf("GOOS=%s", b.GOOS))
	env = append(env, fmt.Sprintf("GOARCH=%s", b.GOARCH))
	env = append(env, "CGO_ENABLED=0")

	for k, v := range b.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	fmt.Printf("Building for %s/%s...\n", b.GOOS, b.GOARCH)
	fmt.Printf("  → go %s\n", strings.Join(args, " "))

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}

	// Get file info
	info, err := os.Stat(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat binary: %w", err)
	}

	fmt.Printf("  → Built %s (%.2f MB)\n", b.Output, float64(info.Size())/(1024*1024))

	return outputPath, nil
}

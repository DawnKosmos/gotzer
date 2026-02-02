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
	Type    string // "go" or "static"
	MainPkg string
	Output  string
	Command string // for static builds
	Dir     string // for static builds
	GOOS    string
	GOARCH  string
	LDFlags string
	Env     map[string]string
}

// NewBuilder creates a new builder for the target architecture
func NewBuilder(buildType, mainPkg, output, goarch string) *Builder {
	return &Builder{
		Type:    buildType,
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

	if b.Type == "static" {
		fmt.Printf("Building static project...\n")
		fmt.Printf("  → %s\n", b.Command)

		cmd := exec.CommandContext(ctx, "sh", "-c", b.Command)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		for k, v := range b.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("static build failed: %w", err)
		}

		// Verify output directory
		if _, err := os.Stat(b.Dir); err != nil {
			return "", fmt.Errorf("build output directory %s not found: %w", b.Dir, err)
		}

		return b.Dir, nil
	}

	// Go build logic...
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

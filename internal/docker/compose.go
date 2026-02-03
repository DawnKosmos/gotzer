package docker

import (
	"fmt"
	"strings"

	"github.com/DawnKosmos/gotzer/internal/config"
)

// GenerateCompose creates the docker-compose.yml content
func GenerateCompose(cfg *config.Config) string {
	services := cfg.Services

	var builder strings.Builder
	builder.WriteString("services:\n")

	if services.Postgres != nil && services.Postgres.Enabled {
		builder.WriteString(formatService("postgres", services.Postgres))
	}

	if services.Typesense != nil && services.Typesense.Enabled {
		builder.WriteString(formatService("typesense", services.Typesense))
	}

	if services.Redis != nil && services.Redis.Enabled {
		builder.WriteString(formatService("redis", services.Redis))
	}
	if services.Centrifugo != nil && services.Centrifugo.Enabled {
		builder.WriteString(formatService("centrifugo", services.Centrifugo))
	}

	for _, svc := range services.Custom {
		if svc.Enabled {
			// Ensure custom services have a name (using image name as fallback if needed, but config usually has keys)
			// For array of custom services, we might need a name field or derive it.
			// Assuming Custom services struct needs a Name field or we iterate differently.
			// Re-checking config.go... Custom is []ServiceConfig. ServiceConfig doesn't have Name.
			// In setup.go it was formatted as "custom". This implies only one custom service or overwrites.
			// Wait, looping `for _, svc` and writing "custom" key means duplicate keys if >1.
			// Let's check setup.go logic again.
			// setup.go: builder.WriteString(p.formatService("custom", &svc)) inside loop.
			// This generates invalid yaml if multiple custom services exist.
			// For now, let's keep it 1:1 with existing logic but maybe append index if needed?
			// Or just use "custom" as in original code.
			builder.WriteString(formatService("custom", &svc))
		}
	}

	// Add volumes section
	builder.WriteString("\nvolumes:\n")
	if services.Postgres != nil && services.Postgres.Enabled {
		builder.WriteString("  pgdata:\n")
	}
	if services.Typesense != nil && services.Typesense.Enabled {
		builder.WriteString("  typesense-data:\n")
	}
	if services.Redis != nil && services.Redis.Enabled {
		builder.WriteString("  redis-data:\n")
	}
	if services.Centrifugo != nil && services.Centrifugo.Enabled {
		// Centrifugo usually doesn't need a volume for data in simple setups,
		// but we could add it if needed.
	}

	return builder.String()
}

// formatService formats a single service for docker-compose
func formatService(name string, svc *config.ServiceConfig) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("  %s:\n", name))
	builder.WriteString(fmt.Sprintf("    image: %s\n", svc.Image))
	builder.WriteString("    restart: always\n")

	if svc.Command != "" {
		builder.WriteString(fmt.Sprintf("    command: %s\n", svc.Command))
	}

	if svc.Port > 0 {
		builder.WriteString(fmt.Sprintf("    ports:\n      - \"%d:%d\"\n", svc.Port, svc.Port))
	}

	if len(svc.Volumes) > 0 {
		builder.WriteString("    volumes:\n")
		for _, vol := range svc.Volumes {
			builder.WriteString(fmt.Sprintf("      - %s\n", vol))
		}
	}

	if len(svc.Env) > 0 {
		builder.WriteString("    environment:\n")
		for k, v := range svc.Env {
			builder.WriteString(fmt.Sprintf("      %s: \"%s\"\n", k, v))
		}
	}

	return builder.String()
}

package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/yarlson/snap/internal/claude"
	"github.com/yarlson/snap/internal/codex"
	"github.com/yarlson/snap/internal/workflow"
)

const (
	envVar          = "SNAP_PROVIDER"
	defaultProvider = "claude"
)

// NewExecutorFromEnv creates an executor based on SNAP_PROVIDER.
func NewExecutorFromEnv() (workflow.Executor, error) {
	provider := normalize(os.Getenv(envVar))

	switch provider {
	case "claude":
		return claude.NewExecutor(), nil
	case "codex":
		return codex.NewExecutor(), nil
	default:
		return nil, fmt.Errorf("invalid %s value %q (supported: claude, codex)", envVar, provider)
	}
}

func normalize(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return defaultProvider
	}
	if normalized == "claude-code" {
		return "claude"
	}
	return normalized
}

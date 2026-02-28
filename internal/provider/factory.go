package provider

import (
	"fmt"
	"os"
	"os/exec"
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

type providerInfo struct {
	Binary      string
	InstallURL  string
	DisplayName string
	Alternative string
}

var providers = map[string]providerInfo{
	"claude": {
		Binary:      "claude",
		InstallURL:  "https://docs.anthropic.com/en/docs/claude-cli",
		DisplayName: "Claude CLI",
		Alternative: "codex",
	},
	"codex": {
		Binary:      "codex",
		InstallURL:  "https://github.com/openai/codex",
		DisplayName: "Codex CLI",
		Alternative: "claude",
	},
}

// ValidateCLI checks that the provider's CLI binary exists in PATH.
func ValidateCLI(providerName string) error {
	info, ok := providers[providerName]
	if !ok {
		return fmt.Errorf("unknown provider %q (supported: claude, codex)", providerName)
	}

	if _, err := exec.LookPath(info.Binary); err != nil {
		return fmt.Errorf( //nolint:staticcheck // ST1005: capitalized for user-facing DESIGN.md error format
			"Error: %s not found in PATH\n\nsnap requires the %s to run. Install it:\n  %s\n\nOr use a different provider:\n  SNAP_PROVIDER=%s snap",
			info.Binary, info.DisplayName, info.InstallURL, info.Alternative,
		)
	}

	return nil
}

// ValidateGH checks that the gh CLI binary exists in PATH.
func ValidateGH() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf( //nolint:staticcheck // ST1005: capitalized for user-facing DESIGN.md error format
			"Error: gh not found in PATH\n\nGitHub features require the gh CLI. Install it:\n  https://cli.github.com/\n\nOr use a non-GitHub remote to skip GitHub features",
		)
	}
	return nil
}

// ResolveProviderName returns the normalized provider name from SNAP_PROVIDER.
func ResolveProviderName() string {
	return normalize(os.Getenv(envVar))
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

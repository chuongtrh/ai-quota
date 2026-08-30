package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const appDirectoryName = "AIQuota"

type Paths struct {
	DataDir string
	HomeDir string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home directory: %w", err)
	}
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve configuration directory: %w", err)
	}
	return Paths{
		DataDir: filepath.Join(configRoot, appDirectoryName),
		HomeDir: home,
	}, nil
}

func (p Paths) Ensure() error {
	return os.MkdirAll(p.DataDir, 0o700)
}

func (p Paths) CodexCache() string {
	return filepath.Join(p.DataDir, "codex-quota.json")
}

func (p Paths) ClaudeCache() string {
	return filepath.Join(p.DataDir, "claude-quota.json")
}

func (p Paths) AlertState() string {
	return filepath.Join(p.DataDir, "alert-state.json")
}

func (p Paths) ClaudeStatusLineBackup() string {
	return filepath.Join(p.DataDir, "claude-statusline-backup.json")
}

func (p Paths) ClaudeHelper() string {
	return filepath.Join(p.DataDir, "bin", "aiquota-bridge")
}

func (p Paths) ClaudeSettings() string {
	return filepath.Join(p.HomeDir, ".claude", "settings.json")
}

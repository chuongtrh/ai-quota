package codex

import (
	"errors"
	"os"

	"github.com/chuongtrh/ai-quota/internal/config"
)

type Installer struct {
	Paths config.Paths
}

func (i Installer) IsConnected() (bool, error) {
	_, err := os.Stat(i.Paths.CodexDisabled())
	if err == nil {
		return false, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	return false, err
}

func (i Installer) Connect() error {
	if err := i.Paths.Ensure(); err != nil {
		return err
	}
	err := os.Remove(i.Paths.CodexDisabled())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (i Installer) Disconnect() error {
	if err := i.Paths.Ensure(); err != nil {
		return err
	}
	f, err := os.OpenFile(i.Paths.CodexDisabled(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

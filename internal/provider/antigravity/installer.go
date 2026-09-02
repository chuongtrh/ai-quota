package antigravity

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

var ErrSettingsChanged = errors.New("the Google Antigravity CLI status line was changed by another application")

type StatusLineBackup struct {
	HadValue bool            `json:"had_value"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type Installer struct {
	Paths      config.Paths
	Executable string
}

func (i Installer) IsConnected() (bool, error) {
	settings, _, err := readSettings(i.Paths.AntigravitySettings())
	if err != nil {
		return false, err
	}
	return isOurStatusLine(settings["statusLine"]), nil
}

func (i Installer) Connect() error {
	if i.Executable == "" {
		return errors.New("could not locate the AI quota executable")
	}
	if err := i.Paths.Ensure(); err != nil {
		return err
	}
	if err := copyExecutable(i.Executable, i.Paths.AntigravityHelper()); err != nil {
		return fmt.Errorf("install Google Antigravity CLI connector: %w", err)
	}

	settings, original, err := readSettings(i.Paths.AntigravitySettings())
	if err != nil {
		return err
	}
	if isOurStatusLine(settings["statusLine"]) {
		return nil
	}

	backup := StatusLineBackup{}
	stackWithDefault := true
	if current, exists := settings["statusLine"]; exists {
		backup.HadValue = true
		backup.Value = append(json.RawMessage(nil), current...)
		var previous struct {
			StackWithDefault bool `json:"stack_with_default"`
		}
		if json.Unmarshal(current, &previous) == nil {
			stackWithDefault = previous.StackWithDefault
		}
	}
	if err := storage.WriteJSON(i.Paths.AntigravityStatusLineBackup(), backup, 0o600); err != nil {
		return fmt.Errorf("back up Google Antigravity CLI status line: %w", err)
	}
	if len(original) > 0 {
		backupName := fmt.Sprintf("settings.aiquota-backup-%s.json", time.Now().Format("20060102-150405"))
		backupPath := filepath.Join(filepath.Dir(i.Paths.AntigravitySettings()), backupName)
		if err := storage.WriteFileAtomic(backupPath, original, 0o600); err != nil {
			return fmt.Errorf("back up Google Antigravity CLI settings.json: %w", err)
		}
	}

	settings["statusLine"] = mustJSON(map[string]any{
		"type":               "command",
		"command":            shellQuote(i.Paths.AntigravityHelper()) + " " + BridgeFlag,
		"enabled":            true,
		"stack_with_default": stackWithDefault,
	})
	if err := storage.WriteJSON(i.Paths.AntigravitySettings(), settings, 0o600); err != nil {
		return fmt.Errorf("connect Google Antigravity CLI status line: %w", err)
	}
	return nil
}

func (i Installer) Disconnect() error {
	settings, _, err := readSettings(i.Paths.AntigravitySettings())
	if err != nil {
		return err
	}
	if !isOurStatusLine(settings["statusLine"]) {
		return ErrSettingsChanged
	}

	var backup StatusLineBackup
	if err := storage.ReadJSON(i.Paths.AntigravityStatusLineBackup(), &backup); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read Google Antigravity CLI status line backup: %w", err)
		}
		backup = StatusLineBackup{}
	}
	if backup.HadValue {
		settings["statusLine"] = append(json.RawMessage(nil), backup.Value...)
	} else {
		delete(settings, "statusLine")
	}
	if err := storage.WriteJSON(i.Paths.AntigravitySettings(), settings, 0o600); err != nil {
		return fmt.Errorf("restore Google Antigravity CLI status line: %w", err)
	}
	return nil
}

func readSettings(path string) (map[string]json.RawMessage, []byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]json.RawMessage), nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	settings := make(map[string]json.RawMessage)
	if len(data) == 0 {
		return settings, data, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, nil, fmt.Errorf("Google Antigravity CLI settings.json is invalid: %w", err)
	}
	return settings, data, nil
}

func isOurStatusLine(value json.RawMessage) bool {
	if len(value) == 0 {
		return false
	}
	var statusLine struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(value, &statusLine); err != nil {
		return false
	}
	return strings.Contains(statusLine.Command, BridgeFlag)
}

func copyExecutable(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".aiquota-antigravity-bridge-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o700); err != nil {
		return err
	}
	if _, err := io.Copy(temporary, input); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	committed = true
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func mustJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func ReadJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("read JSON %s: %w", filepath.Base(path), err)
	}
	return nil
}

func WriteJSON(path string, value any, permission os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	return WriteFileAtomic(path, data, permission)
}

func WriteFileAtomic(path string, data []byte, permission os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create directory %s: %w", directory, err)
	}

	temporary, err := os.CreateTemp(directory, ".aiquota-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(permission); err != nil {
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace file %s: %w", path, err)
	}
	committed = true
	return nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

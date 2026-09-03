package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/chuongtrh/ai-quota/internal/config"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/storage"
)

const BridgeFlag = "--antigravity-statusline"

var ErrNoQuotaData = errors.New("Google Antigravity CLI has not sent quota data yet; send at least one prompt first")

type statusLineInput struct {
	Quota map[string]quotaEntry `json:"quota"`
}

type quotaEntry struct {
	RemainingFraction *float64 `json:"remaining_fraction"`
	ResetTime         string   `json:"reset_time"`
	ResetInSeconds    int64    `json:"reset_in_seconds"`
}

func ParseStatusLine(data []byte, now time.Time) (model.ProviderStatus, error) {
	status := model.ProviderStatus{Provider: model.ProviderAntigravity, UpdatedAt: now}
	var input statusLineInput
	if err := json.Unmarshal(data, &input); err != nil {
		return status, fmt.Errorf("decode Google Antigravity CLI status line: %w", err)
	}
	for bucket, quota := range input.Quota {
		if bucket == "" || quota.RemainingFraction == nil || math.IsNaN(*quota.RemainingFraction) || math.IsInf(*quota.RemainingFraction, 0) {
			continue
		}
		reset, ok := quotaResetTime(quota, now)
		if !ok {
			continue
		}
		status.Windows = append(status.Windows, model.Window{
			Kind:        model.WindowKind(bucket),
			Label:       bucketLabel(bucket),
			UsedPercent: 100 * (1 - *quota.RemainingFraction),
			ResetsAt:    reset,
		})
	}
	status.Normalize()
	if len(status.Windows) == 0 {
		return status, ErrNoQuotaData
	}
	return status, nil
}

func quotaResetTime(quota quotaEntry, now time.Time) (time.Time, bool) {
	if quota.ResetTime != "" {
		if reset, err := time.Parse(time.RFC3339Nano, quota.ResetTime); err == nil {
			return reset, true
		}
	}
	if quota.ResetInSeconds > 0 {
		return now.Add(time.Duration(quota.ResetInSeconds) * time.Second), true
	}
	return time.Time{}, false
}

func bucketLabel(bucket string) string {
	words := strings.FieldsFunc(bucket, func(r rune) bool { return r == '-' || r == '_' })
	for index, word := range words {
		if word != "" {
			words[index] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func RunBridge(paths config.Paths, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(input, 4*1024*1024))
	if err != nil {
		return fmt.Errorf("read Google Antigravity CLI status line: %w", err)
	}
	status, parseErr := ParseStatusLine(data, time.Now())
	if parseErr == nil {
		_ = storage.WriteJSON(paths.AntigravityCache(), status, 0o600)
	}
	previousOutput := runPreviousStatusLine(paths, data)
	if len(bytes.TrimSpace(previousOutput)) > 0 {
		_, _ = output.Write(previousOutput)
		if previousOutput[len(previousOutput)-1] != '\n' {
			_, _ = io.WriteString(output, "\n")
		}
	} else {
		_, _ = io.WriteString(output, formatStatusLine(status)+"\n")
	}
	return nil
}

func runPreviousStatusLine(paths config.Paths, input []byte) []byte {
	var backup StatusLineBackup
	if err := storage.ReadJSON(paths.AntigravityStatusLineBackup(), &backup); err != nil || !backup.HadValue {
		return nil
	}
	var previous struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(backup.Value, &previous); err != nil || previous.Command == "" {
		return nil
	}
	if strings.Contains(previous.Command, BridgeFlag) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", previous.Command)
	cmd.Stdin = bytes.NewReader(input)
	var output bytes.Buffer
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return nil
	}
	return output.Bytes()
}

func formatStatusLine(status model.ProviderStatus) string {
	parts := []string{"[Antigravity]"}
	for _, window := range status.Windows {
		parts = append(parts, fmt.Sprintf("%s: %d%% left", window.DisplayName(), window.RoundedRemainingPercent()))
	}
	if len(status.Windows) == 0 {
		parts = append(parts, "waiting for quota data")
	}
	return strings.Join(parts, " · ")
}

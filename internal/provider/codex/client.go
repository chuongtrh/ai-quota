package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chuongtrh/ai-quota/internal/model"
)

var ErrNotInstalled = errors.New("codex command not found")

type Client struct {
	Command string
	Version string

	commandMu       sync.Mutex
	resolvedCommand string
	lookupCommand   func(string) (string, error)
}

func New(version string) *Client {
	return &Client{Command: "codex", Version: version}
}

func (c *Client) ID() model.Provider {
	return model.ProviderCodex
}

func (c *Client) Fetch(ctx context.Context) (model.ProviderStatus, error) {
	status := model.ProviderStatus{Provider: model.ProviderCodex, UpdatedAt: time.Now()}
	path, err := c.commandPath()
	if err != nil {
		return status, ErrNotInstalled
	}

	cmd := exec.CommandContext(ctx, path, "app-server")
	cmd.Env = environmentWithCommandDir(filepath.Dir(path))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return status, fmt.Errorf("open Codex App Server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return status, fmt.Errorf("open Codex App Server stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		c.invalidateCommandPath(path)
		return status, fmt.Errorf("start Codex App Server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	version := c.Version
	if version == "" {
		version = "dev"
	}
	requests := []any{
		map[string]any{
			"method": "initialize",
			"id":     1,
			"params": map[string]any{
				"clientInfo": map[string]string{
					"name":    "ai_quota",
					"title":   "AI quota",
					"version": version,
				},
			},
		},
		map[string]any{"method": "initialized", "params": map[string]any{}},
		map[string]any{"method": "account/rateLimits/read", "id": 2},
	}
	encoder := json.NewEncoder(stdin)
	for _, request := range requests {
		if err := encoder.Encode(request); err != nil {
			return status, fmt.Errorf("send request to Codex App Server: %w", err)
		}
	}

	response, err := readRateLimitResponse(stdout)
	if err != nil {
		if stderr.Len() > 0 {
			return status, fmt.Errorf("%w: %s", err, firstLine(stderr.String()))
		}
		return status, err
	}
	return parseRateLimitResult(response.Result, status.UpdatedAt)
}

func (c *Client) commandPath() (string, error) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	if c.resolvedCommand != "" {
		return c.resolvedCommand, nil
	}
	command := c.Command
	if command == "" {
		command = "codex"
	}
	lookup := c.lookupCommand
	if lookup == nil {
		lookup = findCommand
	}
	path, err := lookup(command)
	if err != nil {
		return "", err
	}
	c.resolvedCommand = path
	return path, nil
}

func (c *Client) invalidateCommandPath(path string) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	if c.resolvedCommand == path {
		c.resolvedCommand = ""
	}
}

func findCommand(command string) (string, error) {
	if override := os.Getenv("AIQUOTA_CODEX_PATH"); override != "" {
		if executable(override) {
			return override, nil
		}
	}
	if path, err := exec.LookPath(command); err == nil {
		return path, nil
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		"/opt/homebrew/bin/codex",
		"/usr/local/bin/codex",
		filepath.Join(home, ".local", "bin", "codex"),
		filepath.Join(home, "bin", "codex"),
		filepath.Join(home, ".npm-global", "bin", "codex"),
	}
	if matches, _ := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "codex")); len(matches) > 0 {
		candidates = append(candidates, matches...)
	}
	for _, candidate := range candidates {
		if executable(candidate) {
			return candidate, nil
		}
	}
	return "", ErrNotInstalled
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func environmentWithCommandDir(commandDirectory string) []string {
	pathParts := []string{
		commandDirectory,
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
	}
	if current := os.Getenv("PATH"); current != "" {
		pathParts = append(pathParts, current)
	}
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PATH=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "PATH="+strings.Join(pathParts, ":"))
}

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func readRateLimitResponse(reader io.Reader) (rpcResponse, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var response rpcResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			continue
		}
		if response.ID != 2 {
			continue
		}
		if response.Error != nil {
			return response, fmt.Errorf("Codex App Server (%d): %s", response.Error.Code, response.Error.Message)
		}
		return response, nil
	}
	if err := scanner.Err(); err != nil {
		return rpcResponse{}, fmt.Errorf("read Codex App Server: %w", err)
	}
	return rpcResponse{}, errors.New("Codex App Server closed before returning quota data")
}

type rateLimitResult struct {
	RateLimits          *rateLimitBucket           `json:"rateLimits"`
	RateLimitsByLimitID map[string]rateLimitBucket `json:"rateLimitsByLimitId"`
}

type rateLimitBucket struct {
	LimitID   string           `json:"limitId"`
	LimitName string           `json:"limitName"`
	PlanType  string           `json:"planType"`
	Primary   *rateLimitWindow `json:"primary"`
	Secondary *rateLimitWindow `json:"secondary"`
}

type rateLimitWindow struct {
	UsedPercent       float64 `json:"usedPercent"`
	WindowDurationMin int64   `json:"windowDurationMins"`
	ResetsAt          int64   `json:"resetsAt"`
}

func parseRateLimitResult(raw json.RawMessage, now time.Time) (model.ProviderStatus, error) {
	status := model.ProviderStatus{
		Provider:  model.ProviderCodex,
		UpdatedAt: now,
		Metadata:  make(map[string]string),
	}
	var result rateLimitResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return status, fmt.Errorf("decode Codex quota data: %w", err)
	}

	seen := make(map[string]bool)
	appendBucket := func(bucket rateLimitBucket) {
		if bucket.PlanType != "" {
			status.Metadata["plan"] = bucket.PlanType
		}
		for _, source := range []*rateLimitWindow{bucket.Primary, bucket.Secondary} {
			if source == nil || source.ResetsAt <= 0 || source.WindowDurationMin <= 0 {
				continue
			}
			key := fmt.Sprintf("%d:%d", source.WindowDurationMin, source.ResetsAt)
			if seen[key] {
				continue
			}
			seen[key] = true
			status.Windows = append(status.Windows, model.Window{
				Kind:            model.ClassifyWindow(source.WindowDurationMin),
				UsedPercent:     source.UsedPercent,
				DurationMinutes: source.WindowDurationMin,
				ResetsAt:        time.Unix(source.ResetsAt, 0),
			})
		}
	}

	if result.RateLimits != nil {
		appendBucket(*result.RateLimits)
	}
	if len(status.Windows) == 0 {
		for _, bucket := range result.RateLimitsByLimitID {
			appendBucket(bucket)
		}
	}
	status.Normalize()
	if len(status.Windows) == 0 {
		return status, errors.New("Codex did not return quota data for the current account")
	}
	return status, nil
}

func firstLine(value string) string {
	for i, character := range value {
		if character == '\n' || character == '\r' {
			return value[:i]
		}
	}
	return value
}

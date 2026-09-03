package antigravity

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chuongtrh/ai-quota/internal/model"
)

var (
	ErrLanguageServerNotFound = errors.New("no running Antigravity Language Server found")
	ErrLanguageServerNoQuota  = errors.New("Antigravity Language Server did not return quota data")
)

type userQuotaSummaryResponse struct {
	Response struct {
		Groups []quotaGroup `json:"groups"`
	} `json:"response"`
}

type quotaGroup struct {
	DisplayName string        `json:"displayName"`
	Description string        `json:"description"`
	Buckets     []quotaBucket `json:"buckets"`
}

type quotaBucket struct {
	BucketID          string   `json:"bucketId"`
	DisplayName       string   `json:"displayName"`
	Description       string   `json:"description"`
	Window            string   `json:"window"`
	RemainingFraction *float64 `json:"remainingFraction"`
	ResetTime         string   `json:"resetTime"`
}

type lsProcessInfo struct {
	PID          int
	CsrfToken    string
	ExplicitPort int
}

var (
	csrfRegex = regexp.MustCompile(`--csrf_token(?:\s+|=)([a-zA-Z0-9_-]+)`)
	portRegex = regexp.MustCompile(`--https_server_port(?:\s+|=)(\d+)`)
	pidRegex  = regexp.MustCompile(`^\s*(\d+)`)
	lsofRegex = regexp.MustCompile(`:(\d+)\s+\(LISTEN\)`)
)

func ParseLanguageServerQuota(data []byte, now time.Time) (model.ProviderStatus, error) {
	status := model.ProviderStatus{Provider: model.ProviderAntigravity, UpdatedAt: now}
	var res userQuotaSummaryResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return status, fmt.Errorf("decode Antigravity Language Server quota: %w", err)
	}

	for _, group := range res.Response.Groups {
		for _, bucket := range group.Buckets {
			if bucket.BucketID == "" || bucket.RemainingFraction == nil ||
				math.IsNaN(*bucket.RemainingFraction) || math.IsInf(*bucket.RemainingFraction, 0) {
				continue
			}
			reset, err := time.Parse(time.RFC3339Nano, bucket.ResetTime)
			if err != nil {
				continue
			}
			label := formatBucketLabel(group.DisplayName, bucket.DisplayName, bucket.BucketID)
			status.Windows = append(status.Windows, model.Window{
				Kind:        model.WindowKind(bucket.BucketID),
				Label:       label,
				UsedPercent: 100 * (1 - *bucket.RemainingFraction),
				ResetsAt:    reset,
			})
		}
	}
	status.Normalize()
	if len(status.Windows) == 0 {
		return status, ErrLanguageServerNoQuota
	}
	return status, nil
}

func formatBucketLabel(groupName, bucketName, bucketID string) string {
	prefix := ""
	groupLower := strings.ToLower(groupName)
	switch {
	case strings.Contains(groupLower, "gemini"):
		prefix = "Gemini"
	case strings.Contains(groupLower, "claude") || strings.Contains(groupLower, "gpt"):
		prefix = "Claude/GPT"
	case groupName != "":
		prefix = strings.TrimSuffix(groupName, " Models")
		prefix = strings.TrimSuffix(prefix, " models")
	}

	windowName := ""
	nameLower := strings.ToLower(bucketName)
	idLower := strings.ToLower(bucketID)
	switch {
	case strings.Contains(nameLower, "weekly") || strings.Contains(idLower, "weekly"):
		windowName = "Weekly"
	case strings.Contains(nameLower, "five hour") || strings.Contains(nameLower, "5-hour") || strings.Contains(idLower, "5h"):
		windowName = "5-Hour"
	case strings.Contains(nameLower, "daily") || strings.Contains(idLower, "daily"):
		windowName = "Daily"
	default:
		windowName = bucketLabel(bucketID)
	}

	if prefix != "" && !strings.Contains(windowName, prefix) {
		return prefix + " " + windowName
	}
	return windowName
}

func FetchFromLanguageServer(ctx context.Context) (model.ProviderStatus, error) {
	candidates, err := discoverLanguageServerProcesses(ctx)
	if err != nil || len(candidates) == 0 {
		return model.ProviderStatus{Provider: model.ProviderAntigravity}, ErrLanguageServerNotFound
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Localhost language server uses internal self-signed TLS cert
		DialContext: (&net.Dialer{
			Timeout: 1 * time.Second,
		}).DialContext,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   2 * time.Second,
	}
	defer tr.CloseIdleConnections()

	for _, proc := range candidates {
		ports := getProcessPorts(ctx, proc.PID)
		if proc.ExplicitPort > 0 {
			ports = append([]int{proc.ExplicitPort}, ports...)
		}
		for _, port := range ports {
			for _, proto := range []string{"https", "http"} {
				reqCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
				status, err := queryEndpoint(reqCtx, client, proto, port, proc.CsrfToken)
				cancel()
				if err == nil && len(status.Windows) > 0 {
					return status, nil
				}
			}
		}
	}

	return model.ProviderStatus{Provider: model.ProviderAntigravity}, ErrLanguageServerNoQuota
}

func queryEndpoint(ctx context.Context, client *http.Client, proto string, port int, csrf string) (model.ProviderStatus, error) {
	url := fmt.Sprintf("%s://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary", proto, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return model.ProviderStatus{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-codeium-csrf-token", csrf)

	resp, err := client.Do(req)
	if err != nil {
		return model.ProviderStatus{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.ProviderStatus{}, fmt.Errorf("language server HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return model.ProviderStatus{}, err
	}

	return ParseLanguageServerQuota(data, time.Now())
}

func discoverLanguageServerProcesses(ctx context.Context) ([]lsProcessInfo, error) {
	cmd := exec.CommandContext(ctx, "ps", "-A", "-o", "pid,command")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var results []lsProcessInfo
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "language_server") || !strings.Contains(line, "--csrf_token") {
			continue
		}
		mPID := pidRegex.FindStringSubmatch(line)
		mCsrf := csrfRegex.FindStringSubmatch(line)
		if len(mPID) < 2 || len(mCsrf) < 2 {
			continue
		}
		pid, err := strconv.Atoi(mPID[1])
		if err != nil {
			continue
		}
		info := lsProcessInfo{
			PID:       pid,
			CsrfToken: mCsrf[1],
		}
		if mPort := portRegex.FindStringSubmatch(line); len(mPort) >= 2 {
			if port, err := strconv.Atoi(mPort[1]); err == nil && port > 0 {
				info.ExplicitPort = port
			}
		}
		results = append(results, info)
	}
	return results, nil
}

func getProcessPorts(ctx context.Context, pid int) []int {
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-a", "-p", strconv.Itoa(pid), "-iTCP", "-sTCP:LISTEN")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	matches := lsofRegex.FindAllStringSubmatch(string(out), -1)
	seen := make(map[int]bool)
	var ports []int
	for _, m := range matches {
		if len(m) >= 2 {
			if port, err := strconv.Atoi(m[1]); err == nil && !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}
	return ports
}

package tray

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/systray"

	"github.com/chuongtrh/ai-quota/internal/appcore"
	appicon "github.com/chuongtrh/ai-quota/internal/icon"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/notify"
	"github.com/chuongtrh/ai-quota/internal/provider/claude"
)

type App struct {
	service   *appcore.Service
	installer claude.Installer
	version   string
	ctx       context.Context
	cancel    context.CancelFunc

	codexHeader   *systray.MenuItem
	codexSession  *systray.MenuItem
	codexWeekly   *systray.MenuItem
	claudeHeader  *systray.MenuItem
	claudeSession *systray.MenuItem
	claudeWeekly  *systray.MenuItem
	updated       *systray.MenuItem
	connectClaude *systray.MenuItem
	refresh       *systray.MenuItem
	quit          *systray.MenuItem
}

func New(service *appcore.Service, installer claude.Installer, version string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		service:   service,
		installer: installer,
		version:   version,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (a *App) Run() {
	systray.Run(a.onReady, a.onExit)
}

func (a *App) onReady() {
	trayIcon := appicon.TrayPNG(36)
	systray.SetTemplateIcon(trayIcon, trayIcon)
	systray.SetTitle("🤖 —")
	systray.SetTooltip("AI quota · Codex and Claude Code")
	systray.SetRemovalAllowed(false)

	a.codexHeader = disabledItem("Codex")
	a.codexSession = disabledItem("  Session —")
	a.codexWeekly = disabledItem("  Weekly —")
	systray.AddSeparator()
	a.claudeHeader = disabledItem("Claude Code")
	a.claudeSession = disabledItem("  Session —")
	a.claudeWeekly = disabledItem("  Weekly —")
	systray.AddSeparator()
	disabledItem("🔔 Alerts at 20% and 5% remaining")
	a.updated = disabledItem("🔄 Not updated yet")
	a.refresh = systray.AddMenuItem("↻ Refresh now", "Fetch the latest quota data")
	a.connectClaude = systray.AddMenuItem("🔌 Connect Claude Code", "Install the official status line connector")
	systray.AddSeparator()
	disabledItem("ℹ️ Version " + a.version)
	a.quit = systray.AddMenuItem("⏻ Quit", "Quit AI quota")

	notify.RequestPermission()
	go a.service.Refresh(a.ctx)
	go a.eventLoop()
	a.updateMenu()
}

func (a *App) eventLoop() {
	refreshTicker := time.NewTicker(60 * time.Second)
	uiTicker := time.NewTicker(2 * time.Second)
	defer refreshTicker.Stop()
	defer uiTicker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-refreshTicker.C:
			go a.service.Refresh(a.ctx)
		case <-uiTicker.C:
			a.updateMenu()
		case <-a.refresh.ClickedCh:
			go a.service.Refresh(a.ctx)
		case <-a.connectClaude.ClickedCh:
			a.toggleClaudeConnection()
		case <-a.quit.ClickedCh:
			systray.Quit()
		}
	}
}

func (a *App) updateMenu() {
	statuses := a.service.Statuses()
	a.updateProvider(
		statuses[model.ProviderCodex],
		a.codexHeader,
		a.codexSession,
		a.codexWeekly,
	)
	a.updateProvider(
		statuses[model.ProviderClaudeCode],
		a.claudeHeader,
		a.claudeSession,
		a.claudeWeekly,
	)

	if remaining, ok := a.service.MostUrgentRemaining(time.Now()); ok {
		severity := model.SeverityForRemaining(float64(remaining))
		systray.SetTitle(fmt.Sprintf("%s %d%%", severity.Emoji(), remaining))
	} else {
		systray.SetTitle("🤖 —")
	}

	latest := time.Time{}
	for _, status := range statuses {
		if status.UpdatedAt.After(latest) {
			latest = status.UpdatedAt
		}
	}
	if latest.IsZero() {
		a.updated.SetTitle("🔄 No data yet")
	} else {
		a.updated.SetTitle("🔄 Updated " + formatAgo(time.Since(latest)))
	}

	connected, err := a.installer.IsConnected()
	if err == nil && connected {
		a.connectClaude.SetTitle("🔌 Disconnect Claude Code")
		a.connectClaude.SetTooltip("Restore the backed-up status line")
	} else {
		a.connectClaude.SetTitle("🔌 Connect Claude Code")
		a.connectClaude.SetTooltip("Install the official status line connector")
	}
}

func (a *App) updateProvider(
	status model.ProviderStatus,
	header *systray.MenuItem,
	sessionItem *systray.MenuItem,
	weeklyItem *systray.MenuItem,
) {
	headerTitle := status.Provider.DisplayName()
	if headerTitle == "" {
		headerTitle = "—"
	}
	if status.Error != "" {
		headerTitle = "⚠️ " + headerTitle
	}
	header.SetTitle(headerTitle)

	if len(status.Windows) == 0 && status.Error != "" {
		sessionItem.SetTitle("  " + shorten(status.Error, 54))
		weeklyItem.SetTitle("  Weekly —")
		return
	}
	sessionItem.SetTitle(formatWindow(status, model.WindowSession, time.Now()))
	weeklyItem.SetTitle(formatWindow(status, model.WindowWeekly, time.Now()))
}

func (a *App) toggleClaudeConnection() {
	connected, err := a.installer.IsConnected()
	if err != nil {
		_ = notify.Send("⚠️ Could not read Claude Code settings", err.Error())
		return
	}
	if connected {
		err = a.installer.Disconnect()
		if err == nil {
			_ = notify.Send("AI quota", "Claude Code was disconnected and its previous status line was restored.")
		}
	} else {
		err = a.installer.Connect()
		if err == nil {
			_ = notify.Send("AI quota", "Claude Code is connected. Send a prompt to receive the latest quota data.")
		}
	}
	if err != nil {
		message := err.Error()
		if errors.Is(err, claude.ErrSettingsChanged) {
			message = "The status line changed after connection. AI quota will not overwrite the current settings."
		}
		_ = notify.Send("⚠️ Could not update Claude Code", message)
	}
	a.updateMenu()
}

func (a *App) onExit() {
	a.cancel()
}

func disabledItem(title string) *systray.MenuItem {
	item := systray.AddMenuItem(title, "")
	item.Disable()
	return item
}

func formatWindow(status model.ProviderStatus, kind model.WindowKind, now time.Time) string {
	window, exists := status.Window(kind)
	if !exists {
		return fmt.Sprintf("  %s —", kind.DisplayName())
	}
	if !window.ResetsAt.After(now) {
		return fmt.Sprintf("⚪ %s · waiting for new data", kind.DisplayName())
	}
	remaining := window.RemainingPercent()
	return fmt.Sprintf(
		"%s %s · %d%% left · %s",
		model.SeverityForRemaining(remaining).Emoji(),
		kind.DisplayName(),
		window.RoundedRemainingPercent(),
		model.FormatReset(now, window.ResetsAt),
	)
}

func formatAgo(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		return fmt.Sprintf("%d min ago", int(duration/time.Minute))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%d hr ago", int(duration/time.Hour))
	default:
		return fmt.Sprintf("%d days ago", int(duration/(24*time.Hour)))
	}
}

func shorten(value string, limit int) string {
	value = strings.TrimSpace(value)
	characters := []rune(value)
	if len(characters) <= limit {
		return value
	}
	return string(characters[:limit-1]) + "…"
}

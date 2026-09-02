package tray

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fyne.io/systray"

	"github.com/chuongtrh/ai-quota/internal/appcore"
	appicon "github.com/chuongtrh/ai-quota/internal/icon"
	"github.com/chuongtrh/ai-quota/internal/model"
	"github.com/chuongtrh/ai-quota/internal/notify"
	"github.com/chuongtrh/ai-quota/internal/provider/claude"
)

type App struct {
	service          *appcore.Service
	installer        claude.Installer
	claudeConnection connectionStateCache
	version          string
	ctx              context.Context
	cancel           context.CancelFunc

	quotaItems    map[model.Provider]providerQuotaItems
	providerItems map[model.Provider]providerControlItems
	waiting       *systray.MenuItem
	updated       *systray.MenuItem
	refresh       *systray.MenuItem
	quit          *systray.MenuItem
}

type connectionStateCache struct {
	check       func() (bool, error)
	interval    time.Duration
	now         func() time.Time
	checkedAt   time.Time
	initialized bool
	connected   bool
	err         error
}

func (c *connectionStateCache) Get(force bool) (bool, error) {
	now := c.now()
	if !force && c.initialized && now.Sub(c.checkedAt) < c.interval {
		return c.connected, c.err
	}
	c.connected, c.err = c.check()
	c.checkedAt = now
	c.initialized = true
	return c.connected, c.err
}

type providerQuotaItems struct {
	header  *systray.MenuItem
	session *systray.MenuItem
	weekly  *systray.MenuItem
}

func (items providerQuotaItems) Show() {
	items.header.Show()
	items.session.Show()
	items.weekly.Show()
}

func (items providerQuotaItems) Hide() {
	items.header.Hide()
	items.session.Hide()
	items.weekly.Hide()
}

type providerControlItems struct {
	status *systray.MenuItem
	action *systray.MenuItem
}

var providerOrder = []model.Provider{
	model.ProviderCodex,
	model.ProviderClaudeCode,
}

func New(service *appcore.Service, installer claude.Installer, version string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		service:   service,
		installer: installer,
		claudeConnection: connectionStateCache{
			check:    installer.IsConnected,
			interval: time.Minute,
			now:      time.Now,
		},
		version:       version,
		ctx:           ctx,
		cancel:        cancel,
		quotaItems:    make(map[model.Provider]providerQuotaItems),
		providerItems: make(map[model.Provider]providerControlItems),
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

	for _, provider := range providerOrder {
		items := providerQuotaItems{
			header:  disabledItem(provider.DisplayName()),
			session: disabledItem("  Session —"),
			weekly:  disabledItem("  Weekly —"),
		}
		items.Hide()
		a.quotaItems[provider] = items
	}
	a.waiting = disabledItem("⏳ Waiting for provider setup")
	systray.AddSeparator()
	disabledItem("🔔 Alerts at 20% and 5% remaining")
	a.updated = disabledItem("🔄 Not updated yet")
	a.refresh = systray.AddMenuItem("↻ Refresh now", "Fetch the latest quota data")
	providers := systray.AddMenuItem("🧩 Providers", "Manage quota tracking providers")
	for _, provider := range providerOrder {
		providerMenu := providers.AddSubMenuItem(provider.DisplayName(), "Manage "+provider.DisplayName()+" quota tracking")
		status := providerMenu.AddSubMenuItem("⏳ Checking status", "")
		status.Disable()
		action := providerMenu.AddSubMenuItem(initialProviderAction(provider), initialProviderActionTooltip(provider))
		a.providerItems[provider] = providerControlItems{status: status, action: action}
	}
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
	uiTicker := time.NewTicker(30 * time.Second)
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
		case <-a.providerItems[model.ProviderCodex].action.ClickedCh:
			go a.service.Refresh(a.ctx)
		case <-a.providerItems[model.ProviderClaudeCode].action.ClickedCh:
			a.toggleClaudeTracking()
		case <-a.quit.ClickedCh:
			systray.Quit()
		}
	}
}

func (a *App) updateMenu() {
	statuses := a.service.Statuses()
	claudeConnected, claudeSettingsErr := a.claudeConnection.Get(false)
	visibleProviders := make(map[model.Provider]bool, len(providerOrder))
	visibleCount := 0
	for _, provider := range providerOrder {
		visible := len(statuses[provider].Windows) > 0
		if provider == model.ProviderClaudeCode {
			visible = visible && claudeSettingsErr == nil && claudeConnected
		}
		visibleProviders[provider] = visible
		if a.updateProvider(statuses[provider], a.quotaItems[provider], visible) {
			visibleCount++
		}
	}
	if visibleCount == 0 {
		if claudeConnected && claudeSettingsErr == nil {
			a.waiting.SetTitle("⏳ Waiting for quota data")
		} else {
			a.waiting.SetTitle("⏳ Waiting for provider setup")
		}
		a.waiting.Show()
	} else {
		a.waiting.Hide()
	}

	if remaining, ok := mostUrgentRemaining(statuses, visibleProviders, time.Now()); ok {
		severity := model.SeverityForRemaining(float64(remaining))
		systray.SetTitle(fmt.Sprintf("%s %d%%", severity.Emoji(), remaining))
	} else {
		systray.SetTitle("🤖 —")
	}

	latest := time.Time{}
	for provider, status := range statuses {
		if !visibleProviders[provider] {
			continue
		}
		if status.UpdatedAt.After(latest) {
			latest = status.UpdatedAt
		}
	}
	if latest.IsZero() {
		a.updated.SetTitle("🔄 No data yet")
	} else {
		a.updated.SetTitle("🔄 Updated " + formatAgo(time.Since(latest)))
	}

	a.updateCodexProviderMenu(statuses[model.ProviderCodex])
	a.updateClaudeProviderMenu(statuses[model.ProviderClaudeCode], claudeConnected, claudeSettingsErr)
}

func (a *App) updateProvider(
	status model.ProviderStatus,
	items providerQuotaItems,
	visible bool,
) bool {
	if !visible {
		items.Hide()
		return false
	}
	items.Show()
	headerTitle := status.Provider.DisplayName()
	if headerTitle == "" {
		headerTitle = "—"
	}
	if status.Error != "" {
		headerTitle = "⚠️ " + headerTitle
	}
	items.header.SetTitle(headerTitle)
	items.session.SetTitle(formatWindow(status, model.WindowSession, time.Now()))
	items.weekly.SetTitle(formatWindow(status, model.WindowWeekly, time.Now()))
	return true
}

func (a *App) toggleClaudeTracking() {
	connected, err := a.claudeConnection.Get(true)
	if err != nil {
		_ = notify.Send("⚠️ Could not read Claude Code settings", err.Error())
		return
	}
	if connected {
		err = a.installer.Disconnect()
		if err == nil {
			a.service.ClearProvider(model.ProviderClaudeCode)
			_ = notify.Send("AI quota", "Claude Code tracking was disabled and its previous status line was restored.")
		}
	} else {
		err = a.installer.Connect()
		if err == nil {
			a.service.ClearProvider(model.ProviderClaudeCode)
			_ = notify.Send("AI quota", "Claude Code tracking is enabled. Send a prompt to receive the latest quota data.")
		}
	}
	if err != nil {
		message := err.Error()
		if errors.Is(err, claude.ErrSettingsChanged) {
			message = "The status line changed after tracking was enabled. AI quota will not overwrite the current settings."
		}
		_ = notify.Send("⚠️ Could not update Claude Code tracking", message)
	}
	_, _ = a.claudeConnection.Get(true)
	a.updateMenu()
}

func (a *App) updateCodexProviderMenu(status model.ProviderStatus) {
	items := a.providerItems[model.ProviderCodex]
	items.action.SetTitle("Check availability")
	items.action.SetTooltip("Check the local Codex CLI and refresh quota data")
	switch {
	case len(status.Windows) > 0 && status.Error == "":
		items.status.SetTitle("🟢 Tracking active")
		items.status.SetTooltip("Codex quota data is available")
	case len(status.Windows) > 0:
		items.status.SetTitle("🟡 Showing cached quota")
		items.status.SetTooltip(status.Error)
	case status.Error != "":
		items.status.SetTitle("⚪ Setup required")
		items.status.SetTooltip(status.Error)
	default:
		items.status.SetTitle("⏳ Checking availability")
		items.status.SetTooltip("Waiting for the first Codex quota refresh")
	}
}

func (a *App) updateClaudeProviderMenu(status model.ProviderStatus, connected bool, settingsErr error) {
	items := a.providerItems[model.ProviderClaudeCode]
	if settingsErr != nil {
		items.status.SetTitle("⚠️ Settings unavailable")
		items.status.SetTooltip(settingsErr.Error())
		items.action.SetTitle("Enable tracking")
		items.action.SetTooltip("Install the official Claude Code status line connector")
		return
	}
	if !connected {
		items.status.SetTitle("⚪ Tracking disabled")
		items.status.SetTooltip("Claude Code quota tracking is not configured")
		items.action.SetTitle("Enable tracking")
		items.action.SetTooltip("Install the official Claude Code status line connector")
		return
	}
	if len(status.Windows) == 0 {
		items.status.SetTitle("⏳ Waiting for quota data")
		items.status.SetTooltip("Send a Claude Code prompt to receive quota data")
	} else if status.Error != "" {
		items.status.SetTitle("🟡 Showing cached quota")
		items.status.SetTooltip(status.Error)
	} else {
		items.status.SetTitle("🟢 Tracking active")
		items.status.SetTooltip("Claude Code quota data is available")
	}
	items.action.SetTitle("Disable tracking")
	items.action.SetTooltip("Restore the previous Claude Code status line")
}

func initialProviderAction(provider model.Provider) string {
	if provider == model.ProviderCodex {
		return "Check availability"
	}
	return "Enable tracking"
}

func initialProviderActionTooltip(provider model.Provider) string {
	if provider == model.ProviderCodex {
		return "Check the local Codex CLI and refresh quota data"
	}
	return "Enable quota tracking for " + provider.DisplayName()
}

func mostUrgentRemaining(
	statuses map[model.Provider]model.ProviderStatus,
	visible map[model.Provider]bool,
	now time.Time,
) (int, bool) {
	minimum := 101
	found := false
	for provider, status := range statuses {
		if !visible[provider] {
			continue
		}
		for _, window := range status.Windows {
			if !window.ResetsAt.After(now) {
				continue
			}
			remaining := window.RoundedRemainingPercent()
			if remaining < minimum {
				minimum = remaining
				found = true
			}
		}
	}
	return minimum, found
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
	return formatQuotaWindow(window, now)
}

func windowRows(status model.ProviderStatus, now time.Time) []string {
	rows := make([]string, 0, len(status.Windows))
	for _, window := range status.Windows {
		rows = append(rows, formatQuotaWindow(window, now))
	}
	return rows
}

func formatQuotaWindow(window model.Window, now time.Time) string {
	if !window.ResetsAt.After(now) {
		return fmt.Sprintf("⚪ %s · waiting for new data", window.DisplayName())
	}
	remaining := window.RemainingPercent()
	return fmt.Sprintf(
		"%s %s · %d%% left · %s",
		model.SeverityForRemaining(remaining).Emoji(),
		window.DisplayName(),
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

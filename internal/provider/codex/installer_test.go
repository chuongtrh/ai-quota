package codex

import (
	"testing"

	"github.com/chuongtrh/ai-quota/internal/config"
)

func TestInstallerConnectDisconnect(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DataDir: root, HomeDir: root}
	installer := Installer{Paths: paths}

	// Initially connected (no disabled file)
	connected, err := installer.IsConnected()
	if err != nil || !connected {
		t.Fatalf("expected connected initially, got connected=%v, err=%v", connected, err)
	}

	// Disconnect creates disabled marker
	if err := installer.Disconnect(); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
	connected, err = installer.IsConnected()
	if err != nil || connected {
		t.Fatalf("expected disconnected, got connected=%v, err=%v", connected, err)
	}

	// Connect removes disabled marker
	if err := installer.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	connected, err = installer.IsConnected()
	if err != nil || !connected {
		t.Fatalf("expected connected after re-enabling, got connected=%v, err=%v", connected, err)
	}
}

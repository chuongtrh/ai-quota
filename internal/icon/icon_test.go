package icon

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/chuongtrh/ai-quota/internal/model"
)

func TestTrayPNGGeneratesValidPNG(t *testing.T) {
	data := TrayPNG(36)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode TrayPNG: %v", err)
	}
	if img.Bounds().Dx() != 36 || img.Bounds().Dy() != 36 {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
}

func TestTrayColorPNGSeverities(t *testing.T) {
	severities := []model.Severity{
		model.SeverityHealthy,
		model.SeverityWarning,
		model.SeverityCritical,
		model.SeverityExhausted,
	}

	for _, sev := range severities {
		data := TrayColorPNG(36, sev, true)
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode TrayColorPNG(%v): %v", sev, err)
		}
		if img.Bounds().Dx() != 36 || img.Bounds().Dy() != 36 {
			t.Fatalf("unexpected bounds for severity %v: %v", sev, img.Bounds())
		}
	}
}

func TestStatusDotPNGSeverities(t *testing.T) {
	severities := []model.Severity{
		model.SeverityHealthy,
		model.SeverityWarning,
		model.SeverityCritical,
		model.SeverityExhausted,
	}

	for _, sev := range severities {
		data := StatusDotPNG(16, sev, true)
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode StatusDotPNG(%v): %v", sev, err)
		}
		if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 16 {
			t.Fatalf("unexpected bounds for severity %v: %v", sev, img.Bounds())
		}
	}
}

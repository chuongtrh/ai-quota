package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/chuongtrh/ai-quota/internal/model"
)

func TrayPNG(size int) []byte {
	return TrayColorPNG(size, model.SeverityHealthy, false)
}

func TrayColorPNG(size int, severity model.Severity, active bool) []byte {
	if size < 16 {
		size = 16
	}
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size-1) / 2
	radius := float64(size) * 0.34
	thickness := math.Max(1.5, float64(size)*0.085)

	iconColor := color.RGBA{R: 142, G: 142, B: 147, A: 255}
	if active {
		switch severity {
		case model.SeverityExhausted:
			iconColor = color.RGBA{R: 215, G: 0, B: 21, A: 255}
		case model.SeverityCritical:
			iconColor = color.RGBA{R: 255, G: 69, B: 58, A: 255}
		case model.SeverityWarning:
			iconColor = color.RGBA{R: 255, G: 159, B: 10, A: 255}
		default:
			iconColor = color.RGBA{R: 50, G: 205, B: 90, A: 255}
		}
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			distance := math.Hypot(float64(x)-center, float64(y)-center)
			if math.Abs(distance-radius) <= thickness {
				canvas.SetRGBA(x, y, iconColor)
			}
		}
	}
	drawLine(canvas, center, center, center+radius*0.62, center-radius*0.52, thickness, iconColor)
	drawCircle(canvas, center, center, thickness*1.2, iconColor)

	var buffer bytes.Buffer
	_ = png.Encode(&buffer, canvas)
	return buffer.Bytes()
}

func StatusDotPNG(size int, severity model.Severity, active bool) []byte {
	if size < 16 {
		size = 32
	}
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size) / 2
	radius := float64(size) * 0.21

	dotColor := color.RGBA{R: 142, G: 142, B: 147, A: 255}
	if active {
		switch severity {
		case model.SeverityExhausted:
			dotColor = color.RGBA{R: 215, G: 0, B: 21, A: 255}
		case model.SeverityCritical:
			dotColor = color.RGBA{R: 255, G: 59, B: 48, A: 255}
		case model.SeverityWarning:
			dotColor = color.RGBA{R: 255, G: 149, B: 0, A: 255}
		default:
			dotColor = color.RGBA{R: 52, G: 199, B: 89, A: 255}
		}
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)+0.5-center, float64(y)+0.5-center)
			delta := radius - d
			if delta >= 0.5 {
				canvas.SetRGBA(x, y, dotColor)
			} else if delta > -0.5 {
				coverage := delta + 0.5
				alpha := uint8(float64(dotColor.A) * coverage)
				canvas.SetRGBA(x, y, color.RGBA{R: dotColor.R, G: dotColor.G, B: dotColor.B, A: alpha})
			}
		}
	}
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, canvas)
	return buffer.Bytes()
}

func AppImage(size int) image.Image {
	if size < 16 {
		size = 16
	}
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	corner := float64(size) * 0.22
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !insideRoundedSquare(x, y, size, corner) {
				continue
			}
			ratio := float64(x+y) / float64(2*size)
			canvas.SetRGBA(x, y, color.RGBA{
				R: uint8(38 + 55*ratio),
				G: uint8(109 + 28*ratio),
				B: uint8(245 - 25*ratio),
				A: 255,
			})
		}
	}

	center := float64(size-1) / 2
	radius := float64(size) * 0.29
	thickness := math.Max(2, float64(size)*0.055)
	white := color.RGBA{R: 255, G: 255, B: 255, A: 245}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - center
			dy := float64(y) - center
			distance := math.Hypot(dx, dy)
			angle := math.Atan2(dy, dx)
			if math.Abs(distance-radius) <= thickness && angle > -2.85 && angle < 2.85 {
				canvas.SetRGBA(x, y, white)
			}
		}
	}
	drawLine(canvas, center, center, center+radius*0.62, center-radius*0.58, thickness*0.85, white)
	drawCircle(canvas, center, center, thickness*1.25, white)
	return canvas
}

func insideRoundedSquare(x, y, size int, radius float64) bool {
	fx := float64(x)
	fy := float64(y)
	maximum := float64(size - 1)
	if fx >= radius && fx <= maximum-radius {
		return true
	}
	if fy >= radius && fy <= maximum-radius {
		return true
	}
	cx := radius
	if fx > maximum-radius {
		cx = maximum - radius
	}
	cy := radius
	if fy > maximum-radius {
		cy = maximum - radius
	}
	return math.Hypot(fx-cx, fy-cy) <= radius
}

func drawLine(canvas *image.RGBA, x1, y1, x2, y2, width float64, value color.RGBA) {
	bounds := canvas.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if distanceToSegment(float64(x), float64(y), x1, y1, x2, y2) <= width {
				canvas.SetRGBA(x, y, value)
			}
		}
	}
}

func drawCircle(canvas *image.RGBA, cx, cy, radius float64, value color.RGBA) {
	bounds := canvas.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if math.Hypot(float64(x)-cx, float64(y)-cy) <= radius {
				canvas.SetRGBA(x, y, value)
			}
		}
	}
}

func distanceToSegment(px, py, x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	if dx == 0 && dy == 0 {
		return math.Hypot(px-x1, py-y1)
	}
	t := ((px-x1)*dx + (py-y1)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(x1+t*dx), py-(y1+t*dy))
}

package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	appicon "github.com/chuongtrh/ai-quota/internal/icon"
)

func main() {
	output := flag.String("out", "build/AIQuota.iconset", "output iconset directory")
	flag.Parse()
	if err := os.MkdirAll(*output, 0o755); err != nil {
		fatal(err)
	}
	icons := map[string]int{
		"icon_16x16.png":      16,
		"icon_16x16@2x.png":   32,
		"icon_32x32.png":      32,
		"icon_32x32@2x.png":   64,
		"icon_128x128.png":    128,
		"icon_128x128@2x.png": 256,
		"icon_256x256.png":    256,
		"icon_256x256@2x.png": 512,
		"icon_512x512.png":    512,
		"icon_512x512@2x.png": 1024,
	}
	for name, size := range icons {
		path := filepath.Join(*output, name)
		file, err := os.Create(path)
		if err != nil {
			fatal(err)
		}
		if err := png.Encode(file, appicon.AppImage(size)); err != nil {
			_ = file.Close()
			fatal(err)
		}
		if err := file.Close(); err != nil {
			fatal(err)
		}
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

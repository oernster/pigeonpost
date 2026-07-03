// Command installer is the bespoke PigeonPost setup program, built as a Wails app so it shares the
// application's WebView and dark theme. It carries the built application as an embedded zip payload
// and supports install, repair, upgrade and uninstall, all per-user with no administrator rights.
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed payload.zip
var payload []byte

// appVersion is overridden at build time via -ldflags "-X main.appVersion=x.y.z".
var appVersion = "dev"

const (
	windowTitle = "PigeonPost Setup"
	windowW     = 640
	windowH     = 480
)

func main() {
	app := NewApp(payload, appVersion)
	_ = wails.Run(&options.App{
		Title:            windowTitle,
		Width:            windowW,
		Height:           windowH,
		DisableResize:    true,
		BackgroundColour: &options.RGBA{R: 22, G: 24, B: 29, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	})
}

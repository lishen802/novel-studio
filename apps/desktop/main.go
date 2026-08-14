package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	macoptions "github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := newApp()
	err := wails.Run(&options.App{
		Title:  "织境 Story Studio",
		Width:  1440,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 241, G: 235, B: 222, A: 1},
		Mac: &macoptions.Options{
			TitleBar: macoptions.TitleBarHiddenInset(),
		},
		Menu:          app.menu(),
		OnStartup:     app.startup,
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.shutdown,
		Bind: []interface{}{
			app.api,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

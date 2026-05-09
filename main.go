package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var frontend embed.FS

func main() {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	app := NewApp()
	err = wails.Run(&options.App{
		Title:            "Claude Desktop Gateway",
		Width:            1180,
		Height:           760,
		MinWidth:         920,
		MinHeight:        620,
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

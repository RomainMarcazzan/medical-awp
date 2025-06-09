package main

import (
	"embed"
	"log" // Import the log package

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger" // Import Wails logger
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "medical-awp",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		LogLevel: logger.DEBUG, // Set the LogLevel for development
	})

	if err != nil {
		log.Fatal(err) // Use log.Fatal for critical errors
	}
}

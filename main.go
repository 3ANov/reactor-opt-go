package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"reactoropt/internal/app"
	"reactoropt/internal/storage"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() { application.RegisterEvent[app.JobEvent]("job:updated") }

func main() {
	config, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dataDir := flag.String("data-dir", filepath.Join(config, "ReactorOptGo"), "Каталог данных ReactorOpt")
	flag.Parse()
	dbPath := filepath.Join(*dataDir, "reactoropt.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	var desktop *application.App
	service := app.New(store, func(event app.JobEvent) {
		if desktop != nil {
			desktop.Event.Emit("job:updated", event)
		}
	})
	defer service.Close()
	desktop = application.New(application.Options{
		Name: "ReactorOpt", Description: "Оптимизация технологических процессов",
		Services: []application.Service{application.NewService(service)},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Windows: application.WindowsOptions{
			WebviewUserDataPath: filepath.Join(*dataDir, "webview2"),
		},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "ReactorOpt — Оптимизация процессов",
		Width:            1320,
		Height:           860,
		MinWidth:         800,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(16, 24, 39),
		URL:              "/",
	})
	if err := desktop.Run(); err != nil {
		log.Print(err)
	}
}

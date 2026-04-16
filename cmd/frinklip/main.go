package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/eduard256/frinklip/internal/api"
	"github.com/eduard256/frinklip/internal/upload"
	"github.com/eduard256/frinklip/internal/webui"
	"github.com/eduard256/frinklip/pkg/app"
)

type Module struct {
	Name string
	Init func()
}

func main() {
	config := flag.String("config", "", "optional path to frinklip.yaml")
	level := flag.String("log", "info", "log level: debug|info|warn|error")
	flag.Parse()

	setupLogger(*level)

	app.SetConfigPath(*config)

	modules := []Module{
		{"api", api.Init},
		{"upload", upload.Init},
		{"webui", webui.Init},
	}
	for _, m := range modules {
		m.Init()
	}

	// block forever — all work happens in goroutines started by modules
	select {}
}

func setupLogger(lvl string) {
	var l slog.Level
	switch lvl {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
}

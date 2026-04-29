package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/eduard256/frinklip/internal/api"
	"github.com/eduard256/frinklip/internal/upload"
	"github.com/eduard256/frinklip/internal/webui"
	"github.com/eduard256/frinklip/pkg/app"
)

// Build-time metadata. Populated via -ldflags="-X main.Version=... -X main.Commit=... -X main.Date=..."
// at release build (see .goreleaser.yaml). Local `go build` leaves the defaults below.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type Module struct {
	Name string
	Init func()
}

func main() {
	config := flag.String("config", "", "optional path to frinklip.yaml")
	level := flag.String("log", "info", "log level: debug|info|warn|error")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("frinklip %s (commit %s, built %s)\n", Version, Commit, Date)
		return
	}

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

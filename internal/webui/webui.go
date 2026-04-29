package webui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"

	"github.com/eduard256/frinklip/internal/api"
)

//go:embed all:static
var staticFS embed.FS

// Init serves embedded static files at /
func Init() {
	// Go's mime package doesn't ship a default for .webmanifest; without
	// this Chrome installs the PWA but warns about the manifest MIME.
	// Register both the canonical and a few common adjacent types.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	api.HandleFunc("/", http.FileServer(http.FS(sub)).ServeHTTP)
}

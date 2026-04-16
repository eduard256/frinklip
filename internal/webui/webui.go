package webui

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/eduard256/frinklip/internal/api"
)

//go:embed all:static
var staticFS embed.FS

// Init serves embedded static files at /
func Init() {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	api.HandleFunc("/", http.FileServer(http.FS(sub)).ServeHTTP)
}

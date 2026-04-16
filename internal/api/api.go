package api

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/eduard256/frinklip/pkg/app"
)

var (
	log *slog.Logger

	mu    sync.Mutex
	mux   = http.NewServeMux()
	paths = map[string]bool{}
)

// Init reads api.listen (default :80), starts http server in a goroutine.
// Handlers are registered by other modules via api.HandleFunc before or after Init.
func Init() {
	var conf struct {
		Mod struct {
			Listen string `yaml:"listen"`
		} `yaml:"api"`
	}

	conf.Mod.Listen = ":80"

	app.LoadConfig(&conf)
	log = app.GetLogger("api")

	addr := conf.Mod.Listen
	if addr == "" {
		log.Info("disabled")
		return
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("listen", "addr", addr, "err", err)
		return
	}

	log.Info("listen", "addr", addr)

	go func() {
		// logging wrapper — one line per request
		h := http.Handler(mux)
		h = withLog(h)

		srv := &http.Server{Handler: h}
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("serve", "err", err)
		}
	}()
}

// HandleFunc registers an API endpoint. Safe to call from any module's Init().
// Paths may start with "/" or not — both forms are accepted.
func HandleFunc(path string, handler http.HandlerFunc) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	mu.Lock()
	defer mu.Unlock()

	if paths[path] {
		// second registration would panic inside ServeMux — be explicit instead
		if log != nil {
			log.Warn("duplicate handler", "path", path)
		}
		return
	}
	paths[path] = true
	mux.HandleFunc(path, handler)
}

// internals

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if log != nil {
			log.Debug("req", "m", r.Method, "p", r.URL.Path, "ip", clientIP(r))
		}
	})
}

func clientIP(r *http.Request) string {
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}

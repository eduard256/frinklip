package upload

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/eduard256/frinklip/internal/api"
	"github.com/eduard256/frinklip/pkg/app"
	"github.com/eduard256/frinklip/pkg/dropstore"
	"github.com/eduard256/frinklip/pkg/promptgen"
)

var (
	log   *slog.Logger
	store *dropstore.Store

	histMu sync.Mutex
	// session history — newest first, capped at histMax
	history []HistoryItem
)

const histMax = 50

type HistoryItem struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsImage bool   `json:"is_image"`
	Line    string `json:"line"`
	TS      int64  `json:"ts"`
}

// Init registers POST /api/upload and GET /api/history
func Init() {
	var conf struct {
		Mod struct {
			Dir string `yaml:"dir"`
		} `yaml:"upload"`
	}

	conf.Mod.Dir = "/tmp/dropped"

	app.LoadConfig(&conf)
	log = app.GetLogger("upload")

	s, err := dropstore.New(conf.Mod.Dir)
	if err != nil {
		log.Error("init store", "dir", conf.Mod.Dir, "err", err)
		return
	}
	store = s
	log.Info("ready", "dir", conf.Mod.Dir)

	api.HandleFunc("/api/upload", handleUpload)
	api.HandleFunc("/api/history", handleHistory)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// streaming multipart — no memory caps, no temp files before we open dest
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "multipart: "+err.Error(), http.StatusBadRequest)
		return
	}

	var items []HistoryItem
	var paths []string

	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		if part.FormName() != "files" {
			_ = part.Close()
			continue
		}

		name := part.FileName()
		path, err := store.Save(name, part)
		_ = part.Close()
		if err != nil {
			log.Error("save", "name", name, "err", err)
			http.Error(w, "save: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// size is not provided by multipart; stat it
		size := fileSize(path)

		item := HistoryItem{
			Path:    path,
			Name:    name,
			Size:    size,
			IsImage: promptgen.IsImage(path),
			Line:    promptgen.Line(path),
			TS:      nowUnix(),
		}
		items = append(items, item)
		paths = append(paths, path)
	}

	if len(items) == 0 {
		http.Error(w, "no files", http.StatusBadRequest)
		return
	}

	pushHistory(items)

	resp := struct {
		Files  []HistoryItem `json:"files"`
		Prompt string        `json:"prompt"`
	}{
		Files:  items,
		Prompt: promptgen.Build(paths),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	histMu.Lock()
	cp := make([]HistoryItem, len(history))
	copy(cp, history)
	histMu.Unlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(cp)
}

// internals

func pushHistory(items []HistoryItem) {
	histMu.Lock()
	// prepend newest first
	history = append(items, history...)
	if len(history) > histMax {
		history = history[:histMax]
	}
	histMu.Unlock()
}

package app

import (
	"log/slog"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	configPath string
	configData []byte
	configOnce sync.Once
)

// SetConfigPath must be called before LoadConfig; empty path means "no file, use defaults"
func SetConfigPath(path string) {
	configPath = path
}

// LoadConfig unmarshals the single yaml config into v (usually an anonymous struct).
// Safe to call from any Init() — file is read only once per process.
func LoadConfig(v any) {
	configOnce.Do(func() {
		if configPath == "" {
			return
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("config: read", "path", configPath, "err", err)
			}
			return
		}
		configData = data
	})

	if len(configData) == 0 {
		return
	}
	if err := yaml.Unmarshal(configData, v); err != nil {
		slog.Warn("config: unmarshal", "err", err)
	}
}

// GetLogger returns a module-tagged logger
func GetLogger(module string) *slog.Logger {
	return slog.Default().With("mod", module)
}

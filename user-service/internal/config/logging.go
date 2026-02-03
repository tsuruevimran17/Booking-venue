package config

import (
	"log/slog"
	"os"
)

// EnsureLogDir создает директорию для логов, если её нет.
// Читает путь из переменной окружения LOG_DIR, по умолчанию "logs".
func EnsureLogDir() string {
	dir := os.Getenv("LOG_DIR")
	if dir == "" {
		dir = "logs"
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("failed to create log dir", "dir", dir, "error", err)
	}
	return dir
}

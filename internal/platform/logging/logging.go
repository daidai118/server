package logging

import (
	"log/slog"
	"os"
)

func New(service, environment string) *slog.Logger {
	level := slog.LevelInfo
	if environment == "dev" || environment == "development" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})).With(
		"service", service,
		"env", environment,
	)
}

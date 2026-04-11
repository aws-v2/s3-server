package logging

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func InitLogger(profile string) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	// Use JSON handler for all environments for consistency in microservices
	handler := slog.NewJSONHandler(os.Stdout, opts)
	
	Logger = slog.New(handler).With(
		slog.String("profile", profile),
		slog.String("service", "s3-server"),
	)

	slog.SetDefault(Logger)
}

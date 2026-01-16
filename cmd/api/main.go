package main

import (
	"context"
	"log/slog"
	"os"

	"bitbucket.org/appmax-space/go-boilerplate/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	if err := Start(context.Background(), cfg); err != nil {
		slog.Error("application failed to start", "error", err)
		os.Exit(1)
	}
}

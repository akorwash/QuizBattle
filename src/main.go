package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/akorwash/QuizBattle/api"
	"github.com/akorwash/QuizBattle/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("Quiz Battle stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	application, err := api.New(cfg)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("Quiz Battle server starting", "port", cfg.Port, "environment", cfg.Environment)
	runError := application.Run(shutdownContext)
	closeContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	closeError := application.Close(closeContext)
	if runError != nil || closeError != nil {
		return fmt.Errorf("run error: %v; close error: %v", runError, closeError)
	}
	return nil
}

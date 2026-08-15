package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/akorwash/QuizBattle/api"
	"github.com/akorwash/QuizBattle/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(os.Args[1:]); err != nil {
		slog.Error("Quiz Battle stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		if len(args) == 1 && args[0] == "healthcheck" {
			return runHealthcheck(os.Getenv("PORT"), strings.TrimSpace(os.Getenv("RELEASE_SHA")))
		}
		return fmt.Errorf("unknown command %q", strings.Join(args, " "))
	}

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

	slog.Info("Quiz Battle server starting", "port", cfg.Port, "environment", cfg.Environment, "release", cfg.ReleaseSHA)
	runError := application.Run(shutdownContext)
	closeContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	closeError := application.Close(closeContext)
	if runError != nil || closeError != nil {
		return fmt.Errorf("run error: %v; close error: %v", runError, closeError)
	}
	return nil
}

func runHealthcheck(rawPort, expectedRelease string) error {
	port := strings.TrimSpace(rawPort)
	if port == "" {
		port = "8080"
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return fmt.Errorf("invalid PORT for healthcheck")
	}

	dialer := &net.Dialer{Timeout: 2 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("healthcheck endpoint redirected")
		},
	}
	response, err := client.Get("http://quizbattle-health.local/healthz")
	if err != nil {
		return fmt.Errorf("request health endpoint: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	var payload struct {
		Status  string `json:"status"`
		Release string `json:"release"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(nil, response.Body, 4<<10))
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("decode health response: %w", err)
	}
	if payload.Status != "ok" {
		return fmt.Errorf("health endpoint returned unexpected status %q", payload.Status)
	}
	if expectedRelease != "" && payload.Release != expectedRelease {
		return fmt.Errorf("health endpoint release mismatch")
	}
	return nil
}

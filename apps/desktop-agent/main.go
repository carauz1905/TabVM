package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/tabvm/desktop-agent/internal/config"
	"github.com/tabvm/desktop-agent/internal/runner"
	"github.com/tabvm/desktop-agent/internal/server"
	"github.com/tabvm/desktop-agent/internal/store"
	"github.com/tabvm/desktop-agent/internal/vbox"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// In a packaged (non-development) install no session token is provided via
	// environment, so generate and persist a per-machine token. The agent serves
	// the embedded UI with this token injected, so the browser authenticates
	// without any build-time secret. Development keeps its per-run fallback.
	if cfg.SessionToken == "" && !cfg.IsDevelopment() {
		token, tokenErr := ensureSessionToken(cfg.DataDir)
		if tokenErr != nil {
			logger.Error("failed to establish a session token", "error", tokenErr)
			os.Exit(1)
		}
		cfg.SessionToken = token
	}

	db, err := store.Open(context.Background(), store.Config{
		DBPath:  cfg.DBPath,
		DataDir: cfg.DataDir,
	})
	if err != nil {
		logger.Error("failed to open local state database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	r := runner.NewRunner()
	vboxService := vbox.NewService(r, vbox.Config{
		CandidatePaths: cfg.VBoxManagePaths,
		Store:          db,
	})
	srv := server.New(cfg, vboxService, db, logger)

	// Serve in the background so the tray can own the main thread's message
	// loop. On Windows the tray provides Open/Quit; choosing Quit returns from
	// runTray and we exit cleanly. Without a tray (other platforms) we block so
	// the agent keeps serving.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("agent stopped", "error", err)
			os.Exit(1)
		}
	}()

	if runTray(logger) {
		os.Exit(0)
	}
	select {}
}

// ensureSessionToken returns a stable per-machine session token, reading it from
// (or creating it in) a token file. dataDir is used when set; otherwise the
// per-user config directory (%APPDATA%\TabVM) is used. The file is written with
// owner-only permissions.
func ensureSessionToken(dataDir string) (string, error) {
	dir := dataDir
	if strings.TrimSpace(dir) == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "TabVM")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	tokenPath := filepath.Join(dir, "session.token")
	if existing, err := os.ReadFile(tokenPath); err == nil {
		if token := strings.TrimSpace(string(existing)); token != "" {
			return token, nil
		}
	}

	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		return "", err
	}
	return token, nil
}

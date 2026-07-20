package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
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

	// Under the windowsgui launcher there is no console, so slog output written
	// only to stdout is discarded and diagnostics become unrecoverable. Also
	// write to a discoverable log file. If the file cannot be opened we keep
	// serving with stdout only rather than crash the agent over logging.
	if writer, closeLog, logErr := openLogWriter(cfg.DataDir); logErr != nil {
		logger.Warn("could not open agent log file; continuing with stdout only", "error", logErr)
	} else {
		defer func() { _ = closeLog() }()
		logger = slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
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

// openLogWriter opens (creating as needed) <dataDir>/logs/agent.log for
// appending and returns a writer that fans out to both stdout and that file,
// plus a close function for the file. dataDir mirrors the session-token
// directory: it is used when set, otherwise the per-user config directory
// (%APPDATA%\TabVM) is used. The logs directory is owner-only.
//
// rotation is a follow-up.
func openLogWriter(dataDir string) (io.Writer, func() error, error) {
	dir := strings.TrimSpace(dataDir)
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, nil, err
		}
		dir = filepath.Join(base, "TabVM")
	}

	logsDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		return nil, nil, err
	}

	logPath := filepath.Join(logsDir, "agent.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}

	return io.MultiWriter(os.Stdout, file), file.Close, nil
}

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	commandcenter "github.com/rathix/command-center"
	"github.com/rathix/command-center/internal/certs"
	appconfig "github.com/rathix/command-center/internal/config"
	"github.com/rathix/command-center/internal/health"
	"github.com/rathix/command-center/internal/history"
	"github.com/rathix/command-center/internal/k8s"
	"github.com/rathix/command-center/internal/server"
	"github.com/rathix/command-center/internal/session"
	"github.com/rathix/command-center/internal/sse"
	"github.com/rathix/command-center/internal/state"
)

const defaultAddr = ":8443"

// Version is injected at build time using ldflags.
var Version = "(unknown)"

// config holds all server configuration.
type config struct {
	Dev             bool
	ShowVersion     bool
	ListenAddr      string
	Kubeconfig      string
	HealthInterval  time.Duration
	SessionDuration time.Duration
	DataDir         string
	HistoryFile     string
	LogFormat       string
	TLSCACert       string
	TLSCert         string
	TLSKey          string
	ConfigFile      string
}

func main() {
	// Quick check for version flag before full config loading
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" {
			fmt.Printf("Command Center version %s\n", Version)
			return
		}
	}

	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// loadConfig parses flags and environment variables with precedence: Flag > Env > Default.
func loadConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("command-center", flag.ContinueOnError)

	cfg := config{}
	fs.BoolVar(&cfg.Dev, "dev", getEnvBool("DEV", false), "proxy frontend requests to Vite dev server")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")
	fs.StringVar(&cfg.ListenAddr, "listen-addr", getEnv("LISTEN_ADDR", defaultAddr), "listen address")
	fs.StringVar(&cfg.Kubeconfig, "kubeconfig", getEnv("KUBECONFIG", defaultKubeconfig()), "path to kubeconfig file")

	healthIntervalStr := getEnv("HEALTH_INTERVAL", "30s")
	fs.StringVar(&healthIntervalStr, "health-interval", healthIntervalStr, "health check interval")

	sessionDurationStr := getEnv("SESSION_DURATION", "24h")
	fs.StringVar(&sessionDurationStr, "session-duration", sessionDurationStr, "session cookie duration")

	fs.StringVar(&cfg.DataDir, "data-dir", getEnv("DATA_DIR", "/data"), "data directory for certificates")
	fs.StringVar(&cfg.LogFormat, "log-format", getEnv("LOG_FORMAT", "json"), "log format (json or text)")
	fs.StringVar(&cfg.TLSCACert, "tls-ca-cert", getEnv("TLS_CA_CERT", ""), "custom CA certificate path")
	fs.StringVar(&cfg.TLSCert, "tls-cert", getEnv("TLS_CERT", ""), "custom server certificate path")
	fs.StringVar(&cfg.TLSKey, "tls-key", getEnv("TLS_KEY", ""), "custom server key path")
	fs.StringVar(&cfg.ConfigFile, "config", getEnv("CONFIG_FILE", ""), "path to YAML config file for custom services")
	fs.StringVar(&cfg.HistoryFile, "history-file", getEnv("HISTORY_FILE", ""), "path to history JSONL file")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	if cfg.HistoryFile == "" {
		cfg.HistoryFile = filepath.Join(cfg.DataDir, "history.jsonl")
	}

	interval, err := time.ParseDuration(healthIntervalStr)
	if err != nil {
		return config{}, fmt.Errorf("invalid health interval %q: %w", healthIntervalStr, err)
	}
	if interval < time.Second {
		return config{}, fmt.Errorf("health interval must be at least 1s, got %q", healthIntervalStr)
	}
	cfg.HealthInterval = interval

	sessionDuration, err := time.ParseDuration(sessionDurationStr)
	if err != nil {
		return config{}, fmt.Errorf("invalid session duration %q: %w", sessionDurationStr, err)
	}
	if sessionDuration <= 0 {
		return config{}, fmt.Errorf("session duration must be positive, got %q", sessionDurationStr)
	}
	if sessionDuration < time.Minute {
		return config{}, fmt.Errorf("session duration must be at least 1m, got %q", sessionDurationStr)
	}
	if sessionDuration > 720*time.Hour {
		return config{}, fmt.Errorf("session duration must be at most 720h (30 days), got %q", sessionDurationStr)
	}
	cfg.SessionDuration = sessionDuration

	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		return config{}, fmt.Errorf("unsupported log format %q: must be \"json\" or \"text\"", cfg.LogFormat)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fallback
		}
		return b
	}
	return fallback
}

func defaultKubeconfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.kube/config"
	}
	return filepath.Join(home, ".kube", "config")
}

func setupLogger(format string) *slog.Logger {
	return setupLoggerWithWriter(format, os.Stdout)
}

func setupLoggerWithWriter(format string, writer io.Writer) *slog.Logger {
	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(writer, nil)
	} else {
		handler = slog.NewJSONHandler(writer, nil)
	}
	return slog.New(handler)
}

// run starts the server and handles graceful shutdown.
func run(ctx context.Context, cfg config) error {
	logger := setupLogger(cfg.LogFormat)
	slog.SetDefault(logger)

	slog.Info("Starting Command Center", "version", Version)

	store := state.NewStore()

	// Load optional YAML config for custom services and overrides
	var lastAppCfg *appconfig.Config
	if cfg.ConfigFile != "" {
		appCfg, configErrs := appconfig.Load(cfg.ConfigFile)
		if appCfg != nil {
			slog.Info("Config loaded",
				"services", len(appCfg.Services),
				"overrides", len(appCfg.Overrides),
				"groups", len(appCfg.Groups),
			)
		}
		storeConfigErrors(store, configErrs)
		for _, e := range configErrs {
			if appCfg == nil {
				slog.Error("Config parse failed, continuing without custom services", "error", e)
			} else {
				slog.Warn("Config validation warning", "error", e)
			}
		}
		lastAppCfg = appCfg
	}

	// Start Kubernetes Ingress watcher
	watcherCtx, watcherCancel := context.WithCancel(ctx)
	defer watcherCancel()

	var watcher *k8s.Watcher
	watcher, err := k8s.NewWatcher(cfg.Kubeconfig, store, logger)
	if err != nil {
		slog.Warn("k8s watcher disabled: failed to create watcher", "error", err)
	} else {
		go watcher.Run(watcherCtx)
	}

	// Register config services and apply overrides after K8s watcher starts
	if lastAppCfg != nil {
		appconfig.RegisterServices(store, lastAppCfg)
		slog.Info("Config services registered", "count", len(lastAppCfg.Services))

		// Wait for initial K8s informer sync so overrides can be applied to discovered services.
		if watcher != nil {
			waitCtx, cancelWait := context.WithTimeout(watcherCtx, 5*time.Second)
			if !watcher.WaitForSync(waitCtx) {
				slog.Warn("timed out waiting for k8s sync before applying config overrides")
			}
			cancelWait()
		}

		appconfig.ApplyOverrides(store, lastAppCfg)
		slog.Info("Config overrides applied", "count", len(lastAppCfg.Overrides))
	}

	// Initialize history persistence
	historyWriter, err := history.NewFileWriter(cfg.HistoryFile, logger)
	if err != nil {
		return fmt.Errorf("failed to create history writer: %w", err)
	}
	defer historyWriter.Close()

	records, err := history.ReadHistory(cfg.HistoryFile)
	if err != nil {
		slog.Warn("failed to read history", "error", err)
		records = make(map[string]history.TransitionRecord)
	}
	pendingHistory := history.RestoreHistory(store, records, logger)

	// Apply pending history for late-arriving services
	pendingSub := store.Subscribe()
	go func() {
		defer store.Unsubscribe(pendingSub)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-pendingSub:
				if !ok {
					return
				}
				if event.Type == state.EventDiscovered {
					pendingHistory.ApplyIfPending(store, event.Service.Namespace, event.Service.Name)
				}
			}
		}
	}()

	// Start config file watcher for hot-reload
	if cfg.ConfigFile != "" {
		configWatcher := appconfig.NewWatcher(cfg.ConfigFile, func(newCfg *appconfig.Config, errs []error) {
			if newCfg != nil {
				slog.Info("Config reloaded",
					"services", len(newCfg.Services),
					"overrides", len(newCfg.Overrides),
					"groups", len(newCfg.Groups),
				)
			}
			storeConfigErrors(store, errs)
			for _, e := range errs {
				if newCfg == nil {
					slog.Error("Config reload parse failed", "error", e)
				} else {
					slog.Warn("Config reload validation warning", "error", e)
				}
			}
			if newCfg == nil {
				// Keep the last-known-good config active when reload parsing fails.
				return
			}
			added, removed, updated := appconfig.ReconcileOnReload(store, lastAppCfg, newCfg)
			if added > 0 || removed > 0 || updated > 0 {
				slog.Info("Config reconciled", "added", added, "removed", removed, "updated", updated)
			}
			lastAppCfg = newCfg
		}, logger)
		go func() {
			if err := configWatcher.Run(watcherCtx); err != nil && watcherCtx.Err() == nil {
				slog.Warn("config watcher stopped with error", "error", err)
			}
		}()
	}

	// Create and start SSE broker for real-time event streaming
	broker := sse.NewBroker(store, logger, Version, cfg.HealthInterval)
	go broker.Run(ctx)

	// Create and start HTTP health checker
	probeClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	checker := health.NewChecker(store, store, probeClient, cfg.HealthInterval, historyWriter, logger)
	go checker.Run(ctx)

	retentionDays := 30
	if lastAppCfg != nil && lastAppCfg.History.RetentionDays > 0 {
		retentionDays = lastAppCfg.History.RetentionDays
	}
	pruner := history.NewPruner(cfg.HistoryFile, retentionDays, logger)
	go pruner.Run(ctx)

	mux := http.NewServeMux()

	// Register SSE endpoint before the catch-all static handler
	mux.Handle("GET /api/events", broker)

	if cfg.Dev {
		viteURL := "http://localhost:5173"
		proxy, err := server.NewDevProxyHandler(viteURL)
		if err != nil {
			return fmt.Errorf("failed to create dev proxy: %w", err)
		}
		mux.Handle("/", proxy)
		slog.Info("Dev mode: proxying to Vite", "url", viteURL)
	} else {
		spaHandler, err := server.NewSPAHandler(commandcenter.WebFS, "web/build")
		if err != nil {
			return fmt.Errorf("failed to create SPA handler: %w", err)
		}
		mux.Handle("/", spaHandler)
	}

	var handler http.Handler = mux
	var tlsConfig *tls.Config

	// Configure TLS/mTLS for non-dev mode
	if !cfg.Dev {
		certsCfg := certs.CertsConfig{
			DataDir:      cfg.DataDir,
			CustomCACert: cfg.TLSCACert,
			CustomCert:   cfg.TLSCert,
			CustomKey:    cfg.TLSKey,
		}

		assets, err := certs.LoadOrGenerateCerts(certsCfg)
		if err != nil {
			return fmt.Errorf("failed to load certificates: %w", err)
		}

		if assets.WasGenerated {
			if assets.GenerationReason == "leaf-renewal" {
				slog.Warn("Existing TLS leaf certificates expired — renewed server/client certificates")
			} else {
				slog.Info("No usable TLS certificates found — generating self-signed CA")
			}
			slog.Info("Certificates ready",
				"ca", assets.CACertPath,
				"server", assets.ServerCertPath,
				"server_key", assets.ServerKeyPath,
				"client", assets.ClientCertPath,
				"client_key", assets.ClientKeyPath)
			slog.Info("Install client.crt + client.key in your browser to access Command Center")
		} else if certsCfg.CustomCACert != "" {
			slog.Info("Using custom TLS certificates",
				"ca", assets.CACertPath,
				"server", assets.ServerCertPath,
				"server_key", assets.ServerKeyPath)
		} else {
			slog.Info("Using existing certificates",
				"ca", assets.CACertPath,
				"server", assets.ServerCertPath,
				"server_key", assets.ServerKeyPath,
				"client", assets.ClientCertPath,
				"client_key", assets.ClientKeyPath)
			slog.Info("Install client.crt + client.key in your browser to access Command Center")
		}

		tlsConfig, err = certs.NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
		if err != nil {
			return fmt.Errorf("failed to create TLS config: %w", err)
		}

		// Generate HMAC secret and wrap mux with session middleware
		secret, err := session.GenerateSecret()
		if err != nil {
			return fmt.Errorf("failed to generate session secret: %w", err)
		}
		sessionCfg := session.Config{
			Secret:   secret,
			Duration: cfg.SessionDuration,
			Secure:   true,
		}
		handler = session.Middleware(mux, sessionCfg)
		slog.Info("Session authentication enabled", "duration", cfg.SessionDuration)
	}

	srv := &http.Server{
		Addr:      cfg.ListenAddr,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	// Channel to catch server errors
	serverError := make(chan error, 1)

	go func() {
		var err error
		if cfg.Dev {
			slog.Info("Listening (HTTP)", "addr", cfg.ListenAddr)
			err = srv.ListenAndServe()
		} else {
			slog.Info("Listening (HTTPS+mTLS)", "addr", cfg.ListenAddr)
			err = srv.ListenAndServeTLS("", "")
		}
		if err != nil && err != http.ErrServerClosed {
			serverError <- err
		}
	}()

	// Wait for interruption or server error
	select {
	case <-ctx.Done():
		slog.Info("Shutting down gracefully...")
		// Stop watcher before server shutdown
		watcherCancel()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server forced to shutdown: %w", err)
		}
		slog.Info("Connections drained")
		slog.Info("Server stopped")
	case err := <-serverError:
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// storeConfigErrors converts config error slice to string slice and stores in state.
func storeConfigErrors(store *state.Store, errs []error) {
	strs := make([]string, 0, len(errs))
	for _, e := range errs {
		strs = append(strs, e.Error())
	}
	store.SetConfigErrors(strs)
}

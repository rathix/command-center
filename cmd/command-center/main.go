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
	"github.com/rathix/command-center/internal/health"
	"github.com/rathix/command-center/internal/k8s"
	"github.com/rathix/command-center/internal/server"
	"github.com/rathix/command-center/internal/sse"
	"github.com/rathix/command-center/internal/state"
)

const defaultAddr = ":8443"

// config holds all server configuration.
type config struct {
	Dev            bool
	ListenAddr     string
	Kubeconfig     string
	HealthInterval time.Duration
	DataDir        string
	LogFormat      string
	TLSCACert      string
	TLSCert        string
	TLSKey         string
}

func main() {
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
	fs.StringVar(&cfg.ListenAddr, "listen-addr", getEnv("LISTEN_ADDR", defaultAddr), "listen address")
	fs.StringVar(&cfg.Kubeconfig, "kubeconfig", getEnv("KUBECONFIG", defaultKubeconfig()), "path to kubeconfig file")

	healthIntervalStr := getEnv("HEALTH_INTERVAL", "30s")
	fs.StringVar(&healthIntervalStr, "health-interval", healthIntervalStr, "health check interval")

	fs.StringVar(&cfg.DataDir, "data-dir", getEnv("DATA_DIR", "/data"), "data directory for certificates")
	fs.StringVar(&cfg.LogFormat, "log-format", getEnv("LOG_FORMAT", "json"), "log format (json or text)")
	fs.StringVar(&cfg.TLSCACert, "tls-ca-cert", getEnv("TLS_CA_CERT", ""), "custom CA certificate path")
	fs.StringVar(&cfg.TLSCert, "tls-cert", getEnv("TLS_CERT", ""), "custom server certificate path")
	fs.StringVar(&cfg.TLSKey, "tls-key", getEnv("TLS_KEY", ""), "custom server key path")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	interval, err := time.ParseDuration(healthIntervalStr)
	if err != nil {
		return config{}, fmt.Errorf("invalid health interval %q: %w", healthIntervalStr, err)
	}
	if interval <= 0 {
		return config{}, fmt.Errorf("health interval must be greater than zero, got %q", healthIntervalStr)
	}
	cfg.HealthInterval = interval

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

	// Create in-memory service state store
	store := state.NewStore()

	// Start Kubernetes Ingress watcher
	watcherCtx, watcherCancel := context.WithCancel(ctx)
	defer watcherCancel()

	watcher, err := k8s.NewWatcher(cfg.Kubeconfig, store, logger)
	if err != nil {
		slog.Warn("k8s watcher disabled: failed to create watcher", "error", err)
	} else {
		go watcher.Run(watcherCtx)
	}

	// Create and start SSE broker for real-time event streaming
	broker := sse.NewBroker(store, logger)
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
	checker := health.NewChecker(store, store, probeClient, cfg.HealthInterval, logger)
	go checker.Run(ctx)

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
		handler, err := server.NewSPAHandler(commandcenter.WebFS, "web/build")
		if err != nil {
			return fmt.Errorf("failed to create SPA handler: %w", err)
		}
		mux.Handle("/", handler)
	}

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

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

		tlsConfig, err := certs.NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
		if err != nil {
			return fmt.Errorf("failed to create TLS config: %w", err)
		}
		srv.TLSConfig = tlsConfig
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

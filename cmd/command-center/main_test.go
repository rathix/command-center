package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigPrecedence(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		envs     map[string]string
		expected string
	}{
		{
			name:     "default value",
			args:     []string{},
			envs:     map[string]string{},
			expected: ":8443",
		},
		{
			name:     "env var precedence",
			args:     []string{},
			envs:     map[string]string{"LISTEN_ADDR": ":9443"},
			expected: ":9443",
		},
		{
			name:     "flag precedence over env",
			args:     []string{"--listen-addr", ":9999"},
			envs:     map[string]string{"LISTEN_ADDR": ":9443"},
			expected: ":9999",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(tc.name, flag.ExitOnError)
			for k, v := range tc.envs {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			cfg := LoadConfig(tc.args)
			assert.Equal(t, tc.expected, cfg.ListenAddr)
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := make(chan os.Signal, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		stop <- syscall.SIGTERM
	}()
	select {
	case <-stop:
		cancel()
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for signal")
	}
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestLogFormatSelection(t *testing.T) {
	cases := []struct {
		format string
		isJSON bool
	}{
		{"json", true},
		{"text", false},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			handler := setupLogger(tc.format)
			_, ok := handler.Handler().(*slog.JSONHandler)
			assert.Equal(t, tc.isJSON, ok)
		})
	}
}

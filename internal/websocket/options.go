package websocket

import (
	"log/slog"
	"time"
)

// DefaultPingInterval is the default interval between server-sent pings.
const DefaultPingInterval = 5 * time.Second

// DefaultPongTimeout is the maximum time to wait for a pong reply.
const DefaultPongTimeout = 10 * time.Second

// Options configures a wrapped WebSocket connection.
type Options struct {
	PingInterval time.Duration
	PongTimeout  time.Duration
	Logger       *slog.Logger
}

// Option is a functional option for configuring a WebSocket connection.
type Option func(*Options)

// WithPingInterval sets the interval between server-sent pings.
func WithPingInterval(d time.Duration) Option {
	return func(o *Options) { o.PingInterval = d }
}

// WithPongTimeout sets the maximum time to wait for a pong reply.
func WithPongTimeout(d time.Duration) Option {
	return func(o *Options) { o.PongTimeout = d }
}

// WithLogger sets the logger for the connection.
func WithLogger(l *slog.Logger) Option {
	return func(o *Options) { o.Logger = l }
}

func defaultOptions() Options {
	return Options{
		PingInterval: DefaultPingInterval,
		PongTimeout:  DefaultPongTimeout,
		Logger:       slog.Default(),
	}
}

func applyOptions(opts []Option) Options {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

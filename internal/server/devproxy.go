package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewDevProxyHandler creates a reverse proxy handler that forwards all requests
// to the Vite dev server, enabling HMR and live reloading during development.
func NewDevProxyHandler(target string) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(u), nil
}

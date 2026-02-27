package server

import "net/http"

// ProxyHeaderMiddleware reads X-Forwarded-For and X-Forwarded-Proto headers
// set by reverse proxies and updates the request context accordingly.
func ProxyHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			r.RemoteAddr = xff
		}
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			r.URL.Scheme = proto
		}
		next.ServeHTTP(w, r)
	})
}

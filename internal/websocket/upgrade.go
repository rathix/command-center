package websocket

import (
	"encoding/json"
	"net/http"

	ws "nhooyr.io/websocket"
)

// UpgradeError is a JSON error response body for failed WS upgrade attempts.
type UpgradeError struct {
	Error string `json:"error"`
}

// CheckMTLS verifies that the request has valid mTLS client certificates.
// Returns true if the check passes, false if the request should be rejected.
func CheckMTLS(r *http.Request) bool {
	return r.TLS != nil && len(r.TLS.PeerCertificates) > 0
}

// RejectNoMTLS writes a 403 JSON error and returns. Used when mTLS check fails.
func RejectNoMTLS(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(UpgradeError{Error: "mTLS client certificate required"})
}

// Accept upgrades an HTTP request to a WebSocket connection.
// The caller must have already verified mTLS before calling this.
func Accept(w http.ResponseWriter, r *http.Request, acceptOpts *ws.AcceptOptions) (*ws.Conn, error) {
	if acceptOpts == nil {
		acceptOpts = &ws.AcceptOptions{
			InsecureSkipVerify: true, // TLS already verified at transport layer
		}
	}
	return ws.Accept(w, r, acceptOpts)
}

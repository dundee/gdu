package webui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
)

// actionHeader is the request header the frontend echoes ui.actionToken back
// on for every mutating request (delete, reveal).
const actionHeader = "X-GDU-Action"

// actionTokenParam is the URL query parameter StartUILoop appends the action
// token to (see server.go) so the frontend can pick it up client-side.
const actionTokenParam = "token"

// actionTokenSize is the amount of entropy (in bytes) behind the per-process
// action token.
const actionTokenSize = 32

// generateActionToken returns a cryptographically random, hex-encoded token
// unique to this server run. The frontend receives it once, from the URL
// StartUILoop prints and opens (see actionTokenParam), never over any API
// response: unlike the read-only endpoints, which stay intentionally
// unauthenticated, the token itself must not be readable by another local
// user who can reach the port but not this process's own terminal.
//
// rand.Read never returns an error: since Go 1.24 a broken OS entropy source
// is fatal to the process instead, so there is no error path to handle here.
func generateActionToken() string {
	b := make([]byte, actionTokenSize)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// actionURL is the URL StartUILoop prints and opens the browser to: the
// loopback address the server is listening on, carrying the action token as
// a query parameter (see actionTokenParam) so the frontend can pick it up
// client-side, and so this URL - not any API response - is the token's only
// delivery channel.
func actionURL(addr net.Addr, token string) string {
	return "http://" + addr.String() + "/?" + actionTokenParam + "=" + token
}

// validActionToken reports whether token matches this server's action token,
// using a constant-time comparison so response timing cannot be used to
// recover the token one byte at a time.
func (ui *UI) validActionToken(token string) bool {
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(ui.actionToken)) == 1
}

// sameOriginFetch reports whether r's Fetch Metadata / Origin headers are
// consistent with a request initiated by script running on this server's own
// page, rejecting cross-site calls that a malicious website or another
// browser tab could otherwise trigger against the loopback-bound server.
//
// Sec-Fetch-Site is sent by all current major browsers and cannot be set or
// overridden by page script, so it is authoritative when present. Origin is
// the fallback for the rare browser that omits Fetch Metadata; a request
// with neither header (e.g. a non-browser client) is left to the action
// token check alone.
func sameOriginFetch(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin"
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		return origin == originOf(r)
	}
	return true
}

// originOf returns the scheme://host origin r was received on.
func originOf(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

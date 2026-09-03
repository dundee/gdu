package webui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

// actionHeader is the request header the frontend echoes ui.actionToken back
// on for every mutating request (delete, reveal).
const actionHeader = "X-GDU-Action"

// actionTokenSize is the amount of entropy (in bytes) behind the per-process
// action token.
const actionTokenSize = 32

// generateActionToken returns a cryptographically random, hex-encoded token
// unique to this server run. The frontend receives it once, over the status
// endpoint/SSE stream (see statusJSONForClient), and echoes it back on every
// mutating request. A page on another origin cannot read that value: the
// browser blocks cross-origin scripts from reading the response body of a
// same-origin-only endpoint (no Access-Control-Allow-Origin is ever sent),
// so it has no way to learn the token even though it can still cause the
// browser to *send* a request.
func generateActionToken() string {
	b := make([]byte, actionTokenSize)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand only fails if the OS entropy source is broken, which
		// makes every other security property of the process suspect too.
		panic("webui: failed to generate action token: " + err.Error())
	}
	return hex.EncodeToString(b)
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

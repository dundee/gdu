package webui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// StartUILoop binds the listener, prints the URL, optionally opens the browser,
// and serves the SPA and API until interrupted. It blocks, mirroring the other
// frontends' StartUILoop contract.
func (ui *UI) StartUILoop() error {
	listener, err := net.Listen("tcp", ui.listenAddr)
	if err != nil {
		return fmt.Errorf("binding %s: %w", ui.listenAddr, err)
	}

	url := "http://" + listener.Addr().String()
	fmt.Fprintf(ui.output, "Gdu web UI running at %s\n", url)
	warnIfRemote(ui.output, listener.Addr())

	if ui.openBrowser {
		if err := openBrowser(url, ui.browserCmd); err != nil {
			log.Printf("webui: could not open browser: %s", err)
			fmt.Fprintf(ui.output, "Open %s in your browser.\n", url)
		}
	}

	srv := &http.Server{
		Handler:           ui.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	shutdownDone := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("webui: shutdown: %s", err)
		}
		close(shutdownDone)
	}()

	err = srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return err
}

// routes builds a dedicated mux (never the shared http.DefaultServeMux, which
// is deliberately reset elsewhere to keep pprof handlers isolated).
func (ui *UI) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", ui.handleStatus)
	mux.HandleFunc("GET /api/v1/nodes", ui.handleNodes)
	mux.HandleFunc("GET /api/v1/tree", ui.handleTree)
	mux.HandleFunc("DELETE /api/v1/nodes", ui.requireLocalAction(ui.handleDeleteNode))
	mux.HandleFunc("POST /api/v1/reveal", ui.requireLocalAction(ui.handleReveal))
	mux.HandleFunc("/api/v1/devices", ui.handleDevices)
	mux.HandleFunc("/api/v1/events", ui.handleEvents)
	mux.Handle("/", staticHandler())
	return mux
}

// warnIfRemote prints a security warning when the server is not bound to a
// loopback address, since directory names and sizes can be sensitive.
func warnIfRemote(w io.Writer, addr net.Addr) {
	if isLoopbackHost(addr.String()) {
		return
	}
	fmt.Fprintln(w, "WARNING: the web UI is reachable from other hosts on the network.")
	fmt.Fprintln(w, "         It exposes file names and sizes with no authentication.")
}

// isLoopbackHost reports whether the host component of a "host:port" string
// (or a bare host) is a loopback address, including the "localhost" name.
func isLoopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = strings.Trim(hostport, "[]")
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// isLocalRequest reports whether r both originates from a loopback address
// and targets a loopback Host header. Checking RemoteAddr alone is not
// enough: a page on an attacker-controlled domain that DNS-rebinds to
// 127.0.0.1 also connects from loopback, but the browser still sends the
// attacker's hostname as the Host header, which this rejects.
func isLocalRequest(r *http.Request) bool {
	return isLoopbackHost(r.RemoteAddr) && isLoopbackHost(r.Host)
}

// requireLocalAction wraps a mutating handler so it only runs for requests
// that originate from loopback, carry Fetch Metadata/Origin headers
// consistent with this server's own page, and present this server's action
// token. Together these guard against CSRF-style calls from other pages, tabs,
// and hosts on the network.
func (ui *UI) requireLocalAction(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLocalRequest(r) || !sameOriginFetch(r) || !ui.validActionToken(r.Header.Get(actionHeader)) {
			writeError(w, http.StatusForbidden, "actions are only available from the local web UI")
			return
		}
		next(w, r)
	}
}

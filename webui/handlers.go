package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/pkg/remove"
)

// statusResponse describes the current scan state and display preferences.
type statusResponse struct {
	State            string       `json:"state"` // "scanning" | "done" | "error"
	Error            string       `json:"error,omitempty"`
	RootPath         string       `json:"rootPath"`
	Progress         progressJSON `json:"progress"`
	ShowApparentSize bool         `json:"showApparentSize"`
	ShowRelativeSize bool         `json:"showRelativeSize"`
	UseSIPrefix      bool         `json:"useSIPrefix"`
	DeleteAllowed    bool         `json:"deleteAllowed"`
}

type progressJSON struct {
	CurrentItem string `json:"currentItem"`
	ItemCount   int64  `json:"itemCount"`
	TotalUsage  int64  `json:"totalUsage"`
}

type deviceJSON struct {
	Name       string `json:"name"`
	MountPoint string `json:"mountPoint"`
	Fstype     string `json:"fstype"`
	Size       int64  `json:"size"`
	Free       int64  `json:"free"`
}

const (
	scanStateDone     = "done"
	scanStateScanning = "scanning"
	scanStateError    = "error"
)

func (ui *UI) buildStatus() statusResponse {
	ui.mu.RLock()
	defer ui.mu.RUnlock()

	state := scanStateDone
	switch {
	case ui.scanErr != nil:
		state = scanStateError
	case ui.scanning:
		state = scanStateScanning
	case ui.topDir == nil:
		state = scanStateScanning
	}

	resp := statusResponse{
		State:    state,
		RootPath: ui.topDirPath,
		Progress: progressJSON{
			CurrentItem: ui.progress.CurrentItemName,
			ItemCount:   ui.progress.ItemCount,
			TotalUsage:  ui.progress.TotalUsage,
		},
		ShowApparentSize: ui.ShowApparentSize,
		ShowRelativeSize: ui.ShowRelativeSize,
		UseSIPrefix:      ui.UseSIPrefix,
	}
	reason, _ := ui.deleteDisabledReason(ui.noDelete, ui.scanning)
	resp.DeleteAllowed = reason == ""
	if ui.scanErr != nil {
		resp.Error = ui.scanErr.Error()
	}
	return resp
}

func (ui *UI) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := ui.buildStatus()
	if !isLocalRequest(r) {
		resp.DeleteAllowed = false
	}
	writeJSON(w, http.StatusOK, resp)
}

func (ui *UI) handleNodes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")

	node, err := ui.findNode(path)
	if err != nil {
		writeNodeError(w, err)
		return
	}

	sortBy, order := parseSort(r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	writeJSON(w, http.StatusOK, buildNodeResponse(node, sortBy, order))
}

func (ui *UI) handleTree(w http.ResponseWriter, r *http.Request) {
	node, err := ui.findNode(r.URL.Query().Get("path"))
	if err != nil {
		writeNodeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buildTree(node))
}

// deleteDisabledReason reports why deletion is currently refused given the
// scan state, or "" (with an unused status) when it is allowed. Shared by
// buildStatus, which only needs the boolean, and handleDeleteNode, which also
// needs the message and HTTP status to report.
func (ui *UI) deleteDisabledReason(noDelete, scanning bool) (reason string, status int) {
	switch {
	case noDelete:
		return "deletion is disabled", http.StatusForbidden
	case ui.IsFilteringFiles() && os.Getenv("GDU_ALLOW_DELETE_WITH_FILTER") != "1":
		return "deletion is disabled while filters are active", http.StatusForbidden
	case scanning:
		return "deletion is unavailable while scanning", http.StatusConflict
	default:
		return "", 0
	}
}

func (ui *UI) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	ui.actionMu.Lock()
	defer ui.actionMu.Unlock()

	ui.mu.RLock()
	noDelete := ui.noDelete
	scanning := ui.scanning
	ui.mu.RUnlock()
	if reason, status := ui.deleteDisabledReason(noDelete, scanning); reason != "" {
		writeError(w, status, reason)
		return
	}

	node, err := ui.findNode(r.URL.Query().Get("path"))
	if err != nil {
		writeNodeError(w, err)
		return
	}
	parent := node.GetParent()
	if parent == nil {
		writeError(w, http.StatusBadRequest, "cannot delete the analysis root")
		return
	}
	if isArchiveDescendant(node) {
		writeError(w, http.StatusBadRequest, "cannot delete an entry nested inside a browsed archive")
		return
	}
	if err := remove.ItemFromDir(parent, node); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (ui *UI) handleReveal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	node, err := ui.findNode(body.Path)
	if err != nil {
		writeNodeError(w, err)
		return
	}
	target := node
	if !node.IsDir() {
		target = node.GetParent()
	}
	if target == nil {
		writeError(w, http.StatusBadRequest, "path has no directory to reveal")
		return
	}
	target = realPathAncestor(target)
	if isArchiveRoot(target) {
		// target is a browsed archive's own top-level node: its GetPath
		// names the archive file, so reveal its containing directory
		// instead of opening the archive itself.
		target = target.GetParent()
	}
	if err := ui.revealPath(target.GetPath()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeNodeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errOutsideRoot):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, errNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (ui *UI) handleDevices(w http.ResponseWriter, _ *http.Request) {
	ui.mu.RLock()
	devices := ui.devices
	ui.mu.RUnlock()

	out := make([]deviceJSON, 0, len(devices))
	for _, d := range devices {
		out = append(out, deviceJSON{
			Name:       d.Name,
			MountPoint: d.MountPoint,
			Fstype:     d.Fstype,
			Size:       d.Size,
			Free:       d.Free,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleEvents streams scan status updates using Server-Sent Events.
func (ui *UI) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	local := isLocalRequest(r)

	ch, last := ui.hub.subscribe()
	defer ui.hub.unsubscribe(ch)

	// send current state immediately on connect
	if last == "" {
		last = ui.statusJSON()
	}
	fmt.Fprintf(w, "data: %s\n\n", statusJSONForClient(last, local))
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, open := <-ch:
			if !open {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", statusJSONForClient(msg, local))
			flusher.Flush()
		}
	}
}

// statusJSONForClient adjusts a broadcast status payload for one SSE
// subscriber: the hub broadcasts a single shared message to every client, but
// deleteAllowed must read false for remote clients since requireLocalAction
// rejects their action requests regardless of what this message reports.
func statusJSONForClient(msg string, local bool) string {
	if local {
		return msg
	}
	var resp statusResponse
	if err := json.Unmarshal([]byte(msg), &resp); err != nil {
		return msg
	}
	resp.DeleteAllowed = false
	data, err := json.Marshal(resp)
	if err != nil {
		return msg
	}
	return string(data)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("webui: encoding response: %s", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

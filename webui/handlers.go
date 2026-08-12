package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func (ui *UI) buildStatus() statusResponse {
	ui.mu.RLock()
	defer ui.mu.RUnlock()

	state := "done"
	switch {
	case ui.scanErr != nil:
		state = "error"
	case ui.scanning:
		state = "scanning"
	case ui.topDir == nil:
		state = "scanning"
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
	if ui.scanErr != nil {
		resp.Error = ui.scanErr.Error()
	}
	return resp
}

func (ui *UI) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, ui.buildStatus())
}

func (ui *UI) handleNodes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")

	node, err := ui.findNode(path)
	if err != nil {
		switch err {
		case errOutsideRoot:
			writeError(w, http.StatusForbidden, err.Error())
		case errNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	sortBy, order := parseSort(r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	writeJSON(w, http.StatusOK, buildNodeResponse(node, sortBy, order))
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

	ch, last := ui.hub.subscribe()
	defer ui.hub.unsubscribe(ch)

	// send current state immediately on connect
	if last == "" {
		last = ui.statusJSON()
	}
	fmt.Fprintf(w, "data: %s\n\n", last)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, open := <-ch:
			if !open {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

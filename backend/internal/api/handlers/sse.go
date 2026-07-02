package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

// heartbeatInterval keeps idle SSE connections (and intermediaries) alive.
const heartbeatInterval = 25 * time.Second

// Events handles GET /api/sessions/:id/events — an SSE stream of the session's
// lifecycle. Ownership is enforced before any data is streamed.
func (h *SessionHandler) Events(c *gin.Context) {
	ident := middleware.GetIdentity(c)
	id := c.Param("id")

	current, err := h.svc.Get(c.Request.Context(), id, ident.Subject)
	if err != nil {
		respondLookupError(c, err)
		return
	}

	flusher, ok := sseFlusher(c)
	if !ok {
		return
	}
	writeSSEHeaders(c)

	writeSessionEvent(c, flusher, "update", current)

	w, err := h.svc.Watch(c.Request.Context(), current.Name)
	if err != nil {
		writeRawEvent(c, flusher, "error", `{"message":"watch failed"}`)
		return
	}
	defer w.Stop()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ssePing(c, flusher)
		case ev, open := <-w.ResultChan():
			if !open {
				return
			}
			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			switch ev.Type {
			case watch.Deleted:
				sess := h.svc.ToSession(obj)
				writeSessionEvent(c, flusher, "deleted", &sess)
				return
			case watch.Added, watch.Modified:
				sess := h.svc.ToSession(obj)
				writeSessionEvent(c, flusher, "update", &sess)
			}
		}
	}
}

// QueueEvents handles GET /api/queue/events — an SSE stream of the caller's
// warm-pool queue progress: "queued" position updates, then a terminal
// "assigned" or "error". If the caller isn't queued but already owns a session,
// that session is emitted immediately (covers the assignment-before-stream
// race).
func (h *SessionHandler) QueueEvents(c *gin.Context) {
	ident := middleware.GetIdentity(c)

	flusher, ok := sseFlusher(c)
	if !ok {
		return
	}

	var ch <-chan k8s.QueueEvent
	var unsub func()
	queued := false
	if h.queue != nil {
		ch, unsub, queued = h.queue.Subscribe(ident.Subject)
	}
	if !queued {
		sessions, err := h.svc.List(c.Request.Context(), ident.Subject)
		if err == nil && len(sessions) > 0 {
			writeSSEHeaders(c)
			writeQueueEvent(c, flusher, k8s.QueueEvent{Type: "assigned", Session: &sessions[0]})
			return
		}
		respondError(c, http.StatusNotFound, "not_queued", "you are not in the queue")
		return
	}
	defer unsub()

	writeSSEHeaders(c)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ssePing(c, flusher)
		case ev, open := <-ch:
			if !open {
				return
			}
			writeQueueEvent(c, flusher, ev)
			if ev.Type == "assigned" || ev.Type == "error" {
				return
			}
		}
	}
}

// sseFlusher returns the response flusher, writing a 500 if streaming is
// unsupported.
func sseFlusher(c *gin.Context) (http.Flusher, bool) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		respondError(c, http.StatusInternalServerError, "streaming_unsupported", "server does not support streaming")
	}
	return flusher, ok
}

func writeSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	c.Writer.WriteHeader(http.StatusOK)
}

func ssePing(c *gin.Context, flusher http.Flusher) {
	fmt.Fprint(c.Writer, ": ping\n\n")
	flusher.Flush()
}

func writeQueueEvent(c *gin.Context, flusher http.Flusher, ev k8s.QueueEvent) {
	if payload, err := json.Marshal(ev); err == nil {
		writeRawEvent(c, flusher, ev.Type, string(payload))
	}
}

func writeSessionEvent(c *gin.Context, flusher http.Flusher, event string, sess *models.Session) {
	if payload, err := json.Marshal(sess); err == nil {
		writeRawEvent(c, flusher, event, string(payload))
	}
}

func writeRawEvent(c *gin.Context, flusher http.Flusher, event, data string) {
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

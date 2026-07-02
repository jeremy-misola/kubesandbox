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

// Events handles GET /api/sessions/:id/events — a Server-Sent Events stream of
// the session's lifecycle. Ownership is enforced before any data is streamed.
func (h *SessionHandler) Events(c *gin.Context) {
	ident := middleware.GetIdentity(c)
	id := c.Param("id")

	// Ownership + existence check (returns 404 for unknown/unowned/malformed).
	current, err := h.svc.Get(c.Request.Context(), id, ident.Subject)
	if err != nil {
		respondLookupError(c, err)
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "streaming_unsupported",
			"message": "server does not support streaming",
		})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Disable proxy buffering so events flush immediately.
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	// Emit the current state right away.
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
			// SSE comment line as keep-alive ping.
			fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
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
// warm-pool queue progress (Phase E). Events: "queued" (position updates),
// then a terminal "assigned" (with the session) or "error". If the caller is
// not queued but already owns a session, that session is emitted immediately
// (covers the race where assignment lands before the stream opens).
func (h *SessionHandler) QueueEvents(c *gin.Context) {
	ident := middleware.GetIdentity(c)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "streaming_unsupported",
			"message": "server does not support streaming",
		})
		return
	}

	var ch <-chan k8s.QueueEvent
	var unsub func()
	queued := false
	if h.queue != nil {
		ch, unsub, queued = h.queue.Subscribe(ident.Subject)
	}
	if !queued {
		// Not in the queue: maybe already assigned. Emit the session if so.
		sessions, err := h.svc.List(c.Request.Context(), ident.Subject)
		if err == nil && len(sessions) > 0 {
			writeSSEHeaders(c)
			writeQueueEvent(c, flusher, k8s.QueueEvent{Type: "assigned", Session: &sessions[0]})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_queued",
			"message": "you are not in the queue",
		})
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
			fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
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

func writeSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
}

func writeQueueEvent(c *gin.Context, flusher http.Flusher, ev k8s.QueueEvent) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	writeRawEvent(c, flusher, ev.Type, string(payload))
}

func writeSessionEvent(c *gin.Context, flusher http.Flusher, event string, sess *models.Session) {
	payload, err := json.Marshal(sess)
	if err != nil {
		return
	}
	writeRawEvent(c, flusher, event, string(payload))
}

func writeRawEvent(c *gin.Context, flusher http.Flusher, event, data string) {
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

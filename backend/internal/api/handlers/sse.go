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

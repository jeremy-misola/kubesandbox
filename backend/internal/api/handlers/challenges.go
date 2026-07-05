package handlers

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	"github.com/jeremy-misola/kubesandbox/backend/internal/challenges"
	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
)

// ChallengeHandler serves the challenge catalog and the per-session
// grade/reset endpoints (design §7/§8).
type ChallengeHandler struct {
	store  content.Store
	svc    *k8s.SessionService
	grader *challenges.Grader
	seeder *challenges.Seeder

	// Per-session grade rate limit (§7): a min interval, not a budget —
	// guards against a held-down retry key. In-memory per replica by design
	// (degrades to N× the limit with N replicas — acceptable, §15).
	minInterval time.Duration
	mu          sync.Mutex
	lastGrade   map[string]time.Time
}

// NewChallengeHandler constructs a ChallengeHandler.
func NewChallengeHandler(store content.Store, svc *k8s.SessionService, grader *challenges.Grader, seeder *challenges.Seeder, minInterval time.Duration) *ChallengeHandler {
	if minInterval <= 0 {
		minInterval = 2 * time.Second
	}
	return &ChallengeHandler{
		store:       store,
		svc:         svc,
		grader:      grader,
		seeder:      seeder,
		minInterval: minInterval,
		lastGrade:   map[string]time.Time{},
	}
}

// List handles GET /api/challenges — catalog metadata, served from memory.
func (h *ChallengeHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"challenges": h.store.List()})
}

// challengeStep is the public shape of a validation step: id + description
// only — never check internals (§8).
type challengeStep struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// Get handles GET /api/challenges/:id — full metadata including step
// descriptions and hint count. Hint TEXT is revealed via ?hints=n; seed
// manifests and check internals are never returned.
func (h *ChallengeHandler) Get(c *gin.Context) {
	b, ok := h.store.Get(c.Param("id"))
	if !ok {
		respondError(c, http.StatusNotFound, "not_found", "challenge not found")
		return
	}
	steps := make([]challengeStep, 0, len(b.Validate))
	for _, s := range b.Validate {
		steps = append(steps, challengeStep{ID: s.ID, Description: s.Description})
	}
	n, _ := strconv.Atoi(c.DefaultQuery("hints", "0"))
	if n < 0 {
		n = 0
	}
	if n > len(b.Hints) {
		n = len(b.Hints)
	}
	resp := gin.H{
		"id": b.ID, "title": b.Title, "description": b.Description,
		"category": b.Category, "difficulty": b.Difficulty,
		"estMinutes": b.EstMinutes, "tags": b.Tags,
		"steps":      steps,
		"hintsTotal": len(b.Hints),
	}
	if n > 0 {
		resp["hints"] = b.Hints[:n]
	}
	c.JSON(http.StatusOK, resp)
}

// sessionChallenge resolves the caller's session claim and its challenge
// state, writing the 404/409 responses from §7 on the way out.
func (h *ChallengeHandler) sessionChallenge(c *gin.Context) (claimName, challengeID string, ok bool) {
	ident := middleware.GetIdentity(c)
	obj, err := h.svc.GetOwnedClaim(c.Request.Context(), c.Param("id"), ident.Subject)
	if err != nil {
		respondLookupError(c, err)
		return "", "", false
	}
	challengeID = k8s.ChallengeID(obj)
	if challengeID == "" {
		respondError(c, http.StatusNotFound, "no_challenge", "this session has no challenge")
		return "", "", false
	}
	if state := k8s.SeedState(obj); state != k8s.SeedStateSeeded {
		respondError(c, http.StatusConflict, "seed_in_progress",
			"challenge is not ready yet (seed state: "+state+")")
		return "", "", false
	}
	return obj.GetName(), challengeID, true
}

// Grade handles POST /api/sessions/:id/challenge/grade (§7): evaluates the
// bundle's checks against live tenant state. On-demand only.
func (h *ChallengeHandler) Grade(c *gin.Context) {
	claimName, challengeID, ok := h.sessionChallenge(c)
	if !ok {
		return
	}

	if !h.allowGrade(claimName) {
		respondError(c, http.StatusTooManyRequests, "rate_limited",
			"grading is limited to one request per "+h.minInterval.String()+" per session")
		return
	}

	res, err := h.grader.Grade(c.Request.Context(), claimName, challengeID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "grade_failed", "could not grade the challenge")
		return
	}
	c.JSON(http.StatusOK, res)
}

// Reset handles POST /api/sessions/:id/challenge/reset (§7): durable intent
// via CAS (seed-state → pending + reset flag), then the async seeder deletes
// labeled state and re-applies — seconds end-to-end, no pool interaction, no
// new sandbox. The existing SSE stream shows the synthetic Seeding phase
// while it runs.
func (h *ChallengeHandler) Reset(c *gin.Context) {
	claimName, _, ok := h.sessionChallenge(c)
	if !ok {
		return
	}

	// CAS loop: re-read on conflict, give up on anything else.
	ctx := c.Request.Context()
	for attempt := 0; ; attempt++ {
		obj, err := h.svc.GetClaim(ctx, claimName)
		if err != nil {
			respondLookupError(c, err)
			return
		}
		next := obj.DeepCopy()
		annots := next.GetAnnotations()
		annots[k8s.SeedStateAnnot] = k8s.SeedStatePending
		annots[k8s.SeedAttemptsAnnot] = "0"
		annots[k8s.SeedResetAnnot] = "true"
		next.SetAnnotations(annots)
		if _, err := h.svc.UpdateClaim(ctx, next); err != nil {
			if apierrors.IsConflict(err) && attempt < 5 {
				continue
			}
			respondError(c, http.StatusInternalServerError, "reset_failed", "could not reset the challenge")
			return
		}
		break
	}

	h.seeder.Enqueue(claimName)
	c.JSON(http.StatusAccepted, gin.H{
		"status":  "reseeding",
		"message": "challenge state is being reset; watch the session events for the Seeding phase",
	})
}

// allowGrade enforces the per-session min interval and prunes stale entries.
func (h *ChallengeHandler) allowGrade(claimName string) bool {
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	if last, ok := h.lastGrade[claimName]; ok && now.Sub(last) < h.minInterval {
		return false
	}
	h.lastGrade[claimName] = now
	// Prune: the map is bounded by active sessions, but reap anything idle
	// for 100 intervals so deleted sessions don't accumulate forever.
	cutoff := now.Add(-100 * h.minInterval)
	for k, t := range h.lastGrade {
		if t.Before(cutoff) {
			delete(h.lastGrade, k)
		}
	}
	return true
}

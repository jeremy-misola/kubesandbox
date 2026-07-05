package challenges

import (
	"context"
	"fmt"
	"time"

	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
	"github.com/jeremy-misola/kubesandbox/backend/internal/telemetry"
)

// Grader evaluates a bundle's declarative checks against live tenant state
// (design §7). On-demand only: the cost is a handful of GETs against the
// TENANT API — zero host-control-plane load, which is why grading is safe at
// any catalog size. No background polling exists anywhere.
type Grader struct {
	store   content.Store
	tenant  TenantOps
	metrics *telemetry.Metrics
	now     func() time.Time
}

// NewGrader constructs a Grader.
func NewGrader(store content.Store, tenant TenantOps) *Grader {
	return &Grader{store: store, tenant: tenant, now: time.Now}
}

// SetMetrics injects the telemetry instrument set (nil is a valid no-op).
func (g *Grader) SetMetrics(m *telemetry.Metrics) { g.metrics = m }

// Grade evaluates every step of the session's challenge. Steps are
// independent and ALL evaluated — no short-circuit, so the user sees
// everything left to fix (§7). A step passes iff all its checks pass; the
// result's Pass is the conjunction.
//
// Check-semantic failures become {pass:false, message}; infrastructure
// failures (tenant unreachable, unknown bundle) return an error and grade
// nothing.
func (g *Grader) Grade(ctx context.Context, claimName, challengeID string) (*models.GradeResult, error) {
	bundle, ok := g.store.Get(challengeID)
	if !ok {
		return nil, fmt.Errorf("challenge %q not in catalog", challengeID)
	}

	res := &models.GradeResult{
		ChallengeID: challengeID,
		Pass:        true,
		Steps:       make([]models.GradeStep, 0, len(bundle.Validate)),
		GradedAt:    g.now().UTC().Format(time.RFC3339),
	}
	for _, step := range bundle.Validate {
		st := models.GradeStep{ID: step.ID, Description: step.Description, Pass: true}
		for _, check := range step.Checks {
			pass, msg, err := g.evalCheck(ctx, claimName, check)
			if err != nil {
				return nil, fmt.Errorf("step %s: %w", step.ID, err)
			}
			if !pass {
				st.Pass = false
				if st.Message == "" {
					st.Message = msg
				}
				// keep evaluating remaining checks? Within a step the first
				// failure message is the actionable one; later checks in the
				// same step usually depend on the same object. Stop here.
				break
			}
		}
		if !st.Pass {
			res.Pass = false
		}
		res.Steps = append(res.Steps, st)
	}

	g.metrics.RecordGrade(ctx, challengeID, res.Pass)
	return res, nil
}

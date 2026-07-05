package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
	"github.com/jeremy-misola/kubesandbox/backend/internal/challenges"
	"github.com/jeremy-misola/kubesandbox/backend/internal/config"
	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
	k8s "github.com/jeremy-misola/kubesandbox/backend/internal/kubernetes"
	"github.com/jeremy-misola/kubesandbox/backend/internal/models"
)

var testCMGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

// newHandlerFakeSvc mirrors the rv-enforcing fake from pool_test.go.
func newHandlerFakeSvc(t *testing.T, objs ...runtime.Object) *k8s.SessionService {
	t.Helper()
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		models.GVR: models.Kind + "List",
		testCMGVR:  "ConfigMapList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)
	var mu sync.Mutex
	client.PrependReactor("update", "kubesandboxsessions", func(action clienttesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		defer mu.Unlock()
		ua := action.(clienttesting.UpdateAction)
		obj := ua.GetObject().(*unstructured.Unstructured)
		cur, err := client.Tracker().Get(ua.GetResource(), ua.GetNamespace(), obj.GetName())
		if err != nil {
			return true, nil, err
		}
		curRV := cur.(*unstructured.Unstructured).GetResourceVersion()
		if obj.GetResourceVersion() != curRV {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: models.APIGroup, Resource: models.Plural},
				obj.GetName(), fmt.Errorf("rv mismatch"))
		}
		n, _ := strconv.Atoi(curRV)
		next := obj.DeepCopy()
		next.SetResourceVersion(strconv.Itoa(n + 1))
		if err := client.Tracker().Update(ua.GetResource(), next, ua.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, next, nil
	})
	return k8s.NewSessionService(client, "playground", "https://kubesandbox.com", models.DefaultWorkspaceImage)
}

func sessionClaim(name, owner, challengeID, seedState string) *unstructured.Unstructured {
	annots := map[string]interface{}{}
	if challengeID != "" {
		annots["kubesandbox.com/challenge-id"] = challengeID
		annots["kubesandbox.com/seed-state"] = seedState
		annots["kubesandbox.com/seed-attempts"] = "1"
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": models.APIGroup + "/" + models.APIVersion,
		"kind":       models.Kind,
		"metadata": map[string]interface{}{
			"name":            name,
			"namespace":       "playground",
			"resourceVersion": "1",
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "kubesandbox-backend",
				"kubesandbox.com/pool":         "claimed",
			},
			"annotations": annots,
		},
		"spec":   map[string]interface{}{"ownerRef": owner, "tenantRef": owner},
		"status": map[string]interface{}{"workspaceReady": true},
	}}
}

// absentTenant makes every read a NotFound: bundles whose only check is
// resourceAbsent then grade to pass:true without a live tenant.
type absentTenant struct{}

func (absentTenant) Apply(context.Context, string, *content.Bundle) error { return nil }
func (absentTenant) DeleteSeeded(context.Context, string, string) error   { return nil }
func (absentTenant) GetObject(_ context.Context, _ string, t content.TargetRef) (*unstructured.Unstructured, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "x"}, t.Name)
}
func (absentTenant) ListObjects(context.Context, string, content.TargetRef) ([]unstructured.Unstructured, error) {
	return nil, nil
}
func (absentTenant) CanI(context.Context, string, content.SubjectRef, content.AccessRef) (bool, error) {
	return false, nil
}

func gradePassBundle(t *testing.T, id string) *content.Bundle {
	t.Helper()
	y := fmt.Sprintf(`apiVersion: content.kubesandbox.com/v1
id: %s
title: T
description: d
category: rbac
difficulty: easy
hints: [h1, h2, h3]
validate:
  - id: s1
    description: cleanliness
    checks:
      - type: resourceAbsent
        target: {apiVersion: v1, kind: ConfigMap, namespace: demo, name: leftover}
`, id)
	b, err := content.LoadBundle([]byte(y), map[string][]byte{
		"00-ns.yaml": []byte("apiVersion: v1\nkind: Namespace\nmetadata: {name: demo}\n"),
	})
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	return b
}

func newChallengeRouter(t *testing.T, svc *k8s.SessionService, store content.Store, minInterval time.Duration) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	grader := challenges.NewGrader(store, absentTenant{})
	seeder := challenges.NewSeeder(svc, store, absentTenant{}, challenges.SeederConfig{Resync: time.Hour})
	h := NewChallengeHandler(store, svc, grader, seeder, minInterval)
	sessions := NewSessionHandler(svc, k8s.NewAssignQueue(), store, nil)

	r := gin.New()
	api := r.Group("/api")
	api.Use(middleware.IdentityMiddleware(config.Config{UserIDHeader: "X-User-Id", UserEmailHeader: "X-User-Email"}))
	api.POST("/sessions", sessions.Create)
	api.GET("/challenges", h.List)
	api.GET("/challenges/:id", h.Get)
	api.POST("/sessions/:id/challenge/grade", h.Grade)
	api.POST("/sessions/:id/challenge/reset", h.Reset)
	return r
}

func doReq(r *gin.Engine, method, path, user, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-User-Id", user)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateRejectsUnknownChallengeID(t *testing.T) {
	svc := newHandlerFakeSvc(t)
	store := content.FixedStore{"real-challenge": gradePassBundle(t, "real-challenge")}
	r := newChallengeRouter(t, svc, store, time.Second)

	w := doReq(r, http.MethodPost, "/api/sessions", "alice", `{"challengeId":"nope"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for unknown challenge id", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unknown_challenge") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestGradeStatusContract(t *testing.T) {
	store := content.FixedStore{"ch-1": gradePassBundle(t, "ch-1")}
	svc := newHandlerFakeSvc(t,
		sessionClaim("s-nochal", "alice", "", ""),
		sessionClaim("s-seeding", "alice", "ch-1", "seeding"),
		sessionClaim("s-ready", "alice", "ch-1", "seeded"),
	)
	r := newChallengeRouter(t, svc, store, time.Hour) // huge interval to test 429

	// 404: no challenge on the session.
	if w := doReq(r, http.MethodPost, "/api/sessions/playground-s-nochal/challenge/grade", "alice", ""); w.Code != http.StatusNotFound {
		t.Fatalf("no-challenge grade = %d, want 404", w.Code)
	}
	// 404: someone else's session (no existence leak).
	if w := doReq(r, http.MethodPost, "/api/sessions/playground-s-ready/challenge/grade", "mallory", ""); w.Code != http.StatusNotFound {
		t.Fatalf("non-owner grade = %d, want 404", w.Code)
	}
	// 409: mid-seed.
	if w := doReq(r, http.MethodPost, "/api/sessions/playground-s-seeding/challenge/grade", "alice", ""); w.Code != http.StatusConflict {
		t.Fatalf("mid-seed grade = %d, want 409", w.Code)
	}
	// 200: seeded, and the result envelope is complete.
	w := doReq(r, http.MethodPost, "/api/sessions/playground-s-ready/challenge/grade", "alice", "")
	if w.Code != http.StatusOK {
		t.Fatalf("grade = %d (%s), want 200", w.Code, w.Body.String())
	}
	var res models.GradeResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad body: %v", err)
	}
	if !res.Pass || len(res.Steps) != 1 || res.ChallengeID != "ch-1" {
		t.Fatalf("result = %+v", res)
	}
	// 429: immediately again, under the min interval.
	if w := doReq(r, http.MethodPost, "/api/sessions/playground-s-ready/challenge/grade", "alice", ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("rapid re-grade = %d, want 429", w.Code)
	}
}

func TestResetStatusContractAndDurableIntent(t *testing.T) {
	store := content.FixedStore{"ch-1": gradePassBundle(t, "ch-1")}
	svc := newHandlerFakeSvc(t,
		sessionClaim("s-seeding", "alice", "ch-1", "seeding"),
		sessionClaim("s-ready", "alice", "ch-1", "seeded"),
	)
	r := newChallengeRouter(t, svc, store, time.Second)

	// 409 mid-seed.
	if w := doReq(r, http.MethodPost, "/api/sessions/playground-s-seeding/challenge/reset", "alice", ""); w.Code != http.StatusConflict {
		t.Fatalf("mid-seed reset = %d, want 409", w.Code)
	}
	// 202 on a seeded session, with durable intent CAS'd onto the claim.
	w := doReq(r, http.MethodPost, "/api/sessions/playground-s-ready/challenge/reset", "alice", "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("reset = %d (%s), want 202", w.Code, w.Body.String())
	}
	obj, err := svc.GetClaim(context.Background(), "s-ready")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	annots := obj.GetAnnotations()
	if annots["kubesandbox.com/seed-state"] != "pending" || annots["kubesandbox.com/seed-reset"] != "true" {
		t.Fatalf("reset intent not durable on the claim: %v", annots)
	}
}

func TestCatalogEndpoints(t *testing.T) {
	store := content.FixedStore{"ch-1": gradePassBundle(t, "ch-1")}
	r := newChallengeRouter(t, newHandlerFakeSvc(t), store, time.Second)

	w := doReq(r, http.MethodGet, "/api/challenges", "alice", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "ch-1") {
		t.Fatalf("catalog = %d %s", w.Code, w.Body.String())
	}

	// Detail: steps + hint COUNT, no hint text unless asked, never internals.
	w = doReq(r, http.MethodGet, "/api/challenges/ch-1", "alice", "")
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, `"hintsTotal":3`) {
		t.Fatalf("detail = %d %s", w.Code, body)
	}
	if strings.Contains(body, "h1") || strings.Contains(body, "resourceAbsent") || strings.Contains(body, "leftover") {
		t.Fatalf("detail must not leak hint text or check internals: %s", body)
	}

	w = doReq(r, http.MethodGet, "/api/challenges/ch-1?hints=2", "alice", "")
	if !strings.Contains(w.Body.String(), "h2") || strings.Contains(w.Body.String(), "h3") {
		t.Fatalf("hints=2 should reveal exactly two hints: %s", w.Body.String())
	}

	if w := doReq(r, http.MethodGet, "/api/challenges/ghost", "alice", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown challenge = %d, want 404", w.Code)
	}
}

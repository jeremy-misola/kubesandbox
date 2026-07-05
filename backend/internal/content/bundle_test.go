package content

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const validChallengeYAML = `apiVersion: content.kubesandbox.com/v1
id: test-challenge
title: Test Challenge
description: A test.
category: rbac
difficulty: easy
estMinutes: 5
hints: [one, two]
validate:
  - id: step-1
    description: something exists
    checks:
      - type: resourceExists
        target: {apiVersion: v1, kind: ConfigMap, namespace: demo, name: cm}
`

func validSeed() map[string][]byte {
	return map[string][]byte{
		"10-deployment.yaml": []byte(`apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: demo}
spec: {replicas: 1}
`),
		"00-namespace.yaml": []byte(`apiVersion: v1
kind: Namespace
metadata: {name: demo}
`),
		"05-rbac.yaml": []byte(`apiVersion: v1
kind: ServiceAccount
metadata: {name: sa, namespace: demo}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: r, namespace: demo}
rules: []
`),
	}
}

func TestLoadBundleOrdersAndLabelsSeed(t *testing.T) {
	b, err := LoadBundle([]byte(validChallengeYAML), validSeed())
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	if b.ID != "test-challenge" || len(b.Seed) != 4 {
		t.Fatalf("id=%q seeds=%d, want test-challenge/4", b.ID, len(b.Seed))
	}
	// Apply order: Namespace, then RBAC (SA, Role), then Deployment (§6.2).
	kinds := make([]string, 0, 4)
	for _, s := range b.Seed {
		kinds = append(kinds, s.Object.GetKind())
		if got := s.Object.GetLabels()[ChallengeLabel]; got != "test-challenge" {
			t.Fatalf("%s: challenge label = %q (reset/cleanup depend on it)", s.Object.GetKind(), got)
		}
	}
	want := []string{"Namespace", "ServiceAccount", "Role", "Deployment"}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("apply order %v, want %v", kinds, want)
		}
	}
}

func TestLintRejections(t *testing.T) {
	cases := []struct {
		name      string
		challenge string
		seed      map[string][]byte
		wantErr   string
	}{
		{
			name:      "unknown apiVersion is quarantined, never guessed at",
			challenge: strings.Replace(validChallengeYAML, "content.kubesandbox.com/v1", "content.kubesandbox.com/v2", 1),
			seed:      validSeed(),
			wantErr:   "apiVersion",
		},
		{
			name:      "unknown check type",
			challenge: strings.Replace(validChallengeYAML, "type: resourceExists", "type: probe", 1),
			seed:      validSeed(),
			wantErr:   "unknown check type",
		},
		{
			name:      "heavy excluded from v1",
			challenge: validChallengeYAML + "heavy: true\n",
			seed:      validSeed(),
			wantErr:   "heavy",
		},
		{
			name:      "cluster-scoped seed kind rejected",
			challenge: validChallengeYAML,
			seed: map[string][]byte{"00-cr.yaml": []byte(
				"apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata: {name: bad}\nrules: []\n")},
			wantErr: "cluster-scoped",
		},
		{
			name:      "kind outside known table rejected",
			challenge: validChallengeYAML,
			seed: map[string][]byte{"00-crd.yaml": []byte(
				"apiVersion: example.com/v1\nkind: Widget\nmetadata: {name: w, namespace: demo}\n")},
			wantErr: "unsupported kind",
		},
		{
			name:      "namespaced seed without namespace rejected",
			challenge: validChallengeYAML,
			seed: map[string][]byte{"00-cm.yaml": []byte(
				"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: c}\n")},
			wantErr: "metadata.namespace",
		},
		{
			name:      "no seed manifests",
			challenge: validChallengeYAML,
			seed:      map[string][]byte{},
			wantErr:   "at least one manifest",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadBundle([]byte(tc.challenge), tc.seed)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateDirIDMismatch(t *testing.T) {
	_, errs := ValidateDir("some-other-dir", []byte(validChallengeYAML), validSeed())
	if len(errs) == 0 || !strings.Contains(errs[0], "does not match directory") {
		t.Fatalf("errs = %v, want id/directory mismatch", errs)
	}
}

// --- ConfigMapStore ---

func bundleConfigMap(name, challengeYAML string, seed map[string][]byte) *unstructured.Unstructured {
	data := map[string]interface{}{"challenge.yaml": challengeYAML}
	for k, v := range seed {
		data[k] = string(v)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "playground",
			"labels":    map[string]interface{}{BundleConfigMapLabel: "true"},
		},
		"data": data,
	}}
}

func newFakeStore(t *testing.T, objs ...runtime.Object) *ConfigMapStore {
	t.Helper()
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)
	return NewConfigMapStore(client, "playground", 0)
}

func TestStoreQuarantinesInvalidBundles(t *testing.T) {
	valid := bundleConfigMap(BundleConfigMapPrefix+"test-challenge", validChallengeYAML, map[string][]byte{
		"00-namespace.yaml": []byte("apiVersion: v1\nkind: Namespace\nmetadata: {name: demo}\n"),
	})
	// Invalid: schema version the backend doesn't know.
	badYAML := strings.Replace(validChallengeYAML, "content.kubesandbox.com/v1", "content.kubesandbox.com/v9", 1)
	badYAML = strings.Replace(badYAML, "id: test-challenge", "id: future-challenge", 1)
	invalid := bundleConfigMap(BundleConfigMapPrefix+"future-challenge", badYAML, map[string][]byte{
		"00-namespace.yaml": []byte("apiVersion: v1\nkind: Namespace\nmetadata: {name: demo}\n"),
	})
	// Invalid: ConfigMap name doesn't match bundle id.
	misnamed := bundleConfigMap(BundleConfigMapPrefix+"wrong-name", validChallengeYAML, map[string][]byte{
		"00-namespace.yaml": []byte("apiVersion: v1\nkind: Namespace\nmetadata: {name: demo}\n"),
	})

	s := newFakeStore(t, valid, invalid, misnamed)
	if err := s.RebuildOnce(context.Background()); err != nil {
		t.Fatalf("RebuildOnce: %v (invalid bundles must be quarantined, never a crash)", err)
	}

	if _, ok := s.Get("test-challenge"); !ok {
		t.Fatalf("valid bundle missing from catalog")
	}
	if _, ok := s.Get("future-challenge"); ok {
		t.Fatalf("unknown-apiVersion bundle must be quarantined")
	}
	if got := s.List(); len(got) != 1 {
		t.Fatalf("catalog size = %d, want 1", len(got))
	}
	q := s.Quarantined()
	if len(q) != 2 {
		t.Fatalf("quarantined = %v, want the invalid + misnamed ConfigMaps", q)
	}
}

func TestStoreServesRealBundlesFromChart(t *testing.T) {
	// Guard against drift between the authored chart bundles and the loader:
	// this is covered end-to-end by the validator CLI in CI; here we just
	// confirm the loader accepts the design-doc example shape with where
	// predicates and subjectCan.
	y := `apiVersion: content.kubesandbox.com/v1
id: rbac-example
title: RBAC
description: d
category: rbac
difficulty: medium
validate:
  - id: can
    description: sa can list pods
    checks:
      - type: subjectCan
        serviceAccount: {namespace: monitoring, name: metrics-agent}
        access: {verb: list, resource: pods, namespace: monitoring}
  - id: rb
    description: rolebinding grants role
    checks:
      - type: resourceExists
        target: {apiVersion: rbac.authorization.k8s.io/v1, kind: RoleBinding, namespace: monitoring}
        where:
          - {path: .roleRef.name, equals: pod-reader}
`
	if _, err := LoadBundle([]byte(y), map[string][]byte{
		"00-ns.yaml": []byte("apiVersion: v1\nkind: Namespace\nmetadata: {name: monitoring}\n"),
	}); err != nil {
		t.Fatalf("design-doc example shape must load: %v", err)
	}
}

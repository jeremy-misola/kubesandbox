// Package content implements the challenge content pipeline: the bundle
// schema (content.kubesandbox.com/v1), the shared validator used by both the
// CI CLI and the backend, and the ConfigMap-backed ContentStore the backend
// watches at runtime (docs/history/challenges-backend-architecture.md §4).
//
// A challenge bundle is authored as a directory in git:
//
//	challenges/<id>/
//	├── challenge.yaml   # metadata + validation checks
//	└── seed/            # manifests applied into the tenant vcluster
//	    ├── 00-namespace.yaml
//	    └── 10-....yaml
//
// and delivered as ONE ConfigMap per challenge (key challenge.yaml + one key
// per seed file basename), labeled kubesandbox.com/challenge-bundle=true,
// rendered by the kubesandbox-challenges chart and synced by ArgoCD. Adding a
// challenge is a git push — no backend rebuild.
package content

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// BundleAPIVersion is the schema version this backend understands. Bundles
// carrying any other apiVersion are quarantined, never guessed at — the guard
// against content/backend schema skew.
const BundleAPIVersion = "content.kubesandbox.com/v1"

// ChallengeLabel is injected on every seed object at load time; reset and
// cleanup depend on it (grader-side reset deletes by this label).
const ChallengeLabel = "kubesandbox.com/challenge"

// BundleConfigMapLabel marks the delivery ConfigMaps the ContentStore watches.
const BundleConfigMapLabel = "kubesandbox.com/challenge-bundle"

// BundleConfigMapPrefix prefixes each bundle's ConfigMap name; the suffix must
// equal the bundle id (verified at watch time).
const BundleConfigMapPrefix = "sbxchallenge-"

// SizeWarnBytes is the per-bundle size guard: past this the validator warns
// (well under the ~1 MiB ConfigMap ceiling; a generous bundle is tens of KB).
const SizeWarnBytes = 256 * 1024

// Check types (v1) — deliberately declarative and read-only (§4).
const (
	CheckResourceExists      = "resourceExists"
	CheckResourceAbsent      = "resourceAbsent"
	CheckFieldEquals         = "fieldEquals"
	CheckFieldMatches        = "fieldMatches"
	CheckPodReady            = "podReady"
	CheckDeploymentAvailable = "deploymentAvailable"
	CheckSubjectCan          = "subjectCan"
	CheckSubjectCannot       = "subjectCannot"
)

// KnownCheckTypes is the closed set of v1 check types. Anything else (e.g. the
// v2 `probe` escape hatch) is a validation error.
var KnownCheckTypes = map[string]bool{
	CheckResourceExists:      true,
	CheckResourceAbsent:      true,
	CheckFieldEquals:         true,
	CheckFieldMatches:        true,
	CheckPodReady:            true,
	CheckDeploymentAvailable: true,
	CheckSubjectCan:          true,
	CheckSubjectCannot:       true,
}

// Categories allowed by the schema.
var KnownCategories = map[string]bool{
	"rbac": true, "networkpolicy": true, "workloads": true, "config": true,
	"scheduling": true, "storage-lite": true, "troubleshooting": true,
}

// Difficulties allowed by the schema.
var KnownDifficulties = map[string]bool{"easy": true, "medium": true, "hard": true}

// Bundle is one parsed challenge: challenge.yaml plus loaded seed objects.
type Bundle struct {
	APIVersion  string   `json:"apiVersion"`
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Difficulty  string   `json:"difficulty"`
	EstMinutes  int      `json:"estMinutes,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Hints       []string `json:"hints,omitempty"`
	// Heavy is reserved (§10): v1 ships no heavy bundles, but the flag exists
	// so a future slow-path tier needs no schema change.
	Heavy    bool   `json:"heavy,omitempty"`
	Validate []Step `json:"validate"`

	// Seed holds the parsed seed manifests in apply order (Namespaces → RBAC →
	// everything else; numeric filename prefixes are the tiebreaker), with the
	// ChallengeLabel already injected. Populated by the loader, not the YAML.
	Seed []SeedObject `json:"-"`
}

// SeedObject is one manifest destined for the tenant vcluster.
type SeedObject struct {
	// File is the seed key (file basename) the object came from, for ordering
	// and error messages.
	File string
	// Doc is the document index within the file (multi-doc YAML).
	Doc    int
	Object *unstructured.Unstructured
}

// Step is one gradeable step; it passes iff all its checks pass.
type Step struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Checks      []Check `json:"checks"`
}

// Check is one declarative assertion against live tenant state.
type Check struct {
	Type string `json:"type"`

	// Target selects the object(s) for resourceExists/resourceAbsent/
	// fieldEquals/fieldMatches/podReady/deploymentAvailable.
	Target *TargetRef `json:"target,omitempty"`

	// Where holds optional field predicates applied to matched object(s)
	// (resourceExists: at least one matched object must satisfy all;
	// resourceAbsent: no matched object may exist at all — where is invalid).
	Where []Predicate `json:"where,omitempty"`

	// Path/Equals/Matches drive fieldEquals and fieldMatches.
	Path    string      `json:"path,omitempty"`
	Equals  interface{} `json:"equals,omitempty"`
	Matches string      `json:"matches,omitempty"`

	// MinAvailable is the deploymentAvailable threshold (default 1).
	MinAvailable int `json:"minAvailable,omitempty"`

	// ServiceAccount + Access drive subjectCan/subjectCannot: a
	// SelfSubjectAccessReview issued while impersonating the ServiceAccount.
	ServiceAccount *SubjectRef `json:"serviceAccount,omitempty"`
	Access         *AccessRef  `json:"access,omitempty"`
}

// TargetRef selects Kubernetes object(s) inside the tenant vcluster.
type TargetRef struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	// Name and LabelSelector are mutually exclusive; empty LabelSelector with
	// no Name means "any object of that kind in the namespace".
	Name          string `json:"name,omitempty"`
	LabelSelector string `json:"labelSelector,omitempty"`
}

// Predicate is a field assertion on a matched object.
type Predicate struct {
	Path    string      `json:"path"`
	Equals  interface{} `json:"equals,omitempty"`
	Matches string      `json:"matches,omitempty"`
}

// SubjectRef names a ServiceAccount inside the tenant vcluster.
type SubjectRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// AccessRef is the action a subjectCan/subjectCannot check asserts on.
type AccessRef struct {
	Verb      string `json:"verb"`
	APIGroup  string `json:"apiGroup,omitempty"`
	Resource  string `json:"resource"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// Meta is the catalog-listing subset of a bundle (never exposes seed
// manifests or check internals).
type Meta struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Difficulty  string   `json:"difficulty"`
	EstMinutes  int      `json:"estMinutes,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Meta projects the bundle's catalog listing.
func (b *Bundle) Meta() Meta {
	return Meta{
		ID: b.ID, Title: b.Title, Description: b.Description,
		Category: b.Category, Difficulty: b.Difficulty,
		EstMinutes: b.EstMinutes, Tags: b.Tags,
	}
}

// knownKind describes a kind the pipeline understands: its resource name and
// scope. Restricting seed manifests AND check targets to this closed table is
// deliberate — it removes any need for live discovery/RESTMapper machinery in
// the seeder and grader (deterministic, trivially testable), and the validator
// enforces it in CI so an unsupported kind can never reach the cluster.
type knownKind struct {
	resource   string
	namespaced bool
}

// knownKinds maps GroupVersionKind → resource/scope for the CKAD-shaped
// content scope (§1). Extend the table (and this comment) when the catalog
// genuinely needs a new kind; the validator error names this location.
var knownKinds = map[schema.GroupVersionKind]knownKind{
	{Version: "v1", Kind: "Namespace"}:             {"namespaces", false},
	{Version: "v1", Kind: "Pod"}:                   {"pods", true},
	{Version: "v1", Kind: "Service"}:               {"services", true},
	{Version: "v1", Kind: "ConfigMap"}:             {"configmaps", true},
	{Version: "v1", Kind: "Secret"}:                {"secrets", true},
	{Version: "v1", Kind: "ServiceAccount"}:        {"serviceaccounts", true},
	{Version: "v1", Kind: "ResourceQuota"}:         {"resourcequotas", true},
	{Version: "v1", Kind: "LimitRange"}:            {"limitranges", true},
	{Version: "v1", Kind: "PersistentVolume"}:      {"persistentvolumes", false},
	{Version: "v1", Kind: "PersistentVolumeClaim"}: {"persistentvolumeclaims", true},

	{Group: "apps", Version: "v1", Kind: "Deployment"}:  {"deployments", true},
	{Group: "apps", Version: "v1", Kind: "StatefulSet"}: {"statefulsets", true},
	{Group: "apps", Version: "v1", Kind: "DaemonSet"}:   {"daemonsets", true},
	{Group: "apps", Version: "v1", Kind: "ReplicaSet"}:  {"replicasets", true},

	{Group: "batch", Version: "v1", Kind: "Job"}:     {"jobs", true},
	{Group: "batch", Version: "v1", Kind: "CronJob"}: {"cronjobs", true},

	{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"}:               {"roles", true},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"}:        {"rolebindings", true},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}:        {"clusterroles", false},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}: {"clusterrolebindings", false},

	{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"}: {"networkpolicies", true},
	{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"}:       {"ingresses", true},

	{Group: "autoscaling", Version: "v2", Kind: "HorizontalPodAutoscaler"}: {"horizontalpodautoscalers", true},

	{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}: {"storageclasses", false},
}

// KnownGVR pairs a resolvable resource with its scope.
type KnownGVR struct {
	GVR        schema.GroupVersionResource
	Namespaced bool
}

// KnownGVRs returns every resource in the known-kinds table (reset iterates
// it to find labeled leftovers).
func KnownGVRs() []KnownGVR {
	out := make([]KnownGVR, 0, len(knownKinds))
	for gvk, k := range knownKinds {
		out = append(out, KnownGVR{
			GVR:        gvk.GroupVersion().WithResource(k.resource),
			Namespaced: k.namespaced,
		})
	}
	return out
}

// GVRForKind resolves an apiVersion/kind pair against the known-kinds table.
func GVRForKind(apiVersion, kind string) (schema.GroupVersionResource, bool, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, false, fmt.Errorf("invalid apiVersion %q: %w", apiVersion, err)
	}
	k, ok := knownKinds[gv.WithKind(kind)]
	if !ok {
		return schema.GroupVersionResource{}, false, fmt.Errorf(
			"unsupported kind %s/%s (extend knownKinds in backend/internal/content/bundle.go if the catalog needs it)",
			apiVersion, kind)
	}
	return gv.WithResource(k.resource), k.namespaced, nil
}

// seedOrder assigns the apply-phase rank: Namespaces first, then RBAC objects,
// then everything else (§6.2). Within a phase the (file, doc) order — i.e. the
// numeric filename prefixes — is the tiebreaker.
func seedOrder(obj *unstructured.Unstructured) int {
	switch obj.GetKind() {
	case "Namespace":
		return 0
	case "ServiceAccount", "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		return 1
	default:
		return 2
	}
}

// LoadBundle parses one bundle from its delivery form: the challenge.yaml
// bytes plus the seed files keyed by basename. It validates the result and
// injects the ChallengeLabel on every seed object. This is the single load
// path shared by the validator CLI and the backend's ContentStore.
func LoadBundle(challengeYAML []byte, seedFiles map[string][]byte) (*Bundle, error) {
	var b Bundle
	if err := yaml.UnmarshalStrict(challengeYAML, &b); err != nil {
		return nil, fmt.Errorf("challenge.yaml: %w", err)
	}

	names := make([]string, 0, len(seedFiles))
	for name := range seedFiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		docs, err := splitYAMLDocs(seedFiles[name])
		if err != nil {
			return nil, fmt.Errorf("seed %s: %w", name, err)
		}
		for i, doc := range docs {
			obj := &unstructured.Unstructured{}
			if err := yaml.Unmarshal(doc, &obj.Object); err != nil {
				return nil, fmt.Errorf("seed %s doc %d: %w", name, i, err)
			}
			if len(obj.Object) == 0 {
				continue // empty doc (comments only)
			}
			labels := obj.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels[ChallengeLabel] = b.ID
			obj.SetLabels(labels)
			b.Seed = append(b.Seed, SeedObject{File: name, Doc: i, Object: obj})
		}
	}

	// Stable apply order: phase rank, then filename, then doc index.
	sort.SliceStable(b.Seed, func(i, j int) bool {
		oi, oj := seedOrder(b.Seed[i].Object), seedOrder(b.Seed[j].Object)
		if oi != oj {
			return oi < oj
		}
		if b.Seed[i].File != b.Seed[j].File {
			return b.Seed[i].File < b.Seed[j].File
		}
		return b.Seed[i].Doc < b.Seed[j].Doc
	})

	if errs := b.Lint(); len(errs) > 0 {
		return nil, fmt.Errorf("invalid bundle %q: %s", b.ID, strings.Join(errs, "; "))
	}
	return &b, nil
}

// splitYAMLDocs splits a multi-document YAML stream on "---" separators.
func splitYAMLDocs(data []byte) ([][]byte, error) {
	var out [][]byte
	for _, part := range strings.Split(string(data), "\n---") {
		part = strings.TrimPrefix(part, "---")
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, []byte(part))
	}
	return out, nil
}

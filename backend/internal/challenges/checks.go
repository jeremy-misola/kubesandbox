package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jeremy-misola/kubesandbox/backend/internal/content"
)

// evalCheck dispatches one declarative check (design §4 check types).
// Returns (pass, failureMessage, infrastructureError). Failure messages name
// the object and observed vs expected values — never evaluator internals.
func (g *Grader) evalCheck(ctx context.Context, claimName string, c content.Check) (bool, string, error) {
	switch c.Type {
	case content.CheckResourceExists:
		return g.evalResourceExists(ctx, claimName, c)
	case content.CheckResourceAbsent:
		return g.evalResourceAbsent(ctx, claimName, c)
	case content.CheckFieldEquals, content.CheckFieldMatches:
		return g.evalFieldCheck(ctx, claimName, c)
	case content.CheckPodReady:
		return g.evalPodReady(ctx, claimName, c)
	case content.CheckDeploymentAvailable:
		return g.evalDeploymentAvailable(ctx, claimName, c)
	case content.CheckSubjectCan, content.CheckSubjectCannot:
		return g.evalSubjectCheck(ctx, claimName, c)
	default:
		// Unreachable: bundles are validated at load; belt-and-braces only.
		return false, "", fmt.Errorf("unknown check type %q", c.Type)
	}
}

// targetLabel renders "Kind namespace/name" (or selector) for messages.
func targetLabel(t *content.TargetRef) string {
	name := t.Name
	if name == "" {
		if t.LabelSelector != "" {
			name = "[" + t.LabelSelector + "]"
		} else {
			name = "*"
		}
	}
	if t.Namespace == "" {
		return fmt.Sprintf("%s %s", t.Kind, name)
	}
	return fmt.Sprintf("%s %s/%s", t.Kind, t.Namespace, name)
}

// fetchTargets resolves a target to candidate objects: a named Get or a
// (label-)List. NotFound is a semantic miss, not an error.
func (g *Grader) fetchTargets(ctx context.Context, claimName string, t content.TargetRef) ([]unstructured.Unstructured, error) {
	if t.Name != "" {
		obj, err := g.tenant.GetObject(ctx, claimName, t)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return []unstructured.Unstructured{*obj}, nil
	}
	items, err := g.tenant.ListObjects(ctx, claimName, t)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return items, nil
}

func (g *Grader) evalResourceExists(ctx context.Context, claimName string, c content.Check) (bool, string, error) {
	items, err := g.fetchTargets(ctx, claimName, *c.Target)
	if err != nil {
		return false, "", err
	}
	if len(items) == 0 {
		return false, fmt.Sprintf("%s: not found", targetLabel(c.Target)), nil
	}
	if len(c.Where) == 0 {
		return true, "", nil
	}
	// At least one matched object must satisfy ALL predicates.
	var firstMiss string
	for i := range items {
		miss := predicatesMiss(&items[i], c.Where)
		if miss == "" {
			return true, "", nil
		}
		if firstMiss == "" {
			firstMiss = miss
		}
	}
	return false, fmt.Sprintf("%s: %s", targetLabel(c.Target), firstMiss), nil
}

func (g *Grader) evalResourceAbsent(ctx context.Context, claimName string, c content.Check) (bool, string, error) {
	items, err := g.fetchTargets(ctx, claimName, *c.Target)
	if err != nil {
		return false, "", err
	}
	if len(items) > 0 {
		return false, fmt.Sprintf("%s: expected absent, found %d object(s)", targetLabel(c.Target), len(items)), nil
	}
	return true, "", nil
}

func (g *Grader) evalFieldCheck(ctx context.Context, claimName string, c content.Check) (bool, string, error) {
	obj, err := g.tenant.GetObject(ctx, claimName, *c.Target)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, fmt.Sprintf("%s: not found", targetLabel(c.Target)), nil
		}
		return false, "", err
	}
	val, found := evalPath(obj.Object, c.Path)
	if !found {
		return false, fmt.Sprintf("%s: %s is not set", targetLabel(c.Target), c.Path), nil
	}
	if c.Type == content.CheckFieldEquals {
		if jsonEqual(val, c.Equals) {
			return true, "", nil
		}
		return false, fmt.Sprintf("%s: %s is %s, want %s",
			targetLabel(c.Target), c.Path, renderValue(val), renderValue(c.Equals)), nil
	}
	re, err := regexp.Compile(c.Matches) // validated at load; compile is cheap
	if err != nil {
		return false, "", fmt.Errorf("invalid regex %q: %w", c.Matches, err)
	}
	if re.MatchString(stringify(val)) {
		return true, "", nil
	}
	return false, fmt.Sprintf("%s: %s is %s, want match for %q",
		targetLabel(c.Target), c.Path, renderValue(val), c.Matches), nil
}

func (g *Grader) evalPodReady(ctx context.Context, claimName string, c content.Check) (bool, string, error) {
	t := *c.Target
	t.APIVersion, t.Kind = "v1", "Pod"
	items, err := g.fetchTargets(ctx, claimName, t)
	if err != nil {
		return false, "", err
	}
	if len(items) == 0 {
		return false, fmt.Sprintf("%s: not found", targetLabel(&t)), nil
	}
	for i := range items {
		if podIsReady(&items[i]) {
			return true, "", nil
		}
	}
	return false, fmt.Sprintf("%s: no pod is Ready", targetLabel(&t)), nil
}

// podIsReady checks the Ready condition.
func podIsReady(pod *unstructured.Unstructured) bool {
	conds, _, _ := unstructured.NestedSlice(pod.Object, "status", "conditions")
	for _, ci := range conds {
		cond, ok := ci.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == "Ready" && cond["status"] == "True" {
			return true
		}
	}
	return false
}

func (g *Grader) evalDeploymentAvailable(ctx context.Context, claimName string, c content.Check) (bool, string, error) {
	t := *c.Target
	t.APIVersion, t.Kind = "apps/v1", "Deployment"
	obj, err := g.tenant.GetObject(ctx, claimName, t)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, fmt.Sprintf("%s: not found", targetLabel(&t)), nil
		}
		return false, "", err
	}
	want := int64(c.MinAvailable)
	if want <= 0 {
		want = 1
	}
	avail, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
	if avail >= want {
		return true, "", nil
	}
	return false, fmt.Sprintf("%s: availableReplicas %d, want >= %d", targetLabel(&t), avail, want), nil
}

func (g *Grader) evalSubjectCheck(ctx context.Context, claimName string, c content.Check) (bool, string, error) {
	allowed, err := g.tenant.CanI(ctx, claimName, *c.ServiceAccount, *c.Access)
	if err != nil {
		return false, "", err
	}
	subject := fmt.Sprintf("serviceaccount %s/%s", c.ServiceAccount.Namespace, c.ServiceAccount.Name)
	action := fmt.Sprintf("%s %s", c.Access.Verb, c.Access.Resource)
	if c.Access.Namespace != "" {
		action += " in " + c.Access.Namespace
	}
	if c.Type == content.CheckSubjectCan {
		if allowed {
			return true, "", nil
		}
		return false, fmt.Sprintf("%s cannot %s (want allowed)", subject, action), nil
	}
	if !allowed {
		return true, "", nil
	}
	return false, fmt.Sprintf("%s can %s (want forbidden)", subject, action), nil
}

// predicatesMiss returns "" when the object satisfies all predicates, else a
// human-readable description of the first miss.
func predicatesMiss(obj *unstructured.Unstructured, preds []content.Predicate) string {
	for _, p := range preds {
		val, found := evalPath(obj.Object, p.Path)
		if !found {
			return fmt.Sprintf("no object with %s set", p.Path)
		}
		if p.Matches != "" {
			re, err := regexp.Compile(p.Matches)
			if err != nil || !re.MatchString(stringify(val)) {
				return fmt.Sprintf("%s is %s, want match for %q", p.Path, renderValue(val), p.Matches)
			}
			continue
		}
		if !jsonEqual(val, p.Equals) {
			return fmt.Sprintf("%s is %s, want %s", p.Path, renderValue(val), renderValue(p.Equals))
		}
	}
	return ""
}

// --- path evaluation & value comparison ---

// evalPath walks a restricted JSONPath: dot-separated map keys with optional
// [N] list indices, e.g. ".spec.rules[0].host" or ".data.db_host". This
// covers the v1 catalog; keys containing dots are unsupported (documented in
// the bundle authoring rules).
func evalPath(root map[string]interface{}, path string) (interface{}, bool) {
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return root, true
	}
	var cur interface{} = root
	for _, seg := range strings.Split(path, ".") {
		key := seg
		var idxs []int
		for strings.HasSuffix(key, "]") {
			open := strings.LastIndex(key, "[")
			if open < 0 {
				return nil, false
			}
			n, err := strconv.Atoi(key[open+1 : len(key)-1])
			if err != nil {
				return nil, false
			}
			idxs = append([]int{n}, idxs...)
			key = key[:open]
		}
		if key != "" {
			m, ok := cur.(map[string]interface{})
			if !ok {
				return nil, false
			}
			cur, ok = m[key]
			if !ok {
				return nil, false
			}
		}
		for _, n := range idxs {
			list, ok := cur.([]interface{})
			if !ok || n < 0 || n >= len(list) {
				return nil, false
			}
			cur = list[n]
		}
	}
	return cur, true
}

// jsonEqual compares two values by canonical JSON (so int64(1), float64(1)
// and YAML-decoded 1 all agree, and map key order is irrelevant).
func jsonEqual(a, b interface{}) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
}

// stringify renders a value for regex matching: strings raw, everything else
// compact JSON (arrays/maps included — bundles match on serialized form).
func stringify(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// renderValue quotes values for failure messages.
func renderValue(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

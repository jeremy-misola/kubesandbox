package content

import (
	"fmt"
	"regexp"
	"strings"
)

// idPattern keeps bundle ids DNS-label-ish and within the XRD's
// spec.starterLabRef maxLength (128).
var idPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,126}[a-z0-9])?$`)

// Lint validates the bundle against the v1 schema and returns every problem
// found (empty slice = valid). It is the SHARED validator: the CI CLI runs it
// pre-merge, and the backend's ContentStore re-runs it at watch time and
// quarantines failures instead of crashing (§4 "validation happens twice").
func (b *Bundle) Lint() []string {
	var errs []string
	add := func(format string, a ...interface{}) { errs = append(errs, fmt.Sprintf(format, a...)) }

	// Schema version gate: unknown versions are never guessed at.
	if b.APIVersion != BundleAPIVersion {
		add("apiVersion %q is not %q", b.APIVersion, BundleAPIVersion)
	}
	if b.ID == "" {
		add("id is required")
	} else if !idPattern.MatchString(b.ID) {
		add("id %q must be lowercase alphanumeric/hyphen, max 128 chars (fits spec.starterLabRef)", b.ID)
	}
	if b.Title == "" {
		add("title is required")
	}
	if b.Description == "" {
		add("description is required")
	}
	if !KnownCategories[b.Category] {
		add("category %q unknown", b.Category)
	}
	if !KnownDifficulties[b.Difficulty] {
		add("difficulty %q unknown (easy|medium|hard)", b.Difficulty)
	}
	if b.Heavy {
		// §10: v1 has no heavy bundles; the flag is reserved for the future
		// slow-path tier. Refusing it here keeps the 1-2s seed budget honest.
		add("heavy bundles are excluded from v1 (design §10)")
	}

	if len(b.Validate) == 0 {
		add("validate: at least one step is required")
	}
	stepIDs := map[string]bool{}
	for i, s := range b.Validate {
		where := fmt.Sprintf("validate[%d]", i)
		if s.ID == "" {
			add("%s: id is required", where)
		} else if stepIDs[s.ID] {
			add("%s: duplicate step id %q", where, s.ID)
		}
		stepIDs[s.ID] = true
		if s.Description == "" {
			add("%s: description is required", where)
		}
		if len(s.Checks) == 0 {
			add("%s: at least one check is required", where)
		}
		for j, c := range s.Checks {
			for _, e := range lintCheck(c) {
				add("%s.checks[%d]: %s", where, j, e)
			}
		}
	}

	if len(b.Seed) == 0 {
		add("seed: at least one manifest is required")
	}
	for _, s := range b.Seed {
		where := fmt.Sprintf("seed %s doc %d", s.File, s.Doc)
		obj := s.Object
		if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
			add("%s: apiVersion and kind are required", where)
			continue
		}
		_, namespaced, err := GVRForKind(obj.GetAPIVersion(), obj.GetKind())
		if err != nil {
			add("%s: %v", where, err)
			continue
		}
		if obj.GetName() == "" && obj.GetGenerateName() == "" {
			add("%s: metadata.name is required", where)
		}
		// Every seed object is namespaced (or is a Namespace itself): nothing
		// cluster-scoped besides Namespaces may leak into a tenant (§4).
		if namespaced {
			if obj.GetNamespace() == "" {
				add("%s: namespaced object must set metadata.namespace explicitly", where)
			}
		} else if obj.GetKind() != "Namespace" {
			add("%s: cluster-scoped kind %s is not allowed in seed manifests (only Namespace)", where, obj.GetKind())
		}
	}

	return errs
}

// SizeWarning returns a non-fatal warning when the bundle approaches the
// ConfigMap ceiling, or "" when comfortably under it.
func (b *Bundle) SizeWarning(rawBytes int) string {
	if rawBytes > SizeWarnBytes {
		return fmt.Sprintf("bundle %q is %d bytes (> %d): large payloads belong in an image or object store referenced by the manifests, not embedded in them", b.ID, rawBytes, SizeWarnBytes)
	}
	return ""
}

// lintCheck validates one check definition.
func lintCheck(c Check) []string {
	var errs []string
	add := func(format string, a ...interface{}) { errs = append(errs, fmt.Sprintf(format, a...)) }

	if !KnownCheckTypes[c.Type] {
		add("unknown check type %q", c.Type)
		return errs
	}

	needTarget := func(kindImplied string) {
		if c.Target == nil {
			add("%s: target is required", c.Type)
			return
		}
		av, kind := c.Target.APIVersion, c.Target.Kind
		if kindImplied != "" {
			// podReady/deploymentAvailable imply their kind; an explicit one
			// must agree.
			if kind == "" {
				kind = kindImplied
			}
			if kind != kindImplied {
				add("%s: target.kind must be %s", c.Type, kindImplied)
				return
			}
			if av == "" {
				if kindImplied == "Deployment" {
					av = "apps/v1"
				} else {
					av = "v1"
				}
			}
		}
		if av == "" || kind == "" {
			add("%s: target.apiVersion and target.kind are required", c.Type)
			return
		}
		if _, namespaced, err := GVRForKind(av, kind); err != nil {
			add("%s: %v", c.Type, err)
		} else if namespaced && c.Target.Namespace == "" {
			add("%s: target.namespace is required for namespaced kind %s", c.Type, kind)
		}
		if c.Target.Name != "" && c.Target.LabelSelector != "" {
			add("%s: target.name and target.labelSelector are mutually exclusive", c.Type)
		}
	}

	lintPredicate := func(where string, path string, equals interface{}, matches string) {
		if path == "" {
			add("%s: path is required", where)
		} else if !strings.HasPrefix(path, ".") {
			add("%s: path %q must start with '.'", where, path)
		}
		hasEq, hasRe := equals != nil, matches != ""
		if hasEq == hasRe {
			add("%s: exactly one of equals/matches is required", where)
		}
		if hasRe {
			if _, err := regexp.Compile(matches); err != nil {
				add("%s: invalid regex: %v", where, err)
			}
		}
	}

	switch c.Type {
	case CheckResourceExists:
		needTarget("")
		for i, p := range c.Where {
			lintPredicate(fmt.Sprintf("where[%d]", i), p.Path, p.Equals, p.Matches)
		}
	case CheckResourceAbsent:
		needTarget("")
		if len(c.Where) > 0 {
			add("resourceAbsent: where predicates are not allowed (absence is absence)")
		}
	case CheckFieldEquals:
		needTarget("")
		if c.Target != nil && c.Target.Name == "" {
			add("fieldEquals: target.name is required (field checks address one object)")
		}
		lintPredicate("fieldEquals", c.Path, c.Equals, "")
	case CheckFieldMatches:
		needTarget("")
		if c.Target != nil && c.Target.Name == "" {
			add("fieldMatches: target.name is required (field checks address one object)")
		}
		lintPredicate("fieldMatches", c.Path, nil, c.Matches)
	case CheckPodReady:
		needTarget("Pod")
	case CheckDeploymentAvailable:
		needTarget("Deployment")
		if c.MinAvailable < 0 {
			add("deploymentAvailable: minAvailable must be >= 0")
		}
	case CheckSubjectCan, CheckSubjectCannot:
		if c.ServiceAccount == nil || c.ServiceAccount.Name == "" || c.ServiceAccount.Namespace == "" {
			add("%s: serviceAccount.name and serviceAccount.namespace are required", c.Type)
		}
		if c.Access == nil || c.Access.Verb == "" || c.Access.Resource == "" {
			add("%s: access.verb and access.resource are required", c.Type)
		}
	}
	return errs
}

> **TL;DR:** Guided challenges need every session to start from
> challenge-specific state (a broken ConfigMap, a default-deny NetworkPolicy,
> etc.), but the hot pool only works because every warm member is **identical
> and ownerless** — any one can be handed to any user. Baking challenge state
> into pool members before assignment breaks that fungibility and would require
> one warm pool per challenge, which the documented control-plane ceiling
> (realistically low-teens total warm members) cannot support at any real
> content-library scale. **Resolution: don't seed the pool — seed the
> assignment.** Keep the pool exactly as it is today (generic, blank,
> fungible); add a new backend step that runs *after* a member is claimed,
> which applies the chosen challenge's manifests directly into that member's
> own vcluster API server. This requires one new capability the backend
> doesn't have today: a client that talks to a **tenant's virtual API server**,
> not just the host's.

---

## 1. The conflict

The hot pool's entire value proposition rests on uniformity: *"Every sandbox
has the same shape... Uniformity is what makes the hot pool work"*
(`README.md`). `Assign()` (`backend/internal/kubernetes/assign.go`) picks the
oldest `Ready`, unclaimed member off a single shared list — it has no concept
of "which flavor of member" because there's only ever been one flavor.

A challenge needs the opposite: specific starting state inside the vcluster
before the user sees it. If that state is baked in at *pool-warm* time, a
member is no longer generic — it's only useful to a user who picked that exact
challenge. Scaling that to a real content library (60+ challenges) means 60+
independent warm pools. The already-documented control-plane incident
(§7, `hot-pool-design.md`) found that **10 idle warm members saturate the
current 3×2-vCPU control plane**, with a realistic ceiling in the "low single
digits to low teens" *total*, across all challenges combined. Per-challenge
pools are not viable on current (or even modestly scaled) hardware.

## 2. The resolution: seed at assignment, not at warm time

Split "get a sandbox" from "prepare it for a challenge" into two phases that
already map naturally onto the existing flow:

1. **Assignment (unchanged):** `Assign()` claims a generic warm member exactly
   as it does today — metadata-only, sub-second, no knowledge of challenge
   content.
2. **Seeding (new):** immediately after claim, if the create request names a
   challenge, the backend applies that challenge's manifest bundle **into the
   claimed member's own vcluster** — not the host.

This preserves the pool untouched (`pool.go`, `warm.go` need no changes) and
confines all challenge-awareness to the assignment path and a new seeding
component.

### 2.1 The missing capability: a tenant-vcluster client

Checked `backend/internal/kubernetes/client.go` and `service.go`: today the
backend's only dynamic client (`NewDynamicClient`) is built against the
**host's** in-cluster config, used to manage claims, the pool, and host-level
composed resources. There is no code path that talks to a tenant's own virtual
API server.

The vcluster's admin kubeconfig already exists as a Secret
(`vc-{ns}-{name}-vcluster` — the same one mounted into the shell pod at
`/kubeconfig/config`, see the `shell-pod` resource in the composition). Seeding
means: fetch that Secret from the host, build a *second* dynamic client from
its kubeconfig bytes, and apply the challenge's objects through it. The same
client is the natural home for **grading** later (read live state back out of
the tenant API and compare against the challenge's expected state) — one
capability serves both seeding and validation.

### 2.2 Where the challenge selection lives

The XRD (`kubesandbox-session-xrd.yaml`) already has an unused field for
exactly this purpose:

```yaml
starterLabRef:
  type: string
  default: ""
  maxLength: 128
  description: Optional starter lab identifier or template name.
```

This was scaffolded but never wired up. It's the natural home for "which
challenge to seed" — no XRD/CRD schema migration needed, just backend logic
that reads it and acts on it during `Assign()`.

### 2.3 Cost and UX impact

Applying a handful of Kubernetes objects (ConfigMap, Deployment, NetworkPolicy,
etc.) through an API server is on the order of **1-2 seconds** — negligible
against the multi-minute cold-boot cost the hot pool already exists to avoid.
The practical UX impact is small: instead of "assign → Ready," it becomes
"assign → briefly seed → Ready," which fits naturally as a new transient phase
the frontend's existing SSE status-watching can display (e.g. "Preparing your
challenge…") before flipping to `Ready`.

**Caveat:** this only holds for scenarios that are "apply N manifests."
Anything requiring a Helm install inside the vcluster (Prometheus/Grafana,
Headlamp, OpenEBS) takes tens of seconds, not ~1-2s, and would need either its
own explicit "this one's slower" affordance in the UI, or to be excluded from
the fast default path entirely (consistent with the "heavier tier" carve-out
already recommended in `platform-limitations-and-challenges-decision.md`).

## 3. Open questions for the implementation plan

Deliberately left open here — the implementation plan should resolve these:

- **Manifest bundle format & storage.** Where do challenge manifests live
  (backend-embedded directory? ConfigMap? separate content repo?), and what's
  the schema for a "challenge" (manifests + expected-state/validation rules +
  metadata)?
- **Grading API.** On-demand (user clicks "check my work" → backend queries
  the tenant API and diffs) vs. polling/automatic? What's the response shape?
- **Partial-seed failure handling.** If seeding fails halfway through applying
  a manifest bundle, the user must not be handed a half-broken cluster.
  Needs a retry/rollback or "grab the next member and re-seed" strategyd.
- **Security review of the new client.** The backend gains a second client
  with admin access to *tenant* API servers — worth an explicit review of
  scope/blast radius even though each tenant's vcluster is already isolated
  per-session.
- **Frontend surface.** How a user picks a challenge (catalog/list endpoint),
  and how the new transient seeding phase is represented in
  `status.phase`/`status.message` or a new field.
- **Slow-path scenarios.** How Helm-installed-dependency challenges are
  flagged and handled differently from the fast manifest-only path.

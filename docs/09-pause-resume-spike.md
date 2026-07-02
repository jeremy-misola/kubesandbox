# KubeSandbox — vcluster Pause/Resume Spike Findings

**Status:** spike complete (2026-07-02) — resolves the pause/resume gate in [`08-provisioning-latency-approach.md`](./08-provisioning-latency-approach.md) §5.6/§7
**Audience:** Jeremy (platform owner)
**Related:** [`08-provisioning-latency-approach.md`](./08-provisioning-latency-approach.md) · [`01-backend-architecture.md`](./01-backend-architecture.md)

---

> **TL;DR:** Pausing a session vcluster by scaling it to zero and resuming by
> scaling back up **does not fit the 15-second ceiling** — the best resume observed
> was **73 seconds**, and even a fully optimized resume has a floor around
> 25 seconds. The reason is that a scale-up recreates the pod from scratch every
> time: it re-runs the ~33–53 s binary-copy init container and re-boots the
> control-plane API, neither of which persistence avoids. **A *paused* warm pool is
> therefore rejected for the 15 s target; the pool must be *hot* (pods kept
> running), where assignment is a metadata change and effectively instant.** The
> spike also surfaced a large, unrelated win: the production **200m CPU limit on the
> vcluster control plane inflates cold boot ~6× (196 s vs 33 s of control-plane
> boot); it should be raised regardless of pool strategy.**

---

## 1. Method

Two vclusters were provisioned directly via Crossplane `Release` objects (same
provider/config path the composition uses), each in its own namespace, to isolate
vcluster behaviour from the session/auth/route machinery. Latencies were computed
from **cluster-clock pod timestamps** (`creationTimestamp`, container `startedAt`,
and the `Ready` condition's `lastTransitionTime`) so they are not affected by tool
round-trip lag. vcluster chart **0.35.1**, image `vcluster-pro:0.35.1` running the
**k8s distro** (`ghcr.io/loft-sh/kubernetes:v1.36.0`), on node `prod-worker-3`.

Two configurations were tested:

- **Config A — production config:** control-plane CPU limit **200m**, persistence
  **disabled** (emptyDir).
- **Config B — tuned:** control-plane CPU limit **2000m**, persistence **enabled**
  (PVC on `local-path`).

Pause = scale the control-plane workload to 0; resume = scale back to 1, then
measure time to pod `Ready` (which gates on the in-vcluster API `/readyz`).

## 2. Results

| Scenario | Config | Create→Ready | of which: binary-copy init | of which: control-plane→Ready |
|---|---|---:|---:|---:|
| Cold boot | A (200m, emptyDir) | **223 s** | 24 s | 196 s |
| Cold boot | B (2000m, PVC) | **77 s** | 33 s | 33 s |
| **Resume** (scale 0→1) | B (2000m, PVC) | **73 s** | 53 s* | 17 s |

\* The resume's init container ran slower (53 s vs 33 s) due to CPU contention from
an unrelated crash-looping pod on the same node during the test; treat it as noisy
on the high side. The control-plane phase, by contrast, was **faster on resume
(17 s vs 33 s)** because the persisted etcd state on the PVC skipped
re-initialization.

## 3. What the numbers mean

**Scale-to-zero resume cannot hit 15 s.** A scale-up always creates a *new* pod
with empty ephemeral volumes, so vcluster re-runs the full startup: copy the ~Kubernetes
binaries into the pod (init container, throttled to **100m CPU** → tens of seconds),
then boot and pass readiness on the API server. Persistence helps only the last
phase (33 s → 17 s here); it does nothing for the init copy or scheduling. Adding up
the irreducible pieces even under ideal conditions — a few seconds to schedule, ~5 s
for the binary copy if its CPU cap were lifted, and ~17 s for API readiness (with
the startup probe's 6-second polling quantization on top) — lands around **25 s
minimum**. That is still outside the 15 s ceiling.

**Persistence buys little and costs interchangeability.** The only thing a PVC saved
was ~16 seconds on the API phase. Meanwhile it pins each sandbox to a specific
volume and node and adds storage lifecycle to manage. For an *unclaimed* warm
sandbox we actively want a clean cluster, so there is no state worth preserving.

**The 200m CPU limit is the single biggest lever.** Control-plane boot fell from
**196 s to 33 s** purely by raising the CPU limit from 200m to 2000m — a ~6× win on
the slowest phase. This limit lives in the composition's vcluster `Release` values
and throttles the API server exactly when it is most CPU-hungry (startup). This is
worth fixing on its own, independent of any pool work, because it also governs how
fast the pool refills.

**The binary-copy init container is the next lever.** At 100m CPU it takes ~33–53 s
on every pod start and is now a dominant term. Raising its CPU limit — or using a
control-plane image with the Kubernetes binaries already baked in — would cut both
cold boot and any restart materially.

## 4. Verdict and recommendation

- **Reject the paused (scale-to-zero) warm pool for the 15 s target.** Resume is a
  full pod re-boot; measured 73 s, floor ~25 s. It would only satisfy a minute-plus
  budget, which sign-off has ruled out.
- **Use a HOT pool: keep the pooled vclusters running.** Assignment against an
  already-Ready vcluster is a metadata change (stamp owner, start TTL), which is
  sub-second and comfortably inside 15 s. This is the design that meets the target.
- **Idle cost is affordable at the stated scale.** With ~5 arrivals/hour and a
  budget of 10 warm sandboxes ([`08`](./08-provisioning-latency-approach.md) §2.2),
  keeping ~2–3 vclusters hot is trivial headroom — the paused pool's main selling
  point (lower idle cost) isn't needed here.
- **Raise the control-plane CPU limit regardless** (from 200m). It cuts cold boot
  ~3× overall and directly speeds pool refill, which is the one sizing input that
  still matters.
- **Consider the init-container CPU limit / a binaries-baked image** as a second,
  smaller refill optimization.
- **Keep persistence disabled** (the composition's current default). It suits
  interchangeable, clean-slate pool sandboxes and avoids per-sandbox volume
  lifecycle.

## 5. Bonus: partial answer to the cold-path gate

This spike incidentally informs the *other* open gate in
[`08`](./08-provisioning-latency-approach.md) (§2.1 — "where does the 10+ minutes
go?"). The vcluster Helm install itself was **fast (~10 s to `deployed`)**, and the
control-plane cold boot is **~77 s when given CPU** (223 s when throttled). None of
that is close to 10 minutes. So the production end-to-end time is dominated **not**
by vcluster but by the layers around it — most likely Crossplane composition
reconcile/poll intervals and the serialized dependency chain (namespace → vcluster
→ kubeconfig secret → shell pod). A focused breakdown of that chain remains the
recommended next measurement.

## 6. Test hygiene

All spike resources were removed: both `Release`s (`spike-ephemeral`,
`spike-fast`) and both namespaces (`kubesandbox-spike`, `kubesandbox-spike2`,
cascading their pods, PVCs, and secrets). No production resources were touched; the
test ran in dedicated namespaces only.

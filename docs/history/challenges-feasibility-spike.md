# KubeSandbox — iximiuz-Style Challenge Feasibility Spike

**Status:** spike complete (2026-07-04) — one fix shipped (NetworkPolicy sync), rest are recommendations. Historical record.
**Audience:** Jeremy (platform owner)
**Related:** [`docs/challenges.md`](../challenges.md) (scraped iximiuz Labs scenario list) · [`kubesandbox-session-composition.yaml`](../../kubesandbox-charts/kubesandbox/charts/kubesandbox-backend/templates/kubesandbox-session-composition.yaml) · [`pause-resume-spike.md`](./pause-resume-spike.md) · [`hot-pool-design.md`](../reference/hot-pool-design.md)

> **TL;DR:** Of the 64 scraped iximiuz-style challenges, **~45 (70%) already work
> unmodified** on the current vcluster — they're pure Kubernetes-API-object
> exercises (RBAC, ConfigMaps/Secrets, Deployments, probes, `kubectl exec/cp`,
> debugging). **~13 (20%) need a small config or content change** — the biggest
> single item was that **`sync.toHost.networkPolicies` defaults to `false`** in
> the vcluster chart, so every NetworkPolicy-based challenge was a no-op against
> real traffic. **That flag is now flipped in the composition (this spike).**
> The remaining **~6 (10%) are architecturally incompatible** with a
> single-fake-node vcluster-in-a-pod: real multi-node scheduling, static pods on
> a real kubelet, and a kubeadm cluster upgrade. Getting those would mean a
> second, heavier product tier — not a vcluster tweak. Separately, this spike
> surfaced a **security gap** (no Pod Security Standard enforced anywhere in the
> stack) that's worth fixing regardless of the challenge roadmap.

---

## 1. Method

No live cluster access was used for this pass — findings are derived from:

- The actual provisioning code: `kubesandbox-session-composition.yaml` (vcluster
  Helm values, shell pod, NetworkPolicy, RBAC), `kubesandbox-backend/values.yaml`,
  `ttyd/Dockerfile`.
- Upstream vcluster documentation (v0.35 stable), fetched directly, to determine
  which sync/policy behaviors are **on by default** vs. **opt-in**, since the
  composition pins no `sync`/`policies` overrides beyond `controlPlane` CPU.
- The 64-row scraped scenario list in `docs/challenges.md`.

Because there's no chart `version:` pinned in `spec.forProvider.chart` for the
`vcluster-release` resource, defaults were checked against current stable docs;
**this should be re-verified against whatever version is actually installed on
prod-k3s** (`helm history` / the Release's `status.atProvider` would confirm).

## 2. Key finding: NetworkPolicy sync was off — now fixed

vcluster's `sync.toHost.networkPolicies.enabled` defaults to **`false`**. A user
applying a `NetworkPolicy` inside their sandbox had it accepted by the virtual
API server and **never synced to the host** — meaning it enforced nothing against
real pod-to-pod traffic. Every NetworkPolicy-based challenge (default-deny +
DNS-only egress, cross-namespace isolation, both CKA NetworkPolicy drills) would
"apply successfully" but a grading script that tests actual traffic blocking
would fail, or silently pass for the wrong reason (nothing was ever blocked).

**Fix shipped in this spike:** added `sync.toHost.networkPolicies.enabled: true`
to the `vcluster-release` values in the composition. The underlying k3s host
already enforces NetworkPolicy natively (confirmed via the existing comment in
`kubesandbox-backend/values.yaml`'s `networkPolicy.enabled` block), so this
should be sufficient — worth a live smoke test (apply a deny-all policy in a
sandbox, confirm `curl` between two pods actually fails) before relying on it for
graded content.

## 3. Security finding (independent of the challenge roadmap)

Neither the composition nor the vcluster values enable a Pod Security Standard —
`policies.podSecurityStandard` is also off by default in vcluster, and the
session namespace created by the composition carries no
`pod-security.kubernetes.io/enforce` label. The **shell pod itself** is tightly
locked down (non-root, read-only rootfs, all capabilities dropped — see the
`shell-pod` resource), but that's a property of the one pod you control, not a
guardrail on what a user's own workloads *inside* their vcluster can request.

Concretely: nothing today stops a user from creating a `privileged: true` or
`hostNetwork: true` pod inside their vcluster and having it sync down to a real
pod on the real shared k3s host. That's a container-escape / cross-tenant blast
radius risk independent of any challenge content, and I'd fix it before opening
up any scenario that even brushes against privileged/hostPath/hostNetwork
(several challenges below ask for exactly that). Suggested fix: either a
`pod-security.kubernetes.io/enforce: restricted` (or `baseline`) label on the
session namespace at the composition level, or `policies.podSecurityStandard:
restricted` in the vcluster values — whichever is easier to make bulletproof
against the vcluster's own sync path.

## 4. Full classification (64 scraped challenges)

Legend: **✅ Works today** (no changes needed) · **🔧 Needs work** (config/content,
no architecture change) · **⛔ Not feasible** (needs a different provisioning
model, or is a bad idea on shared infra).

| Challenge | Status | Notes |
|---|---|---|
| enforce-networkpolicy-to-block-all-traffic-except-dns | 🔧→✅ | Fixed by this spike's composition change |
| grant-read-only-access-to-a-developer-using-rbac | ✅ | Pure RBAC |
| secure-serviceaccount-token-mounting-using-projected-volumes | ✅ | Pure pod spec |
| configure-memory-backed-emptydir-volume-with-a-size-limit | ✅ | Pure pod spec |
| copy-file-to-running-pod | ✅ | `kubectl cp`, shell present |
| create-a-redis-statefulset-with-stable-pod-identities-storage | ✅* | *Verify default StorageClass reaches the vcluster (PVC dynamic provisioning) |
| create-an-openebs-storageclass | ⛔ | Needs a privileged DaemonSet touching real node disks — real-node problem |
| restrict-pod-traffic-using-networkpolicies-under-default-deny | 🔧→✅ | Fixed by this spike's composition change |
| schedule-pod-on-tainted-node-using-tolerations-host-namespaces | ⛔ | Needs a real, nameable node + hostNetwork/hostPID/hostIPC |
| control-cross-namespace-traffic-using-networkpolicy | 🔧→✅ | Fixed by this spike's composition change |
| resolve-service-and-external-dns-using-a-temporary-pod | ✅ | |
| debug-why-a-configmap-could-not-be-created | ✅ | |
| disable-service-account | ✅ | |
| enforce-resource-policies-using-limitrange | ✅ | |
| kubectl-convert-to-fix-deprecated-apis | 🔧 | `kubectl-convert` plugin isn't installed in the `ttyd` image today — one Dockerfile line |
| manually-schedule-deployment-pods-on-a-specific-node | ⛔ | Needs a real, nameable node |
| restrict-namespace-resources-using-resourcequota | ✅ | |
| rolling-update-and-rollback-a-deployment | ✅ | |
| set-up-hpa | 🔧 | Needs `metrics-server` installed in the vcluster (not present by default) |
| create-a-pod-with-guaranteed-qos-under-resourcequota-limits | ✅ | |
| static-pods-create-and-clean-up | ⛔ | Needs a real kubelet manifest directory on a real node |
| expose-prometheus-and-grafana-using-traefik-ingress | 🔧 | Needs an ingress-controller/exposure story per session (can reuse the HTTPRoute+ReferenceGrant pattern already built for ttyd) |
| resolve-rbac-errors-in-multiple-deployments | ✅ | |
| restrict-service-account-access-to-a-specific-secret-using-rbac | ✅ | |
| schedule-pods-using-required-pod-affinity | ✅† | †Degenerate exercise on a single fake node — everything co-locates trivially; low pedagogical value as-is |
| scheduling-pods-with-taints-affinity | ⛔ | Needs a real, distinguishable tainted node |
| troubleshoot-rbac-permissions-for-a-failing-deployment | ✅ | |
| mount-configmap-and-secret-using-a-single-volume | ✅ | |
| troubleshoot-pod-deployment-failure-due-to-resource-constraints | ✅ | |
| convert-pod-to-deployment-with-secret-management | ✅ | |
| enforce-immutable-container | ✅ | |
| kubernetes-qos-classes-and-resource-management | ✅ | Eviction-priority demo works but is soft-limited by the session's 8Gi/4-CPU ResourceQuota ceiling |
| pod-service-and-cross-namespace-dns-resolution | ✅ | |
| provision-and-configure-pv-pvc | ⛔ | Scraped description explicitly uses `hostPath` — real node filesystem access, also a shared-infra security risk |
| scale-deployment-and-expose-via-nodeport-service | 🔧 | `NodePort` object creation works; *external* reachability needs a real exposure path since you route everything via Envoy Gateway/HTTPRoute today, not raw node ports |
| secure-a-deployment-with-security-context-and-capabilities | ✅ | |
| implement-canary-deployment-strategy | ✅ | |
| application-configuration-mounted-with-configmap-and-probe-validation | ✅ | |
| bind-a-persistent-volume-to-a-specific-node-using-node-affinity | ⛔ | Needs a real, distinguishable node + local/hostPath storage |
| file-aware-readiness-gate-for-container-initialization | ✅ | |
| keep-pods-healthy-with-liveness-and-readiness-probes | ✅ | |
| extracting-and-decoding-serviceaccount-token-from-kubernetes-secret | ✅ | |
| security-contexts-in-multi-container-pods | ✅ | |
| launching-the-headlamp-kubernetes-dashboard-with-helm | 🔧 | Same external-exposure gap as the NodePort item above |
| Configure-Secrets-in-Deployment | ✅ | |
| inspecting-and-extracting-kubernetes-kubeconfig-data | ✅ | |
| using-configmap-as-environment-variables | ✅ | |
| kubernetes-invisible-pod | ✅ | Namespace/label riddle — pure API |
| kubernetes-pull-private-image | 🔧 | Needs a shared dummy private registry + credentials as seed content |
| kubernetes-pod-fundamentals | ✅ | |
| cka-kubeadm-upgrade | ⛔ | No kubeadm control plane exists to upgrade — vcluster's control plane is a pod, not a kubeadm node. Categorically incompatible, not a config gap |
| fix-go-app-container-oom | ✅ | |
| cka-network-policies-between-deployments | 🔧→✅ | Fixed by this spike's composition change |
| make-kubernetes-pod-outlive-oom-event | ✅ | |
| start-pod-with-limited-resources | ✅ | |
| copy-files-to-from-distroless-kubernetes-pod | ✅ | Ephemeral containers / `kubectl debug` — stock k8s feature |
| copy-files-to-from-kubernetes-pod | ✅ | |
| edit-file-in-running-kubernetes-pod | ✅ | |
| kubernetes-what-port-this-pod-listens-on | ✅ | |
| kubernetes-signal-container | ✅ | |
| kubernetes-signal-non-root-container | ✅ | |
| kubernetes-signal-slim-container | ✅ | |
| kubernetes-pod-with-faulty-init-sequence | ✅ | |
| kubernetes-pod-with-sleepy-init-sequence | ✅ | |

## 5. Recommended next steps

1. **Smoke-test the NetworkPolicy fix live** — apply a deny-all-except-DNS policy
   in a real sandbox, confirm cross-pod traffic is actually blocked, before
   building graded content on top of it.
2. **Close the Pod Security Standard gap** (§3) before shipping any scenario
   library — it's a cross-tenant risk today independent of challenges.
3. **Build the scenario-seeding + validation harness against the ✅ bucket
   first** (~45 challenges, zero further infra work) — this is the fastest path
   to a real content library.
4. **Second wave:** install `metrics-server` in the vcluster image/bootstrap,
   add `kubectl-convert` to the `ttyd` Dockerfile, and design one reusable
   per-session external-exposure path (generalizing the existing
   HTTPRoute/ReferenceGrant pattern) to unlock NodePort/Ingress/Headlamp-style
   challenges.
5. **Explicitly scope out** real multi-node scheduling, static pods, and
   kubeadm-upgrade content as out of bounds for the vcluster product line. If
   that content matters strategically, treat it as a separate, heavier
   "advanced sandbox" tier (real ephemeral multi-node clusters), not a config
   change to this one.

# KubeSandbox — Platform Limitations & the Challenges-Content Decision

**Status:** decision memo (2026-07-04). Historical record — revisit if the vcluster
sync/node config changes (see §2, "fixable" column).
**Audience:** Jeremy (platform owner)
**Related:** [`challenges-feasibility-spike.md`](./challenges-feasibility-spike.md) ·
[`docs/challenges.md`](../challenges.md) · [`reference/backend-architecture.md`](../reference/backend-architecture.md)

> **Decision: yes, build the challenge content — but scope it deliberately.**
> KubeSandbox cannot and should not try to be a cluster-administration trainer
> (CKA-style: kubeadm, real nodes, static pods, storage operators). Its
> architecture — one vcluster-in-a-pod per user, hot-pooled for sub-second
> handoff — structurally can't do that safely on shared infrastructure, and
> competitors who *can* do it are running real virtual machines, not shared-kernel
> sandboxes. What it *can* do, fully and correctly, is the workload/policy/
> troubleshooting layer of Kubernetes (RBAC, NetworkPolicy, ConfigMaps/Secrets,
> Deployments, probes, resource management, debugging) — which is both the
> majority of real day-2 Kubernetes work and the majority of CKAD/KCNA-style
> exam content. Lean into that, and lean into the one thing no competitor
> matches: instant, disposable, reset-in-under-a-second sandboxes for rapid
> drilling.

---

## 1. What KubeSandbox actually is, mechanically

Every session is: one `vcluster` (a virtual Kubernetes API server + control
plane) running as a pod inside a shared, small **k3s** host, handed to a user
from a **hot pool** of pre-warmed, unclaimed members. Assignment is a metadata
patch (sub-second); teardown is TTL-driven. The user gets a browser terminal
(`ttyd`) with `kubectl` pointed at their vcluster. No config beyond the
control-plane CPU shape and the NetworkPolicy sync fix (see the linked spike) is
overridden from vcluster's OSS defaults.

That single sentence — "a virtual API server sharing a small real cluster" — is
the source of almost every limitation below. It's also the source of the
speed and cost advantage: there is no real node, disk, or network to provision
per session, so handoff is instant and a session costs a fraction of a real VM.

## 2. Capability matrix: KubeSandbox vs. a real cluster vs. other platforms

| Capability | Real multi-node cluster | **KubeSandbox (today)** | iximiuz Labs | KodeKloud / Killercoda |
|---|---|---|---|---|
| Underlying tech | Real nodes (bare metal/cloud VMs) | vcluster (virtual API server) in a pod, sharing one small k3s host | Firecracker **microVMs**, up to 5 per playground, real kernel/network per VM¹ | Ephemeral containers + real VM-backed scenarios (KubeVirt etc.)² |
| RBAC, ServiceAccounts, admission | ✅ | ✅ | ✅ | ✅ |
| ConfigMaps/Secrets/Deployments/StatefulSets/probes/QoS | ✅ | ✅ | ✅ | ✅ |
| NetworkPolicy enforcement | ✅ | ✅ (fixed this session — was off) | ✅ | ✅ |
| `kubectl exec/cp/debug`, ephemeral containers, signals | ✅ | ✅ | ✅ | ✅ |
| Multiple real, nameable, taintable nodes | ✅ | ❌ (fake single node by default; real-node sync would leak shared host infra to users) | ✅ | ✅ (likely) |
| Static pods via real kubelet manifest dir | ✅ | ❌ | ✅ (real VM, real kubelet) | ✅ (likely) |
| `kubeadm` cluster bootstrap / upgrade | ✅ | ❌ — no kubeadm control plane exists to upgrade | ✅ (has a "Kubernetes the Hard Way" playground³) | ✅ (likely) |
| Storage operators needing privileged DaemonSets (OpenEBS, etc.) | ✅ | ❌ — needs real node disk access | ✅ | ✅ (likely) |
| `hostPath` volumes, `hostNetwork`/`hostPID`/`hostIPC` | ✅ | ⚠️ possible but **should be blocked** — no PSA is enforced today (see §3) | ✅ | ✅ (likely) |
| Cloud integrations (LoadBalancer Services, cloud DNS, IAM, cluster-autoscaler, GPUs) | ✅ (on real cloud) | ❌ | Partial | Varies |
| Session-to-session persistence / save-and-resume | ✅ (it's just a cluster) | ❌ — TTL teardown, no snapshot feature | Ephemeral, similar to KubeSandbox | Ephemeral, similar |
| Provisioning latency | N/A (already running) | **Sub-second** (hot pool, metadata-only assignment) | Fast (microVM boot, seconds) | Fast (containers) to slower (real VM scenarios) |
| Cost per concurrent session | High (real infra) | Low (pod-sized, shared host) | Medium (real microVM, but lightweight) | Medium–high depending on scenario type |
| Guided content / curriculum / certification prep | N/A | **None today** — raw terminal only | Yes — structured playgrounds + challenges with validation | Yes — full courses, certs, progress tracking, community |

¹ ² ³ — see sources at the bottom; these rows describe publicly documented
architecture for competitors, not internals confirmed against a live account.

## 3. The hard limitations, explained

**No real multi-node scheduling.** vcluster shows one pseudo-node by default;
turning on real node sync (`sync.fromHost.nodes`) would show users the *actual*
shared host nodes backing every other tenant's sandbox — an information leak
and a bad trade for a feature that only serves a handful of challenges.

**No `kubeadm`.** The vcluster control plane is a pod, not a kubeadm-bootstrapped
node. There is no cluster to upgrade, no static-pod manifest directory, no
`etcdctl` against a real etcd member you control end-to-end. This is not a
config gap — it's what "virtual cluster" means.

**No safe path to privileged workloads.** Neither the composition nor the
vcluster values enforce a Pod Security Standard today (flagged in the linked
spike). Until that's closed, this is a security gap, not a feature — closing it
will *reduce*, not expand, what a user can request, which is the right
direction of travel regardless of the content roadmap.

**No cloud-provider surface.** No LoadBalancer Services, no CSI drivers beyond
whatever the host's default StorageClass offers, no IAM, no GPUs. Anything
that teaches "how Kubernetes behaves on a real cloud" doesn't fit this model.

**No persistence.** Sessions are deliberately disposable (TTL + hot pool). Long
-running, stateful, "come back tomorrow" exercises aren't supported without a
new save/restore feature — which the earlier pause/resume spike found doesn't
fit the sub-second handoff goal anyway (scale-to-zero-and-resume floor was
~25s, not sub-second).

## 4. What KubeSandbox has that the others don't

**Speed nobody else matches.** The hot pool makes handoff a metadata patch, not
a boot. iximiuz's microVMs and Killercoda's real-VM scenarios are fast for
their category, but nothing in that category is *instant* in the way a
pre-warmed pod assignment is. For rapid-fire drilling — "reset and try again,"
timed exam-style rounds, live workshop cohorts spinning up dozens of sandboxes
at once — this is a genuine, structural edge, not a marketing claim.

**Lower cost per session at the scale you're likely to operate at.** A vcluster
pod is far cheaper to keep warm than a real microVM or full node, which matters
for a solo/small-team-run platform without iximiuz- or KodeKloud-scale infra
budget.

**One-per-user simplicity.** The atomic marker-based one-sandbox-per-user
enforcement and SSE-driven queue are polish competitors don't obviously have
in the same form.

## 5. Decision

**Worth it — with scope discipline.** Cross-referencing the classification in
`challenges-feasibility-spike.md`: roughly 70% of a realistic Kubernetes
challenge catalog (RBAC, policy, workloads, troubleshooting, debugging, resource
management) runs correctly on KubeSandbox as-is or with the fixes already
identified. That 70% is also, not coincidentally, the content that maps to
**CKAD/KCNA-style, application- and operator-facing** Kubernetes skills rather
than **CKA-style cluster-administration** skills — and it's the content most
relevant to how most engineers actually use Kubernetes day to day.

The remaining 30% (cluster upgrades, real node scheduling, storage operators,
static pods) is where iximiuz Labs and Killercoda have a structural advantage
because they hand out real (micro)VMs. Trying to match them there means
either accepting the security/leak trade-offs of real node sync, or rebuilding
KubeSandbox on a heavier per-session VM model — which would destroy the
sub-second hot-pool advantage that's the actual product differentiator.

**Recommendation:** build the guided-scenario/validation layer on top of the
~70% bucket, position KubeSandbox explicitly as a fast, disposable practice
environment for Kubernetes workload, policy, and troubleshooting skills — not
a cluster-admin or certification-cluster-ops trainer — and treat "real
multi-node / kubeadm / storage-operator" content as permanently out of scope
for this product, not a future roadmap item, unless a second, heavier product
tier is deliberately built for it later.

---

**Sources for competitor architecture claims (§2, §4):**
- [What are iximiuz Labs Playgrounds](https://labs.iximiuz.com/docs/playgrounds/what-are-playgrounds) — microVMs, up to 5 per playground, real kernel/network
- [Server-Side Playgrounds Reimagined (iximiuz)](https://iximiuz.com/en/posts/iximiuz-labs-playgrounds-2.0/) — Firecracker-based architecture
- [Kubernetes the Hard Way Playground — iximiuz Labs](https://labs.iximiuz.com/playgrounds/kubernetes-the-hard-way-7df4f945) — evidence of real kubeadm-style multi-node content
- [Killercoda — Interactive Environments](https://killercoda.com/about) — ephemeral containerized + real VM (KubeVirt) scenarios

# KubeSandbox — Guided Challenges: Frontend Architecture

**Status:** proposed 2026-07-05 — design pass only, no implementation yet.
Companion to [`challenges-backend-architecture.md`](./challenges-backend-architecture.md)
(backend implemented and live-verified; this is §13 item 6 of that design).
**Audience:** Jeremy (platform owner) + future maintainers
**Related:** [`../reference/frontend-architecture.md`](../reference/frontend-architecture.md) ·
[`challenges-backend-handoff.md`](./challenges-backend-handoff.md) ·
`frontend/references/design-system.md` (Luxury/Editorial tokens)

> **TL;DR:** Challenges land in the SPA as **three additive surfaces** — a
> **catalog page** (`/challenges`), a **pre-session detail view**
> (`/challenges/:id`), and a **challenge workspace** layered onto the existing
> `/terminal/:id` route, selected purely by the session payload's `challenge`
> block — built entirely from the vocabulary the frontend already has: Zod
> schemas mirroring the backend wire shapes, the typed `api` client, TanStack
> Query hooks, SSE-first updates with polling fallback, and the
> `Card`/`TerminalChrome`/`StatusBadge` component family. The one structural
> pattern is a **seeding gate** that replicates the `workspaceReady` gate
> byte-for-byte in spirit: `session.challenge && seedState !== "seeded"`
> renders the synthetic "Seeding" phase the backend already derives, over a
> skeleton that mirrors the loaded layout. Reset rides the **existing** session
> SSE stream back through Seeding — no new polling loop, no new dependencies,
> no new runtime config. A session with no challenge renders exactly as it
> does today.

---

## 1. Scope and constraints this design answers to

**In scope:** everything the backend's API surface (backend §8) exposes —
catalog browse/filter, challenge detail with progressive hints, create-with-
`challengeId` (including the queue path), the seeded-gated session workspace
(instructions + terminal + grading), on-demand grading with per-step results,
and reset. **Out of scope, matching backend v1:** completion history /
progress badges (backend §11, phase 2), hint economy/scoring (backend §14),
heavy-bundle affordances (backend §10 — the `heavy` flag is reserved but no
v1 bundle sets it).

**Constraints inherited from the existing frontend architecture:**

| Constraint | Consequence |
|---|---|
| Thin client — the claim is the source of truth | The SPA renders `seedState`/`phase` verbatim; no client-side seeding state machine. Unknown seed states fail **closed** (gate stays down). |
| Zod-validated boundary, types via `z.infer` | Every new endpoint gets a schema in `lib/schemas.ts` mirroring the Go structs exactly (incl. `omitempty` → `.optional()`). |
| Real-time by default, SSE over polling | Seeding progress and reset both ride the existing `streamSSE` session-events reader. The only polling is the existing fallback path. |
| One static image, runtime config shim | No new `VITE_*` keys — challenges use the same `/api` base and bearer plumbing. |
| Lean stack | No new dependencies. Filtering/sorting is client-side over the in-memory catalog (the backend serves it from memory; it is small by construction). |
| One sandbox per user | "Start challenge" must handle `409 session_exists` explicitly (§5.2) — the same rule the dashboard already lives with. |
| Challenges are additive | `POST /sessions` without `challengeId`, the dashboard, and the plain terminal page are behaviorally untouched. The discriminator is `session.challenge` presence, nothing else. |

## 2. Decisions

| Question | Decision |
|---|---|
| Where does the challenge session UI live? | **Extend `/terminal/:id`** — `TerminalPage` delegates to a `ChallengeWorkspace` layout when `session.challenge` is present. No new session route: the session payload is the discriminator, deep links keep working, and a challenge-less session renders the existing layout untouched. Rejected: a separate `/challenge/:sessionId` route, which would duplicate the terminal page's loading/handoff/SSE plumbing and invent a second URL for the same resource. |
| Catalog filter/sort | **Client-side**, state in **URL search params** (`?category=rbac&difficulty=medium&sort=difficulty`) so filtered views are shareable/bookmarkable. No server round-trips per filter change. |
| Seeding gate | `session.challenge && session.challenge.seedState !== "seeded"` → gate down, exactly parallel to `session && !session.workspaceReady`. Comparing against `"seeded"` (not against the busy states) means an **unknown future seed state fails closed** — same tolerance posture as `StatusBadge`'s default phase branch. |
| SSE for seeding + reset | Reuse `useSessionEvents` with a widened `enabled` condition (§7). Reset **optimistically writes `seedState: "pending"`** into the session cache so the gate re-engages and the stream re-arms instantly — then the server's SSE updates take over. No new stream, no new poll loop. |
| Grade result state | **Retained mutation state** inside `GradePanel` (TanStack `useMutation().data`), not the query cache. Grade results are ephemeral by backend design (v1 has no persistence); caching them would invent a freshness story the backend doesn't back. Cleared on reset. |
| `429 rate_limited` on grade | Prevent, then soften: the grade button self-disables for the backend's min interval (~2 s) after each submit, so 429 is rare; if it still lands (second tab), render a quiet "hold on a moment" inline state with the button cooling down — **never an error toast**. |
| `409 seed_in_progress` on grade/reset | Invalidate the session query → the fresh payload's `seedState` re-engages the gate and the workspace shows Seeding. The 409 body is informational; the claim state is the truth. |
| Hint reveal | Progressive via `GET /api/challenges/:id?hints=n`. Revealed count lives in **`sessionStorage`** keyed by challenge id (`kubesandbox.hints.<id>`), shared by the detail page and the in-session instructions panel, per-tab and gone on tab close — consistent with the token-storage posture and deliberately unsynced until phase-2 progress exists server-side. |
| `seedState` schema type | `z.string()`, not `z.enum` — the gate compares `=== "seeded"`, so new backend states degrade to "not ready yet" instead of a Zod parse failure killing the whole session payload. |
| Start with an existing sandbox | **Explicit user choice**, never silent deletion: the detail page shows "you already have a sandbox" with *Open current sandbox* and *End it & start this challenge* (reusing `ConfirmDeleteDialog`). See §5.2 for the delete-then-create race. |
| StatusBadge and Seeding | `StatusBadge.tone()` gains a challenge branch **before** the `workspaceReady` check — a seeding challenge session has `workspaceReady: true` (it's a warm member) and would otherwise show "Ready" while the gate says otherwise (§8). |

## 3. Architecture overview

```
/challenges ────────── GET /api/challenges ──────► catalog (Meta[], in-memory
     │  card click                                  filter/sort via URL params)
     ▼
/challenges/:id ────── GET /api/challenges/:id?hints=n ──► detail: description,
     │                                              steps[], hints (progressive),
     │  "Start challenge"                           difficulty/est/tags
     │
     ├─ existing sandbox? → explicit choice (open it / end it & start)
     ▼
POST /api/sessions {ttlMinutes, challengeId}
     ├─ 201 Session{challenge:{id,title,seedState:"pending"}} → /terminal/:id
     └─ 202 QueueStatus → /dashboard (QueueCard; challengeId rode the queued
                          request — backend §3 — nothing extra client-side)
     ▼
/terminal/:id (TerminalPage)
     ├─ session.challenge == null → today's layout, byte-identical
     └─ session.challenge != null → ChallengeWorkspace
            ├─ GATE: seedState !== "seeded" → Seeding panel (synthetic phase +
            │        message from the backend, dot-breathe, skeleton mirror)
            ├─ InstructionsPanel   — description, step checklist, hint reveal
            ├─ TerminalFrame       — reused as-is, zero changes
            └─ GradePanel          — POST …/grade → GradeStep[] pass/fail
                                     POST …/reset → 202 → optimistic pending
                                     → existing SSE shows Seeding → gate cycle
```

New/changed files, mirroring the existing layout:

| File | Change |
|---|---|
| `lib/schemas.ts` | + `challengeMetaSchema`, `challengeDetailSchema`, `challengeRefSchema`, `gradeResultSchema` (+`gradeStepSchema`); extend `sessionSchema` (+`challenge?`) and `createSessionRequestSchema` (+`challengeId?`). |
| `lib/api.ts` | + `listChallenges`, `getChallenge(id, hints)`, `gradeChallenge(sessionId)`, `resetChallenge(sessionId)`. No SSE changes. |
| `lib/queryClient.ts` | + `queryKeys.challenges`, `queryKeys.challenge(id, hints)`. |
| `hooks/useChallenges.ts` | new — catalog + detail queries, grade + reset mutations. |
| `hooks/useSessionEvents.ts` | unchanged file; **callers** widen the `enabled` condition (§7). |
| `pages/ChallengesPage.tsx` | new — catalog. |
| `pages/ChallengeDetailPage.tsx` | new — pre-session view + start flow. |
| `pages/TerminalPage.tsx` | + branch to `ChallengeWorkspace`; existing layout untouched for challenge-less sessions. |
| `components/ChallengeCard.tsx` | new — catalog card on the `Card` primitive. |
| `components/challenge/ChallengeWorkspace.tsx` | new — session layout (instructions + terminal + grading). |
| `components/challenge/InstructionsPanel.tsx` | new — description, `StepList`, `HintReveal`. |
| `components/challenge/GradePanel.tsx` | new — grade button, per-step results, reset. |
| `components/challenge/SeedingNotice.tsx` | new — the gate's down-state card (mirrors the provisioning card). |
| `components/StatusBadge.tsx` | + seeding/failed challenge branch in `tone()` (§8). |
| `components/SessionCard.tsx` | + small challenge title chip when `session.challenge` present (dashboard awareness, nothing more). |
| `components/Layout.tsx` / `App.tsx` | + nav link + two guarded routes. |

## 4. Schemas & API client additions

All shapes below are transcriptions of the live Go structs
(`models/session.go`, `content/bundle.go` `Meta`, `handlers/challenges.go`),
not aspirations — `omitempty` on the Go side becomes `.optional()`.

```ts
// lib/schemas.ts — additions

export const challengeMetaSchema = z.object({
  id: z.string(),
  title: z.string(),
  description: z.string(),
  category: z.string(),          // rbac | networkpolicy | workloads | config |
  difficulty: z.string(),        //   scheduling | storage-lite | troubleshooting
  // The list path (models Meta) uses omitempty, but the detail handler builds
  // a gin.H WITHOUT omitempty — so tags can arrive as null and estMinutes as 0.
  // nullish→default keeps one schema valid against both encoders.
  estMinutes: z.number().nullish().transform((n) => n ?? 0),
  tags: z.array(z.string()).nullish().transform((t) => t ?? []),
});
export type ChallengeMeta = z.infer<typeof challengeMetaSchema>;

export const challengeListSchema = z.object({
  // defensive nullable→[] like sessionListSchema
  challenges: z.array(challengeMetaSchema).nullable().transform((c) => c ?? []),
});

export const challengeStepSchema = z.object({
  id: z.string(),
  description: z.string(),
});

export const challengeDetailSchema = challengeMetaSchema.extend({
  steps: z.array(challengeStepSchema),
  hintsTotal: z.number(),
  hints: z.array(z.string()).optional(),  // present only when ?hints=n > 0
});
export type ChallengeDetail = z.infer<typeof challengeDetailSchema>;

export const challengeRefSchema = z.object({
  id: z.string(),
  title: z.string().optional(),           // Go omitempty
  seedState: z.string(),                   // pending|seeding|seeded|failed —
});                                        // string on purpose: unknown states
export type ChallengeRef = z.infer<typeof challengeRefSchema>;  // fail closed

export const gradeStepSchema = z.object({
  id: z.string(),
  description: z.string(),
  pass: z.boolean(),
  message: z.string().optional(),
});
export const gradeResultSchema = z.object({
  challengeId: z.string(),
  pass: z.boolean(),
  steps: z.array(gradeStepSchema),
  gradedAt: z.string(),
});
export type GradeResult = z.infer<typeof gradeResultSchema>;

// extended existing schemas
export const sessionSchema = z.object({
  /* …existing fields… */
  challenge: challengeRefSchema.optional(),
});
export const createSessionRequestSchema = z.object({
  ttlMinutes: z.number().int().min(TTL.min).max(TTL.max).optional(),
  challengeId: z.string().optional(),
});

/** The one derived helper the gate uses — mirrors the workspaceReady idiom. */
export const isChallengeSeeded = (s: Session) =>
  !s.challenge || s.challenge.seedState === "seeded";
```

```ts
// lib/api.ts — additions (same request() plumbing, bearer, Zod at boundary)

async listChallenges(): Promise<ChallengeMeta[]>
  // GET /challenges → challengeListSchema

async getChallenge(id: string, hints = 0): Promise<ChallengeDetail>
  // GET /challenges/:id?hints=n (param omitted when 0) → challengeDetailSchema

async gradeChallenge(sessionId: string): Promise<GradeResult>
  // POST /sessions/:id/challenge/grade → gradeResultSchema
  // throws ApiError: 409 "seed_in_progress", 429 "rate_limited",
  //                  404 "no_challenge" | "not_found" — handled in GradePanel (§9)

async resetChallenge(sessionId: string): Promise<void>
  // POST /sessions/:id/challenge/reset → 202 {status,message}; body discarded —
  // the session SSE stream is the progress channel (§10)
```

`createSession` itself is unchanged except for the widened request schema —
the 201/202 discriminated result already handles both outcomes, and the 202
queue path needs **zero** new client code because `challengeId` rides the
queued `CreateSessionRequest` server-side (backend §3, §15).

**Error-code map additions** (extends frontend-architecture §4.5): `400
invalid_request` now includes "unknown challengeId" (surfaced on the detail
page if the catalog went stale mid-click — refetch catalog + inline notice);
`404 no_challenge` (grade/reset on a plain sandbox — unreachable through the
UI, treated as a hard error); `409 seed_in_progress`; `429 rate_limited`.

## 5. Routes & pages

```
/challenges           Catalog                      (guarded)
/challenges/:id       Challenge detail / pre-start (guarded)
/terminal/:id         unchanged route; challenge-aware layout (§6.3)
```

`Layout` gains a "Challenges" nav link beside Dashboard. The dashboard itself
is untouched except `SessionCard`'s challenge chip — the catalog, not the
dashboard, is where challenge journeys start.

### 5.1 Catalog — `ChallengesPage`

`useChallenges()` → grid of `ChallengeCard`s. Filters (category, difficulty)
and sort (difficulty, estMinutes, title) are pure client-side derivations,
persisted in URL search params. UX states per the house rules
(design-principles §1/§2): skeleton grid mirroring the loaded card geometry
while loading; an empty state for "no challenges match" (distinct from "the
catalog is empty" — possible when every bundle is quarantined, backend §4);
inline error + retry on fetch failure. Phase-2 note: this page is where
completion badges will merge in (backend §11) — the card reserves a slot, no
layout rework later.

### 5.2 Detail / pre-session — `ChallengeDetailPage`

`useChallenge(id, revealedHints)` renders title, description, category/
difficulty/estMinutes/tags, and the step checklist (`steps[]` — ids +
descriptions only; the backend never exposes check internals). Hints render
as `hintsTotal` slots; "Reveal hint" bumps the sessionStorage counter, which
changes the query key → refetch with `?hints=n`. Revealing is one-way within
a tab (matches the eventual "hints cost something" economy without
committing to it).

**Start challenge** → `useCreateSession({ ttlMinutes, challengeId })`:

- **201** → navigate to `/terminal/:id`. The session arrives with
  `challenge.seedState: "pending"` and the workspace opens gated on Seeding —
  which resolves in ~1–2 s (backend §6.2), usually before the terminal iframe
  has even finished its boot overlay.
- **202** → navigate to `/dashboard`, where the existing `QueueCard` +
  `useQueueWatcher` machinery takes over untouched. On `assigned`, the session
  in the cache carries the challenge block; opening the terminal lands in the
  workspace. (An in-place queue view on the detail page is a polish item, not
  v1 — the queue is rare by design.)
- **409 `session_exists`** → the pre-flight already caught this in the common
  case: the page checks the sessions cache and, when a sandbox exists, swaps
  the Start button for the explicit choice (*Open current sandbox* / *End it &
  start this challenge*, the latter through `ConfirmDeleteDialog`). Because
  deletion is asynchronous (the `pendingDeletes` story in `useSessions`), the
  end-and-start path **waits for the delete to settle** — it reuses the same
  "server stopped returning it" signal — before issuing the create, rather
  than hammering create into 409s. The raw 409 (raced from another tab) shows
  the same choice UI.

### 5.3 Challenge session — `/terminal/:id`

No new route. See §6.3.

## 6. Components

### 6.1 Vocabulary rule

Every new component composes the existing primitives — `Card`,
`TerminalChrome`, `StatusBadge`, `Button`, the `dot-breathe` /
`cursor-blink` / `skeleton` utilities — and the Luxury/Editorial tokens
(§12). Nothing re-implements a status dot, a chrome bar, or a card frame.

### 6.2 `ChallengeCard`

`Card` with the editorial treatment: category as a tiny uppercase
wide-tracked overline, title in `font-display`, description clamped to two
lines, then a mono metadata row (difficulty · `estMinutes` min · tags).
Difficulty is a small mono uppercase chip in the `StatusBadge` visual idiom
(success/warning/danger tint for easy/medium/hard) but static — it is **not**
a `StatusBadge`, which is semantically a live session indicator. Whole card
is the link target; hover follows the house `shadow-card-hover` +
`border-accent/30` behavior already built into `Card`.

### 6.3 `ChallengeWorkspace` (inside `TerminalPage`)

`TerminalPage` keeps sole ownership of: session fetching, the loading
skeleton, the cross-origin `NewTabHandoff`, the header row (back link, title,
TTL countdown, `StatusBadge`). Its render body becomes a three-way branch:

1. `!session.challenge` → the existing provisioning-card / `TerminalFrame`
   markup, **verbatim** (moved, not modified).
2. `session.challenge && !isChallengeSeeded(session)` → workspace shell with
   the **seeding gate down** (§8).
3. seeded → the live workspace.

Workspace geometry (desktop): asymmetric two-column per the design system's
no-50/50 rule — instructions panel ~4 cols left, terminal + grade panel ~8
cols right, `TerminalFrame` reused unchanged with the same flex-fill sizing
the plain page uses; `GradePanel` sits under the terminal as a `Card`.
Mobile: stacked instructions → terminal → grading. Header title becomes the
challenge title (falls back to "Kubernetes sandbox" when `challenge.title` is
absent — it's `omitempty`).

### 6.4 `InstructionsPanel`

Description, then `StepList` — the step checklist whose rows are keyed by
step id. Crucially, **detail step ids and `GradeStep` ids are the same ids**
(both come from the bundle's `validate[]`), so the last grade result merges
into the checklist: untested → neutral dot, pass → success, fail → danger
with the failure `message` inline under the step. One list, one component,
both pages (the detail page renders it without grade state). `HintReveal`
reuses the detail page's sessionStorage counter so hints revealed pre-session
stay revealed in-session. Panel fetches `useChallenge(challenge.id, n)` —
served from the query cache when the user came through the detail page.

### 6.5 `GradePanel`

A `Card` housing: the grade button (primary), the last `GradeResult` summary
(pass/fail + `gradedAt`), and the reset affordance (secondary, with a
confirm). Per-step outcomes render in `StepList` (§6.4) rather than a second
list — the panel shows the verdict, the checklist shows the detail. Detailed
behavior in §9/§10.

### 6.6 `SeedingNotice`

The gate's down-state: same skeleton-mirrors-loaded-layout discipline as the
provisioning state (design-principles §2) — the workspace grid renders with
skeleton instruction lines and a `TerminalChrome`-topped card carrying the
`dot-breathe` warning dot and the backend's own synthetic message
(`session.message`, "Preparing your challenge…"), falling back to static copy
if absent. The terminal iframe is **not** mounted while gated — the pty would
work (authz doesn't gate on seeding, backend §6.3), but showing a live shell
into a half-seeded cluster is exactly what the gate exists to prevent.

### 6.7 `StatusBadge` extension

See §8 — a new branch, no visual system changes.

## 7. State management

New query keys and hooks, same layering as `useSessions`/`useQueue`:

```
['challenges']            catalog — staleTime generous (content changes via
                          git push, not user action); refetchOnWindowFocus ok
['challenge', id, hints]  detail — hints in the key so a reveal refetches
                          exactly once and back-navigation stays cached
```

- `useChallenges()` / `useChallenge(id, hints)` — plain queries.
- `useGradeChallenge(sessionId)` — mutation; result retained in the mutation
  state (decision §2). No cache writes: grading has no side effects on the
  session payload.
- `useResetChallenge(sessionId)` — mutation with an **optimistic seedState
  write** (§10).

**SSE enablement** — the one behavioral change to existing wiring.
`TerminalPage` currently mounts `useSessionEvents(id, !!session &&
!session.workspaceReady)`; it becomes:

```ts
useSessionEvents(id, !!session && (!session.workspaceReady || !isChallengeSeeded(session)));
```

The stream is live exactly while there is something to watch: provisioning
(as today), initial seeding, and re-seeding after reset. Once seeded, the
stream closes as it does today once ready — grading needs no stream. The
existing polling fallback inside `useSessionEvents` covers the degraded case
untouched: a dropped stream while seeding polls `GET /sessions/:id` every 5 s,
which carries `challenge.seedState` too.

**Seed failure is a session death, not a state.** Backend §6.3 fails closed
by deleting the claim and surfacing an error through the session SSE. So the
workspace treats a `deleted` event (or a 404 on poll) while gated as
"preparation failed": show a terminal error card ("This challenge couldn't be
prepared — nothing was handed over. Try again.") with a retry that routes back
through the detail page's start flow. The transient `seedState: "failed"`
annotation may or may not be observed before deletion — the UI must not
depend on seeing it (see §14).

## 8. The seeding gate & phase surfacing

The gate condition is deliberately the same *shape* as the provisioning gate
so the page reads as one idiom:

| | Provisioning gate (exists) | Seeding gate (new) |
|---|---|---|
| Condition | `session && !session.workspaceReady` | `session.challenge && seedState !== "seeded"` |
| Down-state | provisioning card, dot-breathe, live copy | `SeedingNotice`, dot-breathe, backend's synthetic message |
| Lifted by | SSE `update` flipping `workspaceReady` | SSE `update` flipping `seedState` (annotation change streams through the existing claim watch — backend §6.4) |
| Duration | minutes (cold boot) | ~1–2 s (seed budget) — the copy should not over-dramatize it |

Both gates can theoretically be down at once (queue-admitted session on a
not-quite-ready member); provisioning wins — it is the outer, longer gate,
and the seeding gate takes over when `workspaceReady` flips.

**`StatusBadge`**: `tone()` gains, *before* the `workspaceReady` early
return:

```
session.challenge && seedState === "failed"  → danger  "Seed failed"
session.challenge && seedState !== "seeded"  → warning "Seeding", dot-breathe, busy
```

Without this, a seeding challenge session shows "Ready" (warm members have
`workspaceReady: true` from the start) while the page says otherwise — a
direct contradiction in the header. The synthetic `phase: "Seeding"` string
from the backend would land in the badge's default branch anyway, but only
after the `workspaceReady` check; the explicit branch makes the precedence
correct rather than accidental. `SessionCard` on the dashboard inherits the
fix for free since it renders the same badge.

## 9. Grading UX — 409 and 429 without feeling broken

The grade button is the workspace's primary action. Submit →
`useGradeChallenge` → on success, merge `GradeResult` into `StepList` and
show the verdict (§6.5). Failure-handling doctrine: **the backend's guards
are expected states, not errors.**

- **Cooldown (429 prevention):** on every submit the button enters a ~2 s
  disabled cool-down matching the backend's min interval, with a subtle
  progress affordance (the luxury system's slow transitions make a 2 s
  disabled state feel deliberate rather than laggy). This makes 429
  effectively unreachable from a single tab.
- **429 anyway** (second tab, shared session): inline quiet text under the
  button — "Just a moment between grade runs" — and the same cooldown state.
  No toast, no red, nothing resembling failure; the user's next click works.
- **409 `seed_in_progress`:** means the claim's seed state regressed since
  render (a reset raced from elsewhere). Invalidate `['session', id]`; the
  refetched payload re-engages the seeding gate, which *is* the correct UI
  for that state. The 409 itself renders nothing.
- **Network/5xx:** standard error handling (inline + retry), unchanged from
  house style.
- **Grade pass:** `pass: true` earns the one celebratory moment in the flow —
  gold accent treatment on the verdict line, consistent with "gold is the
  reward, used sparingly." No confetti; this is a luxury product.

Steps are all-evaluated server-side (no short-circuit), so the panel always
paints the complete picture — failure messages name object + observed vs
expected, straight from the wire, no client rewriting.

## 10. Reset — riding the existing stream

`useResetChallenge(sessionId)`:

1. `POST …/challenge/reset` → `202` (body discarded).
2. `onMutate`: optimistically write the session cache with
   `challenge.seedState: "pending"`. Two things follow *from the existing
   machinery* with no further code: the seeding gate drops (workspace shows
   `SeedingNotice`), and the widened `useSessionEvents` enablement re-arms
   the SSE stream, which then carries the authoritative
   `pending → seeding → seeded` transitions as the seeder works.
3. `onError` (409 raced, 5xx): roll back the optimistic write, invalidate the
   session query, surface an inline error in `GradePanel`.
4. On the gate lifting, `GradePanel` clears its retained grade result — the
   old verdict describes a cluster that no longer exists.

The terminal iframe unmounts while the gate is down and remounts after
(`TerminalFrame` re-probes and replays its boot overlay — acceptable, and
arguably good signposting that the cluster was rebuilt underneath). Reset
requires a confirm dialog: it destroys user modifications to seeded objects.
Copy should reflect the backend caveat that unlabeled user-created objects
survive (backend §7).

No polling loop anywhere in this flow — the 202's `message` even says "watch
the session events," and that is literally what the page was already doing.

## 11. Additivity guarantees (the non-challenge flow)

Checklist this design holds itself to — each verifiable in review:

- `createSession` without `challengeId` sends the identical wire body as
  today (`challengeId` is `.optional()` and omitted, not `null`).
- `sessionSchema.parse` of a challenge-less session yields today's object
  (new field `optional`); no existing component reads `challenge` except via
  the new guarded branches.
- `TerminalPage` branch 1 (§6.3) is a code *move*, not an edit — plain
  sandboxes render byte-identical DOM.
- `StatusBadge` new branch is behind `session.challenge` presence.
- Dashboard, queue, delete, auth: zero changes beyond `SessionCard`'s
  presence-guarded chip.
- No new runtime config keys; the image promotes exactly as before.

## 12. Design-system application

Specific commitments, not vibes (tokens from `design-system.md`):

- **Catalog** is the editorial surface: generous `py` rhythm, catalog title
  in Playfair with the mixed-italic treatment (e.g. "Guided *Challenges*"),
  cards on the hairline `Card` frame, category overlines at
  `tracking-[0.25em]`. Filters are underline-style controls, not boxed
  dropdowns.
- **Difficulty chips** use the mono-uppercase micro-label idiom
  (`text-[10px]`, wide tracking) — the same type voice as `StatusBadge` and
  `TerminalChrome` titles.
- **Workspace** keeps the terminal visually dominant (the product *is* the
  terminal); instructions panel is quiet — body Inter, steps as a hairline-
  separated list, no boxed checklist. Gold appears exactly three places: hint
  reveal affordance, focus states, and the passing verdict (§9).
- **Motion:** gate transitions (seeding → live) use the slow house easing
  (`duration-500`+ `ease-luxury`); step results animate in with the same
  restrained pop `StatusBadge` uses (450 ms, `out(3)`), staggered like the
  boot lines. `prefers-reduced-motion` respected via the existing
  `prefersReducedMotion()` helper.
- **Seeding copy** stays calm ("Preparing your challenge…") — it's a 2-second
  state; no progress bars, no percentage theater. The dot-breathe is the
  activity signal, as everywhere else.

## 13. Work items (implementation order)

Each item is shippable and inert without the next, mirroring the backend
doc's ordering discipline:

1. **Schemas + client:** all §4 additions to `schemas.ts` / `api.ts` /
   `queryClient.ts`. Pure additive; existing tests keep passing.
2. **Session-shape awareness:** `sessionSchema.challenge`, `isChallengeSeeded`,
   `StatusBadge` branch, `SessionCard` chip, and the widened
   `useSessionEvents` enablement in `TerminalPage`. After this lands, a
   challenge session created *by any means* (curl, tests) already renders a
   correct badge and streams seed transitions — the observability slice.
3. **Seeding gate + workspace shell:** `TerminalPage` three-way branch,
   `ChallengeWorkspace` layout, `SeedingNotice`, `InstructionsPanel` with
   `StepList` (no grading yet). Verifiable against a live seeded session.
4. **Grading:** `GradePanel`, `useGradeChallenge`, cooldown, 409/429
   handling, step-result merge into `StepList`.
5. **Reset:** `useResetChallenge` with the optimistic pending write, confirm
   dialog, grade-result clearing, gate round-trip.
6. **Catalog + detail pages:** routes, nav link, `ChallengesPage`,
   `ChallengeCard`, filters/sort via search params, `ChallengeDetailPage`,
   hint reveal, start flow incl. the `session_exists` choice and
   delete-settle-then-create.
7. **Live verification** (mirrors backend §13 item 7, from the browser):
   catalog renders synced bundles; start → Seeding gate → workspace < 5 s;
   grade fail → fix in embedded terminal → grade pass (gold verdict); rapid
   double-grade → cooldown, no visible 429 breakage; reset → gate cycles via
   SSE only (network tab shows no polling); queue path with a challenge;
   plain sandbox flow byte-identical (DOM diff before/after); kill the SSE
   stream mid-seed → polling fallback still lifts the gate.

## 14. Open questions (deliberately deferred)

- **Seed-failure surface on the wire.** Backend §6.3 fails closed by deleting
  the claim and "surfacing a terminal error through the session SSE" — but
  the session SSE vocabulary today is `update`/`deleted`/`error`. Does a
  failed seed arrive as a distinguishable `error` payload, or only as
  `deleted`? §7's handling (treat gated-then-deleted as preparation failure)
  works either way, but a distinguishable event would let the copy say *why*.
  Needs a check against the as-built handoff doc / a live kill test rather
  than a guess here.
- **End-and-start ergonomics.** §5.2 waits for delete-settle before creating.
  If teardown routinely takes >10 s, this wants a dedicated "switching
  sandbox…" affordance or a backend-side replace semantic — decide after
  measuring real teardown latency, not before.
- **Hint reveal persistence.** sessionStorage is the v1 answer; when phase-2
  progress lands server-side (backend §11), revealed-hint state may want to
  move with it (it becomes score-relevant per backend §14's hint-economy
  question). Deferred together with that question.
- **Queue view on the detail page.** V1 bounces 202 to the dashboard's
  existing QueueCard. An in-place "you're in line for this challenge" state
  is nicer but duplicates queue UI for a rare path — revisit if queueing
  becomes common.
- **`heavy: true` affordance** (backend §10). No v1 bundles set it; when
  heavy content exists, the catalog card and SeedingNotice need the "takes
  ~1 min to prepare" treatment. The reserved schema field costs nothing now.
- **Completion badges** (backend §11 phase 2). `ChallengeCard` reserves the
  slot; the merge of `GET /api/progress` into the catalog query is designed
  there, not here.

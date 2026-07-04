# KubeSandbox Frontend — De-Vibe Redesign

Analysis and plan for taking the frontend from "well-built but costumed" to
"reads as a real product," benchmarked against the four reference sites
(Linear, Railway, Resend, Notion) and the principles in `references/`.

---

## 1. What's actually going on

The engineering here is genuinely good, and that's worth stating up front so the
redesign doesn't throw away the parts that work. The app already does the things
`design-principles.md` and `vibe-coding-fixes.md` ask for: four-state handling
(loading / error / empty / success), skeleton screens that mirror real layout,
the queue rendered as a *determinate* position instead of an infinite spinner,
optimistic delete, TanStack Query caching, visible focus rings, and
`prefers-reduced-motion` respected everywhere. None of that changes.

The problem is the **visual skin**, and it's the specific skin every vibe-coded
app converges on. The identity is described in the CSS as a "Cloud-Native
Terminal," and it lays that theme on so thick it reads as costume rather than
product.

## 2. The tells — measured against the references

The four references share a discipline this app doesn't have yet. Every one of
them leads with **real product UI** (Linear's board, Railway's canvas, Resend's
code editor, Notion's workspace), sits on **one flat background**, spends
restraint on a **single accent**, and earns attention through **typography and
spacing** rather than effects. Against that bar:

**a) The "aurora."** Two blurred radial gradient blobs drifting on 26s/34s loops,
plus a masked blueprint grid, fixed behind the whole app. This is exactly the
"floating orbs / ambient gradient / glassmorphism" pattern
`vibe-coding-fixes.md` §5 names as the generic-AI default. It's emerald instead
of purple, but it's the same genre. None of the four references have anything
like it. → **Cut entirely.**

**b) Gradient text on the hero headline.** `from-primary to-accent bg-clip-text
text-transparent`. A top-three AI tell. The references set their headlines in one
solid color and let weight and size carry them. → **Solid foreground.**

**c) Glow shadows.** Buttons carry `shadow-glow` / `shadow-glow-sm` (a colored
halo). Real products use shadows to imply elevation, not to emit light. →
**Replace with a subtle realistic shadow.**

**d) Terminal cosplay, everywhere.** Traffic-light dots on *every* card,
CRT scanlines, `$` and `❯` prompt glyphs, `~/dashboard` nav labels, and
`font-mono` + `uppercase tracking-widest` micro-labels on nearly every element.
Resend references the terminal by showing a *real code editor* — it doesn't dress
its nav and its buttons as terminal windows. The motif should mean something, not
decorate everything. → **Reserve the terminal treatment for the actual terminal
and live session state; make everything else a clean product surface.**

**e) Emoji as UI icons.** ⏳ for time-left, ⟳ / ↗ / ⤢ for controls, ✗ for errors.
Emoji render inconsistently across platforms and read as unfinished. → **Replace
with restrained inline SVG icons or plain text.**

**f) Space Grotesk as the display face.** One of the most common AI-default
display fonts. → **Consolidate (see §4).**

**g) The landing page is a one-screen stub.** A `max-w-2xl` hero, one paragraph,
a sign-in button — and nothing else. This is the single biggest gap. All four
references are content-rich pages that *prove the product exists* before asking
for anything. Right now there is zero evidence the sandbox is real. →
**Rebuild as a real, if lean, narrative page.**

## 3. Conclusion

This isn't a rewrite — the architecture and UX logic stay. It's a **reskin plus a
real landing page**. Strip the four generic-AI signatures (aurora, gradient text,
glow, emoji), demote the terminal motif from "everywhere" to "where it means
something," retune the palette and type toward the restrained reference
aesthetic, and turn the landing page into something that shows the product
instead of describing it. The bones are good; we're removing accessories, not
adding them — the Chanel rule from `frontend-design.md`.

## 4. Design tokens (the target)

Deliberately lean: two font families, one accent.

**Color** — neutralize the teal-tinted near-black to a cool graphite (closer to
Linear/Resend), and keep emerald as the *single* brand accent — desaturated a
notch so it's confident, not neon. Emerald earns its place here: green = healthy
/ ready / running is literally the product's domain (a live cluster), so it's a
choice grounded in the subject, not a default. The second accent (cyan) is
dropped — one accent is more disciplined.

```
Ink        hsl(222 14% 5%)   page background   (was teal-tinted 170 24% 4%)
Surface    hsl(222 13% 8%)   cards
Raised     hsl(220 11% 13%)  muted / hover
Line       hsl(220 10% 16%)  borders (hairline)
Text       hsl(210 14% 93%)  foreground
Muted text hsl(216 9% 60%)   secondary text
Emerald    hsl(158 64% 45%)  the one accent + "ready"
Amber      hsl(40 90% 58%)   provisioning / warning
Red        hsl(0 72% 58%)    error / destructive
```

**Type** — drop Space Grotesk. Use **Inter** for all UI and headings (weight and
size do the hierarchy work, the way Linear and Resend do it with a single sans),
and keep **JetBrains Mono** strictly for the things that are genuinely
terminal/data: session IDs, resource specs, the shell. Three families → two,
which is also one fewer webfont to load.

**Shadows** — remove `glow`/`glow-sm`. Keep one subtle `card` elevation shadow.

**Motion** — keep the disciplined, meaningful animations (skeleton shimmer,
status-badge pop on phase change, queue-position tick, terminal boot sequence).
Remove only the ambient/decorative motion (the aurora drift). Everything stays
behind `prefers-reduced-motion`.

## 5. Landing page — new structure

Lead with the product, prove it's real, keep it short. Roughly:

```
┌ header (clean: mark + wordmark + Sign in) ───────────────┐
│                                                          │
│  HERO                                                    │
│   Ephemeral Kubernetes sandboxes,                        │
│   ready in seconds.                                      │
│   [ Sign in to get started ]  via SSO                    │
│                                                          │
│   ┌── honest product visual ──────────────────────┐     │
│   │  a real terminal / session card, not a blob    │     │
│   └────────────────────────────────────────────────┘     │
│                                                          │
│  HOW IT WORKS  (a real 3-step sequence → numbering       │
│   01 Create  →  02 Work in the browser  →  03 Auto-clean │
│                 is justified here; it's an ordered flow)  │
│                                                          │
│  FEATURES (concrete, not generic icon-cards)             │
│   · Isolated private vcluster (real spec: cpu/mem)       │
│   · Browser terminal, nothing to install                 │
│   · TTL you set — it deletes itself, costs nothing idle  │
│   · Warm pool — handed over, not cold-booted             │
│                                                          │
│  footer                                                  │
└──────────────────────────────────────────────────────────┘
```

Copy gets rewritten in plain English per `frontend-design.md`: say what it does,
cut the launch-vocabulary. The 3-step numbering is kept because the content
genuinely *is* a sequence (create → use → expire); numbered markers are only
justified when order carries real information, and here it does.

## 6. Scope of the first implementation pass

Highest-impact, lowest-risk changes, in order:

1. **Tokens** — `index.css` + `tailwind.config.js`: repalette, drop aurora +
   glow, swap fonts.
2. **Chrome** — `Layout.tsx`: remove the aurora layer, clean the header/footer.
3. **Button** — drop glow, keep the rest.
4. **Landing** — rebuild `LandingPage.tsx` per §5, remove gradient text.
5. **De-emoji** — swap glyphs for small SVG icons / text in SessionCard,
   TerminalFrame, TerminalPage.
6. **Verify** — `npm run typecheck` + `npm run build`.

What we deliberately **don't** touch: the query/auth/SSE layers, the four-state
logic, the skeletons, the terminal auth-popup dance, routing. Those are the good
bones.

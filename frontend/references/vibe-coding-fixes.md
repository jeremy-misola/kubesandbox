# De-Vibe-Coding Principles

Fixes that take an AI-generated app or landing page from "hackathon project" to "something people actually want to use." Pairs with `design-principles.md`.

## 1. Loading: Skeletons, Not Spinners

### The Fix
Replace loading spinners with skeleton screens — gray placeholder bars/boxes that mirror the actual layout before real data arrives.

### Why
The user's brain processes the page structure while data is still loading, so the wait feels shorter even though it isn't.

### Example
YouTube, Instagram, and LinkedIn all show gray placeholder boxes shaped like thumbnails/cards before content loads — never a bare spinner for full-page or feed loads.

## 2. Caching

### The Fix
Cache fetched data so it loads once and persists across navigation, instead of refetching on every click.

### Why
Refetching on every page visit makes an app feel like dial-up internet — full-second (or longer) delays every time the user clicks around, even for data that hasn't changed.

### Rule of Thumb
If a user has already seen the data this session, don't make them wait for it again. Revalidate in the background if freshness matters, but show the cached version instantly.

## 3. Optimistic Rendering

### The Fix
For actions that succeed the vast majority of the time (likes, toggles, form saves), update the UI immediately — don't wait for server confirmation.

### Why
Waiting for a round-trip on a near-certain action feels sluggish. Assuming success and rolling back on failure feels instant.

### Rule
Only use this when: (1) success rate is very high, and (2) rollback is safe and visible if the rare failure happens. Don't use it for irreversible or high-stakes actions (payments, deletions).

## 4. Tooltips on Icon-Only Buttons

### The Fix
Any button that's just an icon (no label) needs a hover tooltip explaining what it does.

### Why
Icon-only UI looks clean but communicates nothing on its own. If even the person who built the app doesn't remember what an icon button does, users have no chance.

### Rule
Every icon-only control gets a short, literal tooltip ("Delete," "Duplicate," "Share") — not a clever label, just the action.

## 5. Visual De-Vibe-Coding (Landing Pages & UI Chrome)

### Remove Generic AI Aesthetic
- **No purple gradients or glassmorphism.** These read as "generic AI app" because every vibe-coded product defaults to them. Replace with one flat background color plus a single accent color.
- **No floating orbs, fake dashboards, or fake testimonials.** If an element exists only because it looked cool in a Tailwind/component demo, cut it. Nothing decorative that doesn't serve a real purpose.

### Fix Typography & Spacing
- Pick **one** real font family (not a default AI pairing), set proper line height, and fix cramped/off spacing. Cramped, inconsistent spacing is one of the biggest tells that AI generated the layout.

### Fix Copy
- Rewrite all copy in plain English. Cut inflated launch-copy vocabulary: "rocket," "sparkles," "empower," "unleash," "revolutionize," and similar. Say what the product actually does.

### Prove the Product Is Real
- Replace generic three-feature-card grids with a real screenshot, a real demo, or a real number. Three icon-cards in a row with placeholder copy signals "template," not "working product." Anything that proves the product exists beats anything that just describes it.

## 6. Why This Matters Together

Any one of these fixes alone is a nice-to-have. Applied together, they're the difference between an app/page that reads as a weekend AI experiment and one that reads as a real product — the polish compounds.

# Changelog

All notable changes to the KubeSandbox frontend are documented here. Format
loosely follows [Keep a Changelog](https://keepachangelog.com/); this project
does not yet publish versioned releases.

## [Unreleased]

### Changed — Luxury / Editorial redesign

Reskinned the entire app to a Luxury/Editorial design system while keeping the
existing HSL design-token contract, so component color classes were preserved
rather than rewritten.

- **Design tokens** (`tailwind.config.js`, `src/index.css`): one variable
  contract, two palettes. `.theme-light` renders warm alabaster paper with
  charcoal ink for editorial surfaces; `.dark` is the system's inverted charcoal
  variant for the app interior. Gold (`#D4AF37`) is the single accent, used only
  for emphasis, hover, and focus. Type pairing is Playfair Display (headlines) +
  Inter (UI) + JetBrains Mono (terminal). Border radius is squared everywhere
  except functional status dots; shadows are soft and layered; motion uses a
  cinematic `ease-luxury` curve.
- **Theme routing** (`Layout.tsx`): the theme now follows the route — the public
  landing renders light, the dashboard/terminal/dialogs stay dark and legible.
  Added the editorial substrate: fixed vertical gridlines and a paper-grain
  noise overlay.
- **Primitives**: `Button` gained the signature gold-layer-slides-from-left
  primary, fill-on-hover secondary, uppercase wide tracking, and gold focus
  rings. `Card` uses a soft-shadow lift with an optional `featured` gold top
  rule; `TerminalChrome` was squared.
- **Landing page**: rebuilt as an editorial showpiece — asymmetric hero, an
  extreme Playfair headline with italic-gold emphasis, a decorative overline
  rule, a drop-cap intro, large serif step numerals, vertical labels, and a
  slower load stagger.
- **App interior**: Dashboard, SessionCard, QueueCard, StatusBadge, both
  dialogs, TerminalPage, TerminalFrame, CallbackPage, and NotFoundPage moved to
  serif headings, uppercase tracked labels, squared surfaces, and gold accents
  (replacing the previous emerald accent).

Accessibility was preserved: motion is still gated by `prefers-reduced-motion`,
focus indicators remain, and semantic status colors are retained.

### Removed — leanness pass

- Dropped unused design tokens introduced during the redesign: the
  `--paper` / `--ink` / `--gold` CSS constants, the `shadow-feature` and
  `serif` font-family aliases, the `transitionDuration` 1500/2000 extensions,
  and the unused `fadeIn` keyframe/animation.
- Added `dist_test/` to `.gitignore` so the stale secondary build output is no
  longer tracked. (The existing directory should be deleted manually — it is
  owned outside the sandbox and could not be removed automatically.)

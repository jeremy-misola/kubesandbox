# UX/UI Design Principles

## 1. Loading States

### Four States Every Screen Needs
Every screen in your app must handle four states: **loading**, **success**, **error**, and **empty**. Neglecting any of these degrades user experience.

### Time Thresholds for Loaders
| Duration | Appropriate Treatment |
|---|---|
| < 1 second | Show results immediately — a loader flashing briefly feels jarring and makes it feel slower |
| 1–5 seconds | Plain spinner (no text) is acceptable |
| 5–6 seconds | Spinner with static text ("Loading...", "Saving...") buys ~1 more second of patience |
| 6–10 seconds | Spinner with changing / sequential text ("Connecting to your account..." → "Almost there...") — users wait significantly longer when they perceive progress, even if the text is purely cosmetic |
| > 10 seconds | Looped animations stop working and increase frustration. Switch to a progress bar, step-by-step indicator, or other determinate loader |

### When an Error Occurs
Show the error immediately as soon as you can detect it. Do NOT keep a loading spinner running for 20 seconds and then display a failure.

### History of Loaders
- 1984 Mac: wristwatch cursor (no animation)
- 1985 Windows: hourglass (no animation)
- Late 1980s: first progress bars — 86% of users preferred them even when inaccurate
- 2001 Mac OS X: spinning rainbow wheel (animated but indeterminate)
- 2013: skeleton screens introduced (Facebook early adopter)

## 2. Choosing the Right Loader

### Skeleton Screens
- **When to use:** Entire page or large section loading (feeds, dashboards, profiles)
- **Why:** The user's brain processes the layout before data arrives, making it feel faster
- **Example:** Instagram, LinkedIn, YouTube feeds

### Progress Bars
- **When to use:** You know (or can estimate) how long something will take
- **Use cases:** File uploads, downloads, installations, multi-step processes
- **Why:** Gives users a sense of how far along they are and remaining time
- **Note:** Showing a spinner on a file upload may make users assume it's stuck

### Inline Spinners
- **When to use:** Small, contained actions
- **Use cases:** A button just clicked, a single section of a page refreshing

### Optimistic UI
- **When to use:** Actions where success is highly likely and rollback is safe
- **How it works:** Show the success state immediately (before server confirmation), then roll back if it fails
- **Why:** Feels instant to the user
- **Example:** Instagram like — heart turns red immediately, reverts if the server call fails

## 3. Error Placement

### Toast Notifications
- **Use for:** Non-critical, transient messages
- **Characteristics:** Pop up at top or bottom of screen, auto-dismiss after a few seconds
- **Test:** If the user looks away and misses it, will they be okay? If not, use a different approach
- **Example:** "Couldn't connect, retrying"

### Modal Errors
- **Use for:** Critical errors that block progress
- **Characteristics:** Takes over the center of the screen, blocks everything until user responds
- **Rule:** If you block the user, you MUST give them a way forward (action button)
- **Examples:**
  - Payment failure → "Your payment did not process" + "Update payment" button
  - Permission error → "You don't have access to this project" + "Request access" button

### Inline Errors
- **Use for:** Most common error scenario
- **Characteristics:** Appears right next to the element that went wrong
- **General rule:** The closer the error is to the issue, the better
- **Examples:**
  - Form field with red border + correction message next to submit button
  - "Save" button fails → "Try again" displayed right next to the button

## 4. Error Messages

### What to Avoid
- **Raw database or backend logic dumped on screen:** Confusing to users and a security vulnerability (exposes app internals)
- **"Something went wrong" alone:** Too vague — user has no idea what happened (e.g., if a payment fails, did the payment go through or not?)

### Three Components of a Good Error Message
1. **What happened** — "Your payment didn't go through"
2. **Why it happened** — "Your card was declined"
3. **A clear action** — "Please check your card details or try a different payment method"

### Silent Failures (Worst Case)
The user clicks submit, the button does nothing, the screen doesn't change. No message at all. The user has no idea if it worked or broke. AI-generated code commonly produces this by default — must be explicitly guarded against.

## 5. Forms

### 1. Disable Submit Until Valid
- Keep the submit button disabled (grayed out) until all required fields are completed correctly
- But make it OBVIOUS what's missing — a grayed-out button with no explanation is more frustrating than no validation at all
- Mark required fields clearly

### 2. Validate Inline (as the user types / leaves a field)
- Validate the moment the user leaves a field (e.g., email field → check it's a real email address)
- Avoid: user fills out entire form, submits, waits for it to load, then has to scroll back up to fix one thing

### 3. Show Character Count
- If a field has a character limit, display remaining characters as the user types
- Don't let someone write a full paragraph only to discover they must cut it in half

### 4. Pre-fill What You Can
- If the user is logged in, don't make them re-type their email

### 5. Show Password Requirements as They Type
- Display requirements (capital letter, number, length, etc.) and check them off as each is satisfied
- Don't let someone submit and then get rejected for a missing capital letter

### 6. Be Forgiving with Formatting
- Accept phone numbers with dashes, parentheses, spaces, or nothing at all
- Normalize formatting on the backend instead of forcing a specific input format on the user

## 6. Empty States

### First Impressions
The empty state is often the first thing a user sees. Make it good.

### Rules for Empty States
1. **Tell the user why it's empty**
2. **Show them what to do next** (give them an action)
3. **Don't make it feel broken**

### Patterns

**Empty dashboard (no projects yet):**
- Bad: Blank screen, no information, no actions
- Good: "Create your first project" with a call-to-action button
- Better: Step-by-step gamified instructions to get started

**Every section of the app:**
- Don't leave blank sections with no explanation
- Tell the user what the section is for and how to start using it

**Empty search results:**
- "No results" is acceptable but weak
- Better: "No results for 'purble shoes'. Did you mean 'purple shoes'?" with a link to search that term
- Keeps the user moving forward

**Goal-state empty (e.g., zero inbox):**
- Make it feel like an achievement
- Add a nice animation, a celebratory background
- Give the user something they look forward to seeing

## 7. Partial States & Graceful Degradation

### The Problem
A single page loads data from multiple independent sources (profile picture server, feed server, sidebar, charts). Each source loads at a different speed. If one fails, should everything fail?

### The Principle
Load what's available. Each section loads, fails, and displays independently.

### Implementation
- Don't show a loading screen until every single component is ready
- Don't show an entire-page error because one component failed
- Each component manages its own loading, success, error, and empty states
### Examples
- Instagram: stories might be ready while the feed is still loading
- Dashboard: sidebar loads fine, charts are still loading independently


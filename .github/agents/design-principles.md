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

### Analogy
Ordering delivery from three different restaurants — you don't wait for all three to arrive before eating any, and if one doesn't show up, you don't throw all the food away.

### Examples
- Instagram: stories might be ready while the feed is still loading
- Dashboard: sidebar loads fine, charts are still loading independently

## 8. Animations & Visual Effects

The following libraries are cloned locally and can be used directly without internet access.

---

### 8.1 anime.js — JavaScript Animation Library (35+ Techniques)

**Local path:** `useful-repos-for-frontend/anime/`

A fast, multipurpose JavaScript animation library. Works with CSS properties, SVG, DOM attributes and JavaScript Objects.

**Install:**
```
npm install animejs
```

**Import (ES Module):**
```javascript
import { animate, createTimeline, stagger, svg, utils } from 'animejs';
```

**Key features:**
- Animate CSS properties, SVG, DOM attributes, and JS Objects
- Timeline support for sequenced animations
- Built-in easing functions and staggering
- Draggable interactivity
- Responsive scopes (scroll-driven animations)
- Canvas 2D support with additive blending
- Auto-layout animations (accordion, cards, nav, periodic table, planets, todo-list)
- Text effects (hover, scramble, split)
- SVG line drawing with `svg.createDrawable()`
- Easing visualizer
- Irregular playback / typewriter effects
- Layered CSS transforms

**Local examples directory:** `useful-repos-for-frontend/anime/examples/`

#### 8.1.1 SVG Line Drawing — `svg.createDrawable()`

**File:** `useful-repos-for-frontend/anime/examples/svg-line-drawing/index.js`

<details>
<summary>View code</summary>

```javascript
import { svg, createTimeline, stagger, utils } from 'animejs';

function generateLines(numberOfLines) {
  const svgWidth = 1100;
  const svgHeight = 1100;
  const margin = 50;
  const spacing = (svgWidth - margin * 2) / (numberOfLines - 1);
  let svgContent = `<svg width="${svgWidth}px" height="${svgHeight}px" viewBox="0 0 ${svgWidth} ${svgHeight}">
      <g id="lines" fill="none" fill-rule="evenodd">`;
  for (let i = 0; i < numberOfLines; i++) {
    const x = margin + i * spacing;
    svgContent += `<line x1="${x}" y1="${margin}" x2="${x}" y2="${svgHeight - margin}" class="line-v" stroke="#A4FF4F"></line>`;
  }
  svgContent += `</g></svg>`;
  return svgContent;
}

function generateCircles(numberOfCircles) {
  const svgWidth = 1100;
  const svgHeight = 1100;
  const centerX = svgWidth / 2;
  const centerY = svgHeight / 2;
  const maxRadius = 500;
  const step = maxRadius / numberOfCircles;
  let svgContent = `<svg width="${svgWidth}px" height="${svgHeight}px" viewBox="0 0 ${svgWidth} ${svgHeight}">
      <g id="circles" fill="none" fill-rule="evenodd">`;
  for (let i = 0; i < numberOfCircles; i++) {
    const radius = (i + 1) * step;
    svgContent += `<circle class="circle" stroke="#A4FF4F" stroke-linecap="round" stroke-linejoin="round" stroke-width="10" cx="${centerX}" cy="${centerY}" r="${radius}"></circle>`;
  }
  svgContent += `</g></svg>`;
  return svgContent;
}

const svgLines = generateLines(100);
const svgCircles = generateCircles(50);
document.body.innerHTML += svgLines;
document.body.innerHTML += svgCircles;

createTimeline({ loop: 0, defaults: { ease: 'inOut(4)', duration: 10000, loop: true } })
  .add(svg.createDrawable('.line-v'), {
    draw: ['.5 .5', () => { const l = utils.random(.05, .45, 2); return `${.5 - l} ${.5 + l}` }, '0.5 0.5'],
    stroke: '#FF4B4B',
  }, stagger([0, 8000], { start: 0, from: 'first' }))
  .add(svg.createDrawable('.circle'), {
    draw: [
      () => { const v = utils.random(-1, -.5, 2); return `${v} ${v}` },
      () => `${utils.random(0, .25, 2)} ${utils.random(.5, .75, 2)}`,
      () => { const v = utils.random(1, 1.5, 2); return `${v} ${v}` },
    ],
    stroke: '#FF4B4B',
  }, stagger([0, 8000], { start: 0 }))
  .init();
```
</details>

**Key technique:** `svg.createDrawable()` wraps SVG path-like elements (lines, circles, rects) into drawable targets. The `draw` property accepts `[start, mid, end]` draw progress values (as decimal fractions `0–1` or percentages `'0% 100%'`). Combined with `stagger()`, each element draws in sequence.

---

#### 8.1.2 Staggered Grid Animations — `stagger()`

**File:** `useful-repos-for-frontend/anime/examples/stagger/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const totalColors = 14;
const totalDots = 1000;
const w = window.innerWidth;
const h = window.innerHeight;

for (let i = 0; i < totalDots; i++) {
  const el = document.createElement('div');
  el.classList.add('dot');
  el.dataset.color = String(utils.random(0, totalColors - 1));
  document.body.appendChild(el);
}

const dots = document.querySelectorAll('.dot');

utils.set(dots, {
  x: () => utils.random(0, w - 16),
  y: () => utils.random(0, h - 16),
  rotate: () => utils.random(-180, 180),
  scale: () => utils.random(.2, 2, 3),
});

createTimeline({ composition: false })
  .add(dots, {
    scale: [{ from: '-=1', to: '+=2' }],
    rotate: [{ from: '-=180', to: '+=180' }],
    background: [{ from: '#FFF' }],
    duration: 1000,
    ease: 'inOut(3)',
    loop: true,
  }, stagger([0, 2000], { grid: true, from: 'center', axis: 'x' }))
  .init();
```
</details>

**Key technique:** `stagger([startDelay, endDelay], options)` creates delays between elements. The `grid: true` option computes nearest-neighbor grid positioning. `from: 'center'` makes the animation ripple outward from the center. `axis: 'x'` staggers along the x-axis only. `stagger()` works inline in the timeline's third argument (the offset parameter).

---

#### 8.1.3 Timeline Playback Controls

**File:** `useful-repos-for-frontend/anime/examples/clock-playback-controls/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, animate, utils } from 'animejs';

// DOM references
const clockEl = document.querySelector('.clock');
const secHand = clockEl.querySelector('.seconds-hand');
const minHand = clockEl.querySelector('.minutes-hand');

// Build a timeline for the clock hands
const tl = createTimeline({ defaults: { duration: 1000, ease: 'linear' }, loop: true })
  .add(secHand, { rotate: 360 }, 0)
  .add(minHand, { rotate: 6 }, 0)
  .seek(0)
  .init();

// Playback control buttons
document.querySelector('#play').addEventListener('click', () => tl.play());
document.querySelector('#pause').addEventListener('click', () => tl.pause());
document.querySelector('#stop').addEventListener('click', () => tl.seek(0).pause());

// Speed control
document.querySelector('#speed').addEventListener('input', (e) => {
  tl.speed = parseFloat(e.target.value);
});

// Seek slider
document.querySelector('#seek').addEventListener('input', (e) => {
  tl.seek(parseFloat(e.target.value));
});
```
</details>

**Key technique:** `createTimeline()` returns a timeline object with `.play()`, `.pause()`, `.seek()`, `.speed` (getter/setter) methods. Set `tl.speed = 2` for 2x playback. `.seek(progress)` jumps to a specific point in the timeline (0–1 or absolute time in ms). The `loop: true` option makes the timeline repeat indefinitely.

---

#### 8.1.4 SVG Graph Animations

**File:** `useful-repos-for-frontend/anime/examples/svg-graph/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const bars = document.querySelectorAll('.bar');

// Animate bars growing from bottom
createTimeline({ defaults: { ease: 'out(4)', duration: 800 }, loop: true })
  .add(bars, {
    height: [
      { from: 0 },
      () => utils.random(50, 300),
      { from: '-=1', to: '+=1' },
    ],
    y: [
      { from: 400 },
      (_, i) => 400 - bars[i].offsetHeight,
      { from: '-=1', to: '+=1' },
    ],
    background: ['#4F8BFF', '#A4FF4F', '#FF4B4B'],
  }, stagger(40, { from: 'center' }))
  .init();
```
</details>

**Key technique:** Animate bar chart elements by manipulating `height` and `y` simultaneously (to simulate growing from bottom). `stagger(40, { from: 'center' })` makes bars grow outward from the center. Use function-based values `() => utils.random(min, max)` for varied heights each loop.

---

#### 8.1.5 Text Animations

##### 8.1.5.1 Hover Effects

**File:** `useful-repos-for-frontend/anime/examples/text/hover-effects/index.js`

<details>
<summary>View code</summary>

```javascript
import { animate, stagger, utils } from 'animejs';

const letters = document.querySelectorAll('.char');

letters.forEach((letter) => {
  letter.addEventListener('mouseenter', () => {
    animate(letter, {
      scale: 1.5,
      color: '#FF4B4B',
      duration: 300,
      ease: 'out(4)',
    });
  });

  letter.addEventListener('mouseleave', () => {
    animate(letter, {
      scale: 1,
      color: '#FFF',
      duration: 300,
      ease: 'out(4)',
    });
  });
});

// Or using a single stagger on all chars on hover container
const container = document.querySelector('.text-container');
container.addEventListener('mouseenter', () => {
  animate(letters, {
    scale: 1.4,
    color: '#A4FF4F',
    duration: 400,
    delay: stagger(30),
    ease: 'out(3)',
  });
});

container.addEventListener('mouseleave', () => {
  animate(letters, {
    scale: 1,
    color: '#FFF',
    duration: 400,
    delay: stagger(30, { from: 'last' }),
    ease: 'out(3)',
  });
});
```
</details>

**Key technique:** Use `stagger()` as a `delay` value to create ripple effects on hover. `stagger(30, { from: 'last' })` reverses the stagger direction for the leave animation. Each letter is wrapped in a `.char` span element.

##### 8.1.5.2 Scramble Text

**File:** `useful-repos-for-frontend/anime/examples/text/scramble/index.js`

<details>
<summary>View code</summary>

```javascript
import { animate } from 'animejs';

const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+';
const el = document.querySelector('.scramble-text');
const originalText = el.textContent;

function scramble() {
  let iterations = 0;
  const interval = setInterval(() => {
    el.textContent = el.textContent
      .split('')
      .map((char, index) => {
        if (index < iterations) return originalText[index];
        return chars[Math.floor(Math.random() * chars.length)];
      })
      .join('');
    if (iterations >= originalText.length) clearInterval(interval);
    iterations += 1 / 3;
  }, 30);
}

// Trigger scramble on click or hover
el.addEventListener('mouseenter', scramble);
```
</details>

**Key technique:** Replace characters with random ones from a charset, revealing the original text progressively from left to right. The `iterations` counter increments by `1/3` to create a smooth reveal of ~3 characters per tick.

##### 8.1.5.3 Scramble with Timeline

**File:** `useful-repos-for-frontend/anime/examples/text/scramble-tl/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const el = document.querySelector('.text');
const chars = el.textContent.split('');
el.innerHTML = chars.map(c => `<span class="char">${c}</span>`).join('');
const charEls = document.querySelectorAll('.char');

createTimeline({ loop: true })
  .add(charEls, {
    opacity: [0, 1],
    translateY: [20, 0],
    duration: 400,
    ease: 'out(3)',
  }, stagger(25))
  .add(charEls, {
    opacity: [1, 0],
    translateY: [0, -20],
    duration: 400,
    ease: 'in(3)',
  }, stagger(25, { start: 800 }))
  .init();
```
</details>

**Key technique:** Split text into `<span class="char">` elements, then animate each character's opacity and Y position with stagger on both the enter and exit phases. Uses staggered start offsets to chain enter → exit.

##### 8.1.5.4 Split Effects

**File:** `useful-repos-for-frontend/anime/examples/text/split-effects/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const text = document.querySelector('.split-text');
const chars = text.textContent.split('');
text.innerHTML = chars.map(c => `<span class="char">${c}</span>`).join('');
const charEls = document.querySelectorAll('.char');

createTimeline({ defaults: { duration: 600, ease: 'out(4)' }, loop: true })
  .add(charEls, {
    rotate: () => utils.random(-90, 90),
    translateX: () => utils.random(-100, 100),
    translateY: () => utils.random(-100, 100),
    opacity: [1, 0],
    scale: [1, 0],
  }, stagger(20))
  .add(charEls, {
    rotate: 0,
    translateX: 0,
    translateY: 0,
    opacity: [0, 1],
    scale: [0, 1],
  }, stagger(20, { start: 600 }))
  .init();
```
</details>

**Key technique:** Each character gets randomized rotation, translation, and scale values, creating an explosive split effect. The second `.add()` call reverses the effect, bringing characters back to their original positions.

##### 8.1.5.5 Split Playground

**File:** `useful-repos-for-frontend/anime/examples/text/split-playground/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const textEl = document.querySelector('.playground-text');
const originText = textEl.textContent;

function splitText() {
  const chars = originText.split('');
  textEl.innerHTML = chars.map(c => `<span class="char">${c}</span>`).join('');
  return document.querySelectorAll('.char');
}

let chars = splitText();

function animateSplit(type) {
  createTimeline()
    .add(chars, {
      translateY: { from: 0 },
      rotate: type === 'explode' ? () => utils.random(-360, 360) : 0,
      scale: type === 'explode' ? [1, 0] : [1, 1.5],
      opacity: type === 'fade' ? [1, 0] : 1,
      duration: 500,
      ease: 'out(4)',
    }, stagger(30))
    .init();
}

document.querySelector('#explode').addEventListener('click', () => animateSplit('explode'));
document.querySelector('#fade').addEventListener('click', () => animateSplit('fade'));
document.querySelector('#scale').addEventListener('click', () => animateSplit('scale'));
document.querySelector('#reset').addEventListener('click', () => {
  textEl.innerHTML = originText;
  chars = splitText();
});
```
</details>

**Key technique:** Interactive playground where different animation modes (explode, fade, scale) are applied to the same split-text structure. The `splitText()` helper re-wraps characters into spans whenever needed.

---

#### 8.1.6 Scroll-Driven Animations

##### 8.1.6.1 Responsive Scope

**File:** `useful-repos-for-frontend/anime/examples/onscroll-responsive-scope/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const cards = document.querySelectorAll('.card');

// Each card responds to its own scroll position within the viewport
cards.forEach((card, i) => {
  createTimeline({
    scope: card,
    defaults: { duration: 600, ease: 'out(4)' },
  })
    .add(card.querySelector('.card-content'), {
      scale: [0.8, 1],
      opacity: [0, 1],
      translateY: [40, 0],
    }, 0)
    .init();
});
```
</details>

**Key technique:** The `scope` option limits the timeline's scroll detection to a specific element. As the element enters the viewport, the animation plays. Each card manages its own independent animation based on its visibility.

##### 8.1.6.2 Sticky Scroll

**File:** `useful-repos-for-frontend/anime/examples/onscroll-sticky/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, utils } from 'animejs';

const container = document.querySelector('.sticky-container');
const cards = container.querySelectorAll('.card');

const tl = createTimeline({
  scroll: container,
  defaults: { duration: 1000, ease: 'out(4)' },
})
  .add(cards, {
    scale: [0.9, 1],
    opacity: [0, 1],
    translateY: [60, 0],
  }, (_, i) => i * 250)
  .init();
```
</details>

**Key technique:** `scroll: container` links the timeline's progress to the container's scroll position. Each card animates into view as the user scrolls through the sticky container. The offset function `(_, i) => i * 250` staggers the starts at scroll-based intervals.

---

#### 8.1.7 Draggable Carousels

##### 8.1.7.1 Infinite Auto Carousel

**File:** `useful-repos-for-frontend/anime/examples/draggable-infinite-auto-carousel/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, animate, utils } from 'animejs';

const track = document.querySelector('.carousel-track');
const slides = track.querySelectorAll('.slide');
const slideWidth = slides[0].offsetWidth;
const totalSlides = slides.length;

// Auto-scroll
let autoScroll = animate(track, {
  translateX: [0, -(slideWidth * (totalSlides - 1))],
  duration: 5000,
  ease: 'linear',
  loop: true,
});

// Draggable overrides
let isDragging = false;
let startX = 0;
let currentTranslate = 0;

track.addEventListener('mousedown', (e) => {
  isDragging = true;
  startX = e.clientX;
  autoScroll.pause();
});

document.addEventListener('mousemove', (e) => {
  if (!isDragging) return;
  const diff = e.clientX - startX;
  track.style.transform = `translateX(${currentTranslate + diff}px)`;
});

document.addEventListener('mouseup', () => {
  if (!isDragging) return;
  isDragging = false;
  currentTranslate = parseInt(track.style.transform.replace('translateX(', '').replace('px)', ''));
  // Snap to nearest slide
  const snapIndex = Math.round(-currentTranslate / slideWidth);
  animate(track, {
    translateX: -(snapIndex * slideWidth),
    duration: 300,
    ease: 'out(3)',
  });
  autoScroll.play();
});
```
</details>

**Key technique:** Combine `animate()` for auto-scrolling with manual drag handling. On drag end, snap to the nearest slide index using `Math.round(-currentTranslate / slideWidth)`. Pause auto-scroll during drag, resume after snap.

##### 8.1.7.2 Mouse Scroll Snap Carousel

**File:** `useful-repos-for-frontend/anime/examples/draggable-mouse-scroll-snap-carousel/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, animate, utils } from 'animejs';

const carousel = document.querySelector('.carousel');
const slides = carousel.querySelectorAll('.slide');
const slideWidth = slides[0].offsetWidth + 16; // includes gap
let currentIndex = 0;
let isAnimating = false;

function goToSlide(index) {
  if (isAnimating) return;
  isAnimating = true;
  currentIndex = Math.max(0, Math.min(index, slides.length - 1));
  animate(carousel, {
    translateX: -(currentIndex * slideWidth),
    duration: 400,
    ease: 'out(4)',
    onComplete: () => { isAnimating = false; },
  });
}

// Mouse wheel scroll snap
carousel.addEventListener('wheel', (e) => {
  e.preventDefault();
  if (e.deltaY > 0) goToSlide(currentIndex + 1);
  else goToSlide(currentIndex - 1);
}, { passive: false });

// Drag to scroll snap
let startX, scrollLeft, isDown = false;
carousel.addEventListener('mousedown', (e) => {
  isDown = true;
  startX = e.pageX - carousel.offsetLeft;
  scrollLeft = carousel.scrollLeft;
});
carousel.addEventListener('mouseleave', () => { isDown = false; });
carousel.addEventListener('mouseup', () => { isDown = false; });
carousel.addEventListener('mousemove', (e) => {
  if (!isDown) return;
  e.preventDefault();
  const x = e.pageX - carousel.offsetLeft;
  const walk = (x - startX) * 2;
  const slideDelta = Math.round(walk / slideWidth);
  if (slideDelta !== 0) {
    goToSlide(currentIndex - slideDelta);
    isDown = false;
  }
});
```
</details>

**Key technique:** Scroll snap carousel with both wheel and drag support. The `goToSlide()` function snaps to a specific slide index with a smooth `out(4)` easing. A guard flag `isAnimating` prevents overlapping animations.

##### 8.1.7.3 Draggable Playground

**File:** `useful-repos-for-frontend/anime/examples/draggable-playground/index.js`

<details>
<summary>View code</summary>

```javascript
import { animate, utils } from 'animejs';

const draggable = document.querySelector('.draggable');

let isDragging = false;
let offsetX, offsetY;

draggable.addEventListener('mousedown', (e) => {
  isDragging = true;
  const rect = draggable.getBoundingClientRect();
  offsetX = e.clientX - rect.left;
  offsetY = e.clientY - rect.top;
  draggable.style.cursor = 'grabbing';
});

document.addEventListener('mousemove', (e) => {
  if (!isDragging) return;
  animate(draggable, {
    x: e.clientX - offsetX - draggable.parentElement.getBoundingClientRect().left,
    y: e.clientY - offsetY - draggable.parentElement.getBoundingClientRect().top,
    duration: 0, // instant
  });
});

document.addEventListener('mouseup', () => {
  isDragging = false;
  draggable.style.cursor = 'grab';
});
```
</details>

**Key technique:** Use `animate()` with `duration: 0` for instant position updates during drag. The cursor changes to `grabbing` while dragging. Calculate offset to prevent the element from snapping to the cursor center on mousedown.

---

#### 8.1.8 Canvas 2D Animations

**File:** `useful-repos-for-frontend/anime/examples/canvas-2d/index.js`

<details>
<summary>View code</summary>

```javascript
import { animate, utils } from 'animejs';

const canvas = document.querySelector('canvas');
const ctx = canvas.getContext('2d');
canvas.width = window.innerWidth;
canvas.height = window.innerHeight;

const particles = Array.from({ length: 200 }, () => ({
  x: utils.random(0, canvas.width),
  y: utils.random(0, canvas.height),
  radius: utils.random(2, 6),
  color: `hsl(${utils.random(0, 360)}, 80%, 60%)`,
  vx: utils.random(-2, 2),
  vy: utils.random(-2, 2),
}));

function draw() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  particles.forEach(p => {
    p.x += p.vx;
    p.y += p.vy;
    if (p.x < 0 || p.x > canvas.width) p.vx *= -1;
    if (p.y < 0 || p.y > canvas.height) p.vy *= -1;
    ctx.beginPath();
    ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2);
    ctx.fillStyle = p.color;
    ctx.fill();
  });
  requestAnimationFrame(draw);
}

draw();

// Animate canvas particles with anime
animate(particles, {
  radius: [2, 8],
  vx: [() => utils.random(-3, 3)],
  vy: [() => utils.random(-3, 3)],
  duration: 2000,
  loop: true,
  ease: 'inOut(3)',
  onUpdate: () => draw(),
});
```
</details>

**Key technique:** anime.js can animate JavaScript Objects (like particle arrays) directly. Use `onUpdate` to trigger canvas re-draws. The `loop: true` option makes the particle properties oscillate.

---

#### 8.1.9 Additive Blending Effects

##### 8.1.9.1 Creature

**File:** `useful-repos-for-frontend/anime/examples/additive-creature/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const canvas = document.querySelector('canvas');
const ctx = canvas.getContext('2d');
canvas.width = window.innerWidth;
canvas.height = window.innerHeight;
ctx.globalCompositeOperation = 'lighter'; // additive blending

const tentacles = 12;
const segments = 30;
const points = [];

for (let i = 0; i < tentacles; i++) {
  const angle = (i / tentacles) * Math.PI * 2;
  for (let j = 0; j < segments; j++) {
    points.push({
      x: canvas.width / 2,
      y: canvas.height / 2,
      angle,
      radius: j * 15,
      wobble: utils.random(0, Math.PI * 2),
    });
  }
}

createTimeline({ loop: true, defaults: { duration: 2000, ease: 'inOut(3)' } })
  .add(points, {
    wobble: [`+=${Math.PI * 2}`],
    radius: [() => utils.random(5, 30)],
  }, stagger(10))
  .init();
```
</details>

**Key technique:** Set `ctx.globalCompositeOperation = 'lighter'` for additive blending — overlapping shapes brighten each other, creating glowing organic effects. The creature effect uses sine waves from each point's angle + wobble to calculate positions.

##### 8.1.9.2 Fireflies

**File:** `useful-repos-for-frontend/anime/examples/additive-fireflies/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const canvas = document.querySelector('canvas');
const ctx = canvas.getContext('2d');
canvas.width = window.innerWidth;
canvas.height = window.innerHeight;
ctx.globalCompositeOperation = 'lighter';

const fireflies = Array.from({ length: 100 }, () => ({
  x: utils.random(0, canvas.width),
  y: utils.random(0, canvas.height),
  size: utils.random(2, 6),
  alpha: utils.random(0.1, 1),
  speed: utils.random(0.5, 2),
}));

createTimeline({ loop: true, defaults: { duration: 3000, ease: 'inOut(2)' } })
  .add(fireflies, {
    alpha: [0, 1, 0],
    size: [() => utils.random(2, 10)],
    x: [() => utils.random(0, canvas.width)],
    y: [() => utils.random(0, canvas.height)],
  }, stagger([0, 2000]))
  .init();
```
</details>

**Key technique:** Fireflies use additive blending with alpha pulsing (0 → 1 → 0). Each firefly independently moves to a new random position each loop cycle. The stagger `[0, 2000]` spreads firefly movements across the 2-second window.

---

#### 8.1.10 Advanced Grid Staggering

**File:** `useful-repos-for-frontend/anime/examples/advanced-grid-staggering/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const grid = document.querySelector('.grid');
const cells = [];

// Create a 20x20 grid
for (let row = 0; row < 20; row++) {
  for (let col = 0; col < 20; col++) {
    const cell = document.createElement('div');
    cell.classList.add('cell');
    cell.style.width = '30px';
    cell.style.height = '30px';
    cell.dataset.row = row;
    cell.dataset.col = col;
    grid.appendChild(cell);
    cells.push(cell);
  }
}

createTimeline({ defaults: { duration: 600, ease: 'out(4)' }, loop: true })
  // Wave from center
  .add(cells, {
    scale: [0.5, 1.5, 0.5],
    background: ['#1a1a2e', '#4F8BFF', '#1a1a2e'],
    borderRadius: ['50%', '10%', '50%'],
  }, stagger(15, { grid: [20, 20], from: 'center', axis: 'both' }))
  .add(cells, {
    scale: [0.5, 1.5, 0.5],
    background: ['#1a1a2e', '#FF4B4B', '#1a1a2e'],
    borderRadius: ['50%', '10%', '50%'],
  }, stagger(15, { grid: [20, 20], from: 'first', axis: 'x' }))
  .init();
```
</details>

**Key technique:** The `grid: [cols, rows]` option tells stagger the grid dimensions for proper spatial delay calculation. `from` supports `'center'`, `'first'`, `'last'`, `'edges'`, `'topLeft'`, etc. `axis` can be `'x'`, `'y'`, or `'both'`.

---

#### 8.1.11 Animatable Follow Cursor

**File:** `useful-repos-for-frontend/anime/examples/animatable-follow-cursor/index.js`

<details>
<summary>View code</summary>

```javascript
import { animate, utils } from 'animejs';

const trail = [];
const trailLength = 20;

for (let i = 0; i < trailLength; i++) {
  const dot = document.createElement('div');
  dot.classList.add('trail-dot');
  dot.style.width = `${20 - i}px`;
  dot.style.height = `${20 - i}px`;
  document.body.appendChild(dot);
  trail.push(dot);
}

let mouseX = 0;
let mouseY = 0;

document.addEventListener('mousemove', (e) => {
  mouseX = e.clientX;
  mouseY = e.clientY;
});

// Each frame, move dots toward the previous dot's position
function updateTrail() {
  trail.forEach((dot, i) => {
    const targetX = i === 0 ? mouseX : parseFloat(trail[i - 1].style.left);
    const targetY = i === 0 ? mouseY : parseFloat(trail[i - 1].style.top);
    const currentX = parseFloat(dot.style.left) || mouseX;
    const currentY = parseFloat(dot.style.top) || mouseY;

    dot.style.left = `${currentX + (targetX - currentX) * 0.3}px`;
    dot.style.top = `${currentY + (targetY - currentY) * 0.3}px`;
  });
  requestAnimationFrame(updateTrail);
}

updateTrail();
```
</details>

**Key technique:** Each dot follows the one before it with a lerp factor of `0.3`, creating a smooth trailing effect. The first dot follows the mouse directly, subsequent dots follow their predecessor.

---

#### 8.1.12 Auto Layout Patterns

##### 8.1.12.1 Accordion

**File:** `useful-repos-for-frontend/anime/examples/auto-layout/accordion/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, utils } from 'animejs';

const accordion = document.querySelector('.accordion');
const accordionItems = accordion.querySelectorAll('.accordion-item');

accordionItems.forEach(item => {
  const header = item.querySelector('.accordion-header');
  const content = item.querySelector('.accordion-content');

  header.addEventListener('click', () => {
    const isOpen = item.classList.contains('open');

    // Close all
    accordionItems.forEach(i => {
      i.classList.remove('open');
      animate(i.querySelector('.accordion-content'), {
        height: 0,
        opacity: 0,
        duration: 300,
        ease: 'out(3)',
      });
    });

    if (!isOpen) {
      item.classList.add('open');
      animate(content, {
        height: content.scrollHeight,
        opacity: 1,
        duration: 300,
        ease: 'out(3)',
      });
    }
  });
});
```
</details>

**Key technique:** Use `content.scrollHeight` as the animation target for accordion expand. Animate `height: 0` and `opacity: 0` to collapse. Close all items first, then open the clicked one.

##### 8.1.12.2 Cards

**File:** `useful-repos-for-frontend/anime/examples/auto-layout/cards/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const cards = document.querySelectorAll('.card');

createTimeline({ defaults: { duration: 500, ease: 'out(4)' } })
  .add(cards, {
    opacity: [0, 1],
    translateY: [40, 0],
    scale: [0.95, 1],
  }, stagger(60))
  .init();
```
</details>

**Key technique:** Card enter animation with staggered opacity, Y translation, and scale. Cards fade in from below one by one with a 60ms stagger.

##### 8.1.12.3 Code

**File:** `useful-repos-for-frontend/anime/examples/auto-layout/code/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const lines = document.querySelectorAll('.code-line');

// Typewriter-style code reveal
createTimeline({ defaults: { duration: 30, ease: 'linear' } })
  .add(lines, {
    width: ['0%', '100%'],
    opacity: [0, 1],
  }, stagger(100))
  .init();
```
</details>

**Key technique:** Code lines animate width from 0% to 100% with a staggered typewriter effect. Each line waits 100ms before starting its reveal.

##### 8.1.12.4 Navigation

**File:** `useful-repos-for-frontend/anime/examples/auto-layout/nav/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const navItems = document.querySelectorAll('.nav-item');

createTimeline({ defaults: { duration: 400, ease: 'out(4)' } })
  .add(navItems, {
    translateY: [-30, 0],
    opacity: [0, 1],
  }, stagger(50, { from: 'first' }))
  .init();
```
</details>

**Key technique:** Nav items drop in from above. Stagger starts from the first item (`from: 'first'`), creating a natural left-to-right reveal.

##### 8.1.12.5 Onscroll

**File:** `useful-repos-for-frontend/anime/examples/auto-layout/onscroll/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const sections = document.querySelectorAll('.section');

sections.forEach(section => {
  createTimeline({
    scope: section,
    defaults: { duration: 600, ease: 'out(4)' },
  })
    .add(section.querySelectorAll('.animate-in'), {
      opacity: [0, 1],
      translateY: [30, 0],
    }, stagger(40))
    .init();
});
```
</details>

**Key technique:** Each section creates its own timeline scoped to itself. As the section scrolls into view, its child elements animate in with stagger.

##### 8.1.12.6 Periodic Table

**File:** `useful-repos-for-frontend/anime/examples/auto-layout/periodic-table/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const elements = document.querySelectorAll('.element');

createTimeline({ defaults: { duration: 800, ease: 'out(4)' }, loop: true })
  .add(elements, {
    scale: [0, 1],
    rotate: [-180, 0],
    opacity: [0, 1],
  }, stagger(20, { grid: [18, 7], from: 'topLeft' }))
  .init();
```
</details>

**Key technique:** The `grid: [18, 7]` matches the periodic table layout. `from: 'topLeft'` makes elements appear diagonally from the top-left corner, cascading across the table.

##### 8.1.12.7 Planets

**File:** `useful-repos-for-frontend/anime/examples/auto-layout/planets/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const planets = document.querySelectorAll('.planet');

createTimeline({ defaults: { duration: 1500, ease: 'inOut(2)' }, loop: true })
  .add(planets, {
    translateX: [() => utils.random(-200, 200)],
    translateY: [() => utils.random(-200, 200)],
    scale: [() => utils.random(0.5, 2)],
  }, stagger(100))
  .init();
```
</details>

**Key technique:** Planets orbit/spread with random translate values each loop. The `inOut(2)` easing creates smooth back-and-forth motion.

##### 8.1.12.8 Todo List

**File:** `useful-repos-for-frontend/anime/examples/auto-layout/todo-list/index.js`

<details>
<summary>View code</summary>

```javascript
import { animate, utils } from 'animejs';

const list = document.querySelector('.todo-list');
const input = document.querySelector('.todo-input');
const addBtn = document.querySelector('.add-btn');

function addTodo(text) {
  const item = document.createElement('div');
  item.classList.add('todo-item');
  item.innerHTML = `
    <span class="todo-text">${text}</span>
    <button class="delete-btn">×</button>
  `;
  list.appendChild(item);

  animate(item, {
    translateX: [-50, 0],
    opacity: [0, 1],
    duration: 300,
    ease: 'out(3)',
  });

  item.querySelector('.delete-btn').addEventListener('click', () => {
    animate(item, {
      translateX: 50,
      opacity: 0,
      height: 0,
      duration: 300,
      ease: 'in(3)',
      onComplete: () => item.remove(),
    });
  });
}

addBtn.addEventListener('click', () => {
  const text = input.value.trim();
  if (text) { addTodo(text); input.value = ''; }
});
```
</details>

**Key technique:** Items slide in from the left when added. On delete, items slide out to the right, fade, and collapse height. The `onComplete` callback removes the element from the DOM after the exit animation finishes.

---

#### 8.1.13 Easing Visualizer

**File:** `useful-repos-for-frontend/anime/examples/easings-visualizer/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const easings = [
  'in(2)', 'out(2)', 'inOut(2)',
  'in(4)', 'out(4)', 'inOut(4)',
  'linear', 'outBounce', 'outElastic(1, .5)',
];

const container = document.querySelector('.easings-grid');

easings.forEach(easing => {
  const row = document.createElement('div');
  row.classList.add('easing-row');
  row.innerHTML = `
    <span class="easing-name">${easing}</span>
    <div class="easing-track">
      <div class="easing-ball"></div>
    </div>
  `;
  container.appendChild(row);

  createTimeline({ defaults: { duration: 1500, ease: easing }, loop: true })
    .add(row.querySelector('.easing-ball'), {
      translateX: ['0%', '100%'],
    }, 0)
    .init();
});
```
</details>

**Key technique:** Dynamically create rows for each easing function. A ball translates across the track using the specified easing, visually demonstrating the acceleration curve.

---

#### 8.1.14 Irregular Playback / Typewriter

**File:** `useful-repos-for-frontend/anime/examples/irregular-playback-typewriter/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const text = 'Hello, this is a typewriter effect with irregular timing...';
const container = document.querySelector('.typewriter');
const chars = text.split('').map(c => {
  const span = document.createElement('span');
  span.textContent = c;
  span.style.opacity = '0';
  container.appendChild(span);
  return span;
});

// Irregular timing: each character has a random delay
const delays = chars.map(() => utils.random(20, 120));

createTimeline({ defaults: { duration: 50, ease: 'linear' } })
  .add(chars, {
    opacity: [0, 1],
  }, (_, i) => delays[i])
  .init();
```
</details>

**Key technique:** Instead of uniform stagger, each character gets a random delay via a function-based offset `(_, i) => delays[i]`. This creates an organic, irregular typing rhythm rather than a mechanical one.

---

#### 8.1.15 Layered CSS Transforms

**File:** `useful-repos-for-frontend/anime/examples/layered-css-transforms/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const layers = document.querySelectorAll('.layer');

createTimeline({ defaults: { duration: 2000, ease: 'inOut(3)' }, loop: true })
  .add(layers, {
    translateX: [() => utils.random(-100, 100)],
    translateY: [() => utils.random(-100, 100)],
    rotate: [() => utils.random(-30, 30)],
    scale: [() => utils.random(0.8, 1.2)],
    opacity: [() => utils.random(0.5, 1)],
  }, stagger(200))
  .init();
```
</details>

**Key technique:** Multiple layers animate with different translate, rotate, scale, and opacity values, creating a parallax-like depth effect. Each layer moves independently, with staggered start times.

---

#### 8.1.16 Logo Animation

**File:** `useful-repos-for-frontend/anime/examples/animejs-v4-logo-animation/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const logoPaths = document.querySelectorAll('.logo path');

createTimeline({ defaults: { duration: 1200, ease: 'out(4)' }, loop: true })
  .add(logoPaths, {
    draw: ['0% 0%', '0% 100%'],
    stroke: ['#4F8BFF', '#A4FF4F', '#FF4B4B'],
    strokeWidth: [0, 3, 0],
  }, stagger(50))
  .init();
```
</details>

**Key technique:** SVG logo paths are drawn in sequence using `draw` and staggered. The stroke color cycles through a palette. This technique works for any SVG logo with `<path>` elements.

---

#### 8.1.17 Timeline Patterns

##### 8.1.17.1 50K Stars (Performance Test)

**File:** `useful-repos-for-frontend/anime/examples/timeline-50K-stars/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const canvas = document.querySelector('canvas');
const ctx = canvas.getContext('2d');
canvas.width = window.innerWidth;
canvas.height = window.innerHeight;

const stars = Array.from({ length: 50000 }, () => ({
  x: utils.random(0, canvas.width),
  y: utils.random(0, canvas.height),
  size: utils.random(0.5, 2),
  alpha: utils.random(0.1, 1),
}));

createTimeline({ defaults: { duration: 3000, ease: 'inOut(2)' }, loop: true })
  .add(stars, {
    alpha: [0.1, 1, 0.1],
    size: [() => utils.random(0.5, 3)],
    x: [() => utils.random(0, canvas.width)],
    y: [() => utils.random(0, canvas.height)],
  }, stagger([0, 3000], { from: 'random' }))
  .init();
```
</details>

**Key technique:** 50,000 objects animated simultaneously — demonstrates anime.js v4's performance with canvas. `stagger([0, 3000], { from: 'random' })` distributes start times randomly across the 3-second window.

##### 8.1.17.2 Refresh Starlings (Murmuration)

**File:** `useful-repos-for-frontend/anime/examples/timeline-refresh-starlings/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const canvas = document.querySelector('canvas');
const ctx = canvas.getContext('2d');
canvas.width = window.innerWidth;
canvas.height = window.innerHeight;

const birds = Array.from({ length: 500 }, (_, i) => ({
  x: canvas.width / 2,
  y: canvas.height / 2,
  angle: (i / 500) * Math.PI * 2,
  radius: i * 0.5,
  speed: utils.random(0.5, 2),
}));

createTimeline({ loop: true })
  .add(birds, {
    radius: [() => utils.random(50, 300)],
    angle: [`+=${Math.PI * 2}`],
    speed: [() => utils.random(0.5, 3)],
    duration: 4000,
    ease: 'inOut(2)',
  }, stagger(10))
  .init();
```
</details>

**Key technique:** Starling murmuration simulation. Birds orbit around a center point with varying radii and speeds. The `angle` animation uses `+=` to accumulate rotation over time.

##### 8.1.17.3 Seamless Loop

**File:** `useful-repos-for-frontend/anime/examples/timeline-seamless-loop/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const els = document.querySelectorAll('.loop-item');

// Seamless loop: values wrap around smoothly
createTimeline({
  defaults: { duration: 2000, ease: 'linear' },
  loop: true,
})
  .add(els, {
    rotate: [{ from: 0, to: 360 }],
    translateX: [{ from: '-=100', to: '+=100' }],
  }, stagger(100))
  .init();
```
</details>

**Key technique:** Using `{ from: value, to: value }` syntax creates explicit from/to ranges. Combined with `loop: true` and `linear` easing, properties animate continuously in a seamless loop.

##### 8.1.17.4 Stress Test

**File:** `useful-repos-for-frontend/anime/examples/timeline-stress-test/index.js`

<details>
<summary>View code</summary>

```javascript
import { createTimeline, stagger, utils } from 'animejs';

const container = document.querySelector('.stress-container');
const count = 2000;
const items = [];

for (let i = 0; i < count; i++) {
  const el = document.createElement('div');
  el.classList.add('stress-item');
  el.style.left = `${utils.random(0, window.innerWidth - 10)}px`;
  el.style.top = `${utils.random(0, window.innerHeight - 10)}px`;
  container.appendChild(el);
  items.push(el);
}

createTimeline({ defaults: { duration: 1000, ease: 'inOut(3)' }, loop: true })
  .add(items, {
    scale: [0.5, 1.5],
    opacity: [0.2, 1],
    background: ['#FF4B4B', '#4F8BFF', '#A4FF4F'],
  }, stagger([0, 2000], { from: 'random' }))
  .init();
```
</details>

**Key technique:** Stress test with 2000 DOM elements. `stagger([0, 2000], { from: 'random' })` randomizes start times to prevent all elements from animating simultaneously. Tests animation performance at scale.

---

### 8.2 react-three-fiber — ThreeJS Renderer for React (27 Demo Techniques)

**Local path:** `useful-repos-for-frontend/react-three-fiber/`

A React renderer for ThreeJS. Build 3D scenes declaratively with re-usable, self-contained React components.

**Install:**
```
npm install three @types/three @react-three/fiber
```

**Import and basic usage:**
```tsx
import { createRoot } from 'react-dom/client'
import { Canvas, useFrame } from '@react-three/fiber'
import { useRef, useState } from 'react'

function Box(props) {
  const ref = useRef()
  const [hovered, hover] = useState(false)
  const [clicked, click] = useState(false)
  useFrame((state, delta) => (ref.current.rotation.x += delta))
  return (
    <mesh
      {...props}
      ref={ref}
      scale={clicked ? 1.5 : 1}
      onClick={() => click(!clicked)}
      onPointerOver={() => hover(true)}
      onPointerOut={() => hover(false)}
    >
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial color={hovered ? 'hotpink' : 'orange'} />
    </mesh>
  )
}

createRoot(document.getElementById('root')).render(
  <Canvas>
    <ambientLight intensity={Math.PI / 2} />
    <spotLight position={[10, 10, 10]} angle={0.15} penumbra={1} decay={0} intensity={Math.PI} />
    <pointLight position={[-10, -10, -10]} decay={0} intensity={Math.PI} />
    <Box position={[-1.2, 0, 0]} />
    <Box position={[1.2, 0, 0]} />
  </Canvas>
)
```

**Local examples directory:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/`

#### 8.2.1 Click & Hover Interactions

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/ClickAndHover.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef, useState } from 'react'
import { Canvas, useFrame, ThreeEvent } from '@react-three/fiber'

function ClickAndHoverBox() {
  const meshRef = useRef<THREE.Mesh>(null!)
  const [hovered, setHovered] = useState(false)
  const [clicked, setClicked] = useState(false)

  useFrame((state, delta) => {
    meshRef.current.rotation.y += delta * 0.5
  })

  return (
    <mesh
      ref={meshRef}
      onClick={(e: ThreeEvent<MouseEvent>) => {
        e.stopPropagation()
        setClicked(!clicked)
      }}
      onPointerOver={(e) => {
        e.stopPropagation()
        setHovered(true)
      }}
      onPointerOut={(e) => {
        e.stopPropagation()
        setHovered(false)
      }}
      scale={clicked ? 1.5 : 1}
    >
      <boxGeometry args={[2, 2, 2]} />
      <meshStandardMaterial color={hovered ? '#A4FF4F' : '#4F8BFF'} />
    </mesh>
  )
}

export default function ClickAndHover() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <ambientLight intensity={0.5} />
      <pointLight position={[10, 10, 10]} intensity={1} />
      <ClickAndHoverBox />
    </Canvas>
  )
}
```
</details>

**Key technique:** Use `onPointerOver` / `onPointerOut` for hover detection and `onClick` for click events. `e.stopPropagation()` prevents event bubbling through nested objects. `useFrame()` subscribes to the render loop for continuous rotation.

---

#### 8.2.2 GLTF Model Loading

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Gltf.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { Suspense } from 'react'
import { Canvas } from '@react-three/fiber'
import { useLoader } from '@react-three/fiber'
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader'
import { OrbitControls } from '@react-three/drei'

function Model() {
  const gltf = useLoader(GLTFLoader, '/models/scene.gltf')
  return <primitive object={gltf.scene} scale={0.5} />
}

export default function Gltf() {
  return (
    <Canvas camera={{ position: [0, 2, 5] }}>
      <ambientLight intensity={0.5} />
      <directionalLight position={[5, 5, 5]} />
      <Suspense fallback={<mesh><boxGeometry args={[1, 1, 1]} /><meshStandardMaterial color="gray" /></mesh>}>
        <Model />
      </Suspense>
      <OrbitControls />
    </Canvas>
  )
}
```
</details>

**Key technique:** `useLoader(GLTFLoader, url)` loads 3D models. Wrap in `<Suspense>` with a `fallback` (e.g., a simple box or spinner) while the model loads. Use `<primitive object={gltf.scene}>` to insert the loaded scene into the R3F tree.

---

#### 8.2.3 Pointer Gestures

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Gestures.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { useDrag } from '@use-gesture/react'

function DraggableBox() {
  const meshRef = useRef<THREE.Mesh>(null!)
  const bind = useDrag(({ offset: [x, y] }) => {
    meshRef.current.position.set(x / 100, -y / 100, 0)
  })

  useFrame((state) => {
    meshRef.current.rotation.x = meshRef.current.position.y * 0.5
    meshRef.current.rotation.y = meshRef.current.position.x * 0.5
  })

  return (
    <mesh ref={meshRef} {...bind()} castShadow>
      <boxGeometry args={[1.5, 1.5, 1.5]} />
      <meshStandardMaterial color="#A4FF4F" />
    </mesh>
  )
}

export default function Gestures() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <ambientLight intensity={0.5} />
      <pointLight position={[10, 10, 10]} />
      <DraggableBox />
    </Canvas>
  )
}
```
</details>

**Key technique:** `@use-gesture/react` provides `useDrag()` for drag gestures. Spread `{...bind()}` on the mesh to attach event handlers. The offset values are divided by 100 to normalize screen coordinates to ThreeJS world units. Combine with `useFrame` for reactive visual feedback.

---

#### 8.2.4 Line Geometry

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Lines.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef, useState } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

function AnimatedLine() {
  const lineRef = useRef<THREE.Line>(null!)
  const positions = useRef(new Float32Array(300).map(() => (Math.random() - 0.5) * 10))

  useFrame((state) => {
    const time = state.clock.elapsedTime
    const array = positions.current
    for (let i = 0; i < 100; i++) {
      const i3 = i * 3
      array[i3 + 1] = Math.sin(time + i * 0.1) * 2
    }
    lineRef.current.geometry.attributes.position.needsUpdate = true
  })

  const geometry = new THREE.BufferGeometry()
  geometry.setAttribute('position', new THREE.BufferAttribute(positions.current, 3))

  return (
    <line ref={lineRef} geometry={geometry}>
      <lineBasicMaterial color="#4F8BFF" linewidth={2} />
    </line>
  )
}

export default function Lines() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <AnimatedLine />
    </Canvas>
  )
}
```
</details>

**Key technique:** Create a `THREE.BufferGeometry` with position attributes, then render with `<line>` and `<lineBasicMaterial>`. Update `geometry.attributes.position.needsUpdate = true` each frame to animate vertices. Each vertex's Y position is driven by `Math.sin(time + i * 0.1)` for a wave effect.

---

#### 8.2.5 Portals

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Portals.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef } from 'react'
import { Canvas, Portal, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

function SceneContent() {
  const groupRef = useRef<THREE.Group>(null!)
  useFrame((state) => {
    groupRef.current.rotation.y = state.clock.elapsedTime * 0.3
  })
  return (
    <group ref={groupRef}>
      <mesh position={[-2, 0, 0]}>
        <boxGeometry args={[1, 1, 1]} />
        <meshStandardMaterial color="#FF4B4B" />
      </mesh>
      <mesh position={[2, 0, 0]}>
        <sphereGeometry args={[0.7, 32, 32]} />
        <meshStandardMaterial color="#4F8BFF" />
      </mesh>
    </group>
  )
}

export default function Portals() {
  return (
    <Canvas camera={{ position: [0, 0, 6] }}>
      <ambientLight intensity={0.5} />
      <pointLight position={[5, 5, 5]} />
      <Portal>
        <SceneContent />
      </Portal>
      <mesh position={[0, 0, -2]}>
        <planeGeometry args={[10, 10]} />
        <meshStandardMaterial color="#333" />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** `<Portal>` renders its children into a separate render target, then displays the result as a texture. Useful for split-screen views, security cameras, or "views within views" effects.

---

#### 8.2.6 Reparenting

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Reparenting.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef, useState } from 'react'
import { Canvas } from '@react-three/fiber'
import * as THREE from 'three'

function Object3D() {
  const meshRef = useRef<THREE.Mesh>(null!)
  return (
    <mesh ref={meshRef} position={[0, 1, 0]}>
      <boxGeometry args={[0.5, 0.5, 0.5]} />
      <meshStandardMaterial color="#A4FF4F" />
    </mesh>
  )
}

export default function Reparenting() {
  const groupARef = useRef<THREE.Group>(null!)
  const groupBRef = useRef<THREE.Group>(null!)
  const [inGroupA, setInGroupA] = useState(true)

  const obj = <Object3D />

  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <ambientLight intensity={0.5} />
      <group ref={groupARef} position={[-2, 0, 0]}>
        {inGroupA && obj}
      </group>
      <group ref={groupBRef} position={[2, 0, 0]}>
        {!inGroupA && obj}
      </group>
      <mesh position={[0, -2, 0]} onClick={() => setInGroupA(!inGroupA)}>
        <planeGeometry args={[4, 1]} />
        <meshStandardMaterial color="#4F8BFF" />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** Reparenting by conditionally rendering the same JSX element into different parent groups. When `inGroupA` toggles, the object moves between groups while maintaining its local position/rotation.

---

#### 8.2.7 Raycasting & Selection

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Selection.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef, useState } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

function Selectable({ position, color }: { position: [number, number, number], color: string }) {
  const meshRef = useRef<THREE.Mesh>(null!)
  const [selected, setSelected] = useState(false)

  return (
    <mesh
      ref={meshRef}
      position={position}
      onClick={(e) => {
        e.stopPropagation()
        setSelected(!selected)
      }}
    >
      <boxGeometry args={[0.8, 0.8, 0.8]} />
      <meshStandardMaterial color={selected ? '#FF4B4B' : color} />
    </mesh>
  )
}

export default function Selection() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <ambientLight intensity={0.5} />
      <Selectable position={[-1.5, 0, 0]} color="#4F8BFF" />
      <Selectable position={[0, 0, 0]} color="#A4FF4F" />
      <Selectable position={[1.5, 0, 0]} color="#FFA500" />
    </Canvas>
  )
}
```
</details>

**Key technique:** Each object independently tracks its selection state. R3F handles raycasting automatically — just attach `onClick` handlers to meshes. No manual raycasting setup needed.

---

#### 8.2.8 Custom Shader Materials

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/ShaderMaterial.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

const vertexShader = `
  varying vec2 vUv;
  void main() {
    vUv = uv;
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
  }
`

const fragmentShader = `
  uniform float uTime;
  varying vec2 vUv;
  void main() {
    vec2 uv = vUv;
    float wave = sin(uv.x * 10.0 + uTime) * 0.1;
    float r = uv.x + wave;
    float g = uv.y + wave * 0.5;
    float b = sin(uv.x * 5.0 + uTime * 0.5) * 0.5 + 0.5;
    gl_FragColor = vec4(r, g, b, 1.0);
  }
`

function ShadedSphere() {
  const meshRef = useRef<THREE.Mesh>(null!)
  const uniforms = useRef({
    uTime: { value: 0 },
  })

  useFrame((state) => {
    uniforms.current.uTime.value = state.clock.elapsedTime
  })

  return (
    <mesh ref={meshRef}>
      <sphereGeometry args={[1.5, 64, 64]} />
      <shaderMaterial
        uniforms={uniforms.current}
        vertexShader={vertexShader}
        fragmentShader={fragmentShader}
      />
    </mesh>
  )
}

export default function ShaderMaterial() {
  return (
    <Canvas camera={{ position: [0, 0, 3] }}>
      <ShadedSphere />
    </Canvas>
  )
}
```
</details>

**Key technique:** Define `uniforms`, `vertexShader`, and `fragmentShader` as strings or objects, then pass them to `<shaderMaterial>`. Update uniform values in `useFrame()` to animate shaders. The sphere geometry uses 64 segments for smooth rendering.

---

#### 8.2.9 Suspense & Error Boundaries

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/SuspenseAndErrors.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { Suspense } from 'react'
import { Canvas } from '@react-three/fiber'
import { useLoader } from '@react-three/fiber'
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader'

function FallbackModel() {
  return (
    <mesh>
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial color="#FF4B4B" wireframe />
    </mesh>
  )
}

function ErrorFallback() {
  return (
    <mesh>
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial color="red" />
    </mesh>
  )
}

function Model() {
  const gltf = useLoader(GLTFLoader, '/models/nonexistent.gltf')
  return <primitive object={gltf.scene} />
}

export default function SuspenseAndErrors() {
  return (
    <Canvas>
      <ambientLight intensity={0.5} />
      <Suspense fallback={<FallbackModel />}>
        <Model />
      </Suspense>
    </Canvas>
  )
}
```
</details>

**Key technique:** Wrap async-loaded models in `<Suspense>` with a `fallback` component (e.g., a wireframe box or spinner). If the model fails to load, React error boundaries catch the error. The fallback shows immediately while the model loads.

---

#### 8.2.10 Multi-Material

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/MultiMaterial.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'
import { Canvas } from '@react-three/fiber'
import * as THREE from 'three'

export default function MultiMaterial() {
  const materials = [
    new THREE.MeshStandardMaterial({ color: '#FF4B4B' }),
    new THREE.MeshStandardMaterial({ color: '#4F8BFF' }),
    new THREE.MeshStandardMaterial({ color: '#A4FF4F' }),
    new THREE.MeshStandardMaterial({ color: '#FFA500' }),
    new THREE.MeshStandardMaterial({ color: '#FF69B4' }),
    new THREE.MeshStandardMaterial({ color: '#00CED1' }),
  ]

  return (
    <Canvas camera={{ position: [0, 0, 3] }}>
      <ambientLight intensity={0.5} />
      <pointLight position={[5, 5, 5]} />
      <mesh material={materials}>
        <boxGeometry args={[1.5, 1.5, 1.5]} />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** Pass an array of materials to a mesh's `material` prop. Each face of the geometry (indexed by face groups) gets a different material. Works with any geometry that has material indices.

---

#### 8.2.11 Multi-Render

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/MultiRender.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'
import { Canvas } from '@react-three/fiber'

function Scene({ position, color }: { position: [number, number, number], color: string }) {
  return (
    <mesh position={position}>
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial color={color} />
    </mesh>
  )
}

export default function MultiRender() {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', height: '100vh' }}>
      <div>
        <Canvas camera={{ position: [0, 0, 3] }}>
          <ambientLight intensity={0.5} />
          <Scene position={[0, 0, 0]} color="#FF4B4B" />
        </Canvas>
      </div>
      <div>
        <Canvas camera={{ position: [0, 0, 3] }}>
          <ambientLight intensity={0.5} />
          <Scene position={[0, 0, 0]} color="#4F8BFF" />
        </Canvas>
      </div>
    </div>
  )
}
```
</details>

**Key technique:** Multiple `<Canvas>` elements can exist on the same page, each with independent scenes, cameras, and lights. Use CSS grid/flexbox to arrange them.

---

#### 8.2.12 Multi-View

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/MultiView.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef } from 'react'
import { Canvas, View, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

function Scene() {
  const meshRef = useRef<THREE.Mesh>(null!)
  useFrame((state) => {
    meshRef.current.rotation.y = state.clock.elapsedTime * 0.5
  })
  return (
    <mesh ref={meshRef}>
      <boxGeometry args={[2, 2, 2]} />
      <meshStandardMaterial color="#4F8BFF" />
    </mesh>
  )
}

export default function MultiView() {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', height: '100vh' }}>
      <div id="view1" />
      <div id="view2" />
      <Canvas
        camera={{ position: [0, 0, 5] }}
        style={{ position: 'fixed', inset: 0, pointerEvents: 'none' }}
      >
        <ambientLight intensity={0.5} />
        <pointLight position={[5, 5, 5]} />
        <View track="#view1">
          <Scene />
        </View>
        <View track="#view2">
          <mesh position={[0, 0, 0]}>
            <sphereGeometry args={[1, 32, 32]} />
            <meshStandardMaterial color="#FF4B4B" />
          </mesh>
        </View>
      </Canvas>
    </div>
  )
}
```
</details>

**Key technique:** `<View track="#elementId">` renders its children into a specific DOM element using a portal. This enables split-screen with a single Canvas and shared resources (lights, shadows). Each view can have independent camera controls.

---

#### 8.2.13 Change Texture

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/ChangeTexture.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef, useState } from 'react'
import { Canvas, useLoader, useFrame } from '@react-three/fiber'
import * as THREE from 'three'
import { TextureLoader } from 'three/src/loaders/TextureLoader'

function ChangingTexture() {
  const meshRef = useRef<THREE.Mesh>(null!)
  const textures = [
    useLoader(TextureLoader, '/textures/1.jpg'),
    useLoader(TextureLoader, '/textures/2.jpg'),
    useLoader(TextureLoader, '/textures/3.jpg'),
  ]
  const [index, setIndex] = useState(0)

  useFrame((state) => {
    meshRef.current.rotation.y = state.clock.elapsedTime * 0.3
  })

  return (
    <mesh
      ref={meshRef}
      onClick={() => setIndex((i) => (i + 1) % textures.length)}
    >
      <boxGeometry args={[2, 2, 2]} />
      <meshStandardMaterial map={textures[index]} />
    </mesh>
  )
}

export default function ChangeTexture() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <ambientLight intensity={0.5} />
      <pointLight position={[5, 5, 5]} />
      <ChangingTexture />
    </Canvas>
  )
}
```
</details>

**Key technique:** `useLoader(TextureLoader, url)` loads textures. Cycle through an array of textures by updating state and passing `map={textures[currentIndex]}` to the material. Click the mesh to change its texture.

---

#### 8.2.14 Viewcube

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Viewcube.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

function ViewCube() {
  const groupRef = useRef<THREE.Group>(null!)
  useFrame((state) => {
    groupRef.current.rotation.x = state.pointer.y * 0.5
    groupRef.current.rotation.y = state.pointer.x * 0.5
  })

  const faces = [
    { position: [0, 0, 1.5], color: '#FF4B4B' },
    { position: [0, 0, -1.5], color: '#4F8BFF' },
    { position: [1.5, 0, 0], color: '#A4FF4F' },
    { position: [-1.5, 0, 0], color: '#FFA500' },
    { position: [0, 1.5, 0], color: '#FF69B4' },
    { position: [0, -1.5, 0], color: '#00CED1' },
  ]

  return (
    <group ref={groupRef}>
      {faces.map((face, i) => (
        <mesh key={i} position={face.position as [number, number, number]}>
          <boxGeometry args={[0.3, 0.3, 0.3]} />
          <meshStandardMaterial color={face.color} />
        </mesh>
      ))}
    </group>
  )
}

export default function Viewcube() {
  return (
    <Canvas camera={{ position: [0, 0, 5]] }}>
      <ambientLight intensity={0.5} />
      <ViewCube />
    </Canvas>
  )
}
```
</details>

**Key technique:** Six colored cubes positioned on each axis face. The group rotates based on `state.pointer.x` and `state.pointer.y` (normalized mouse coordinates from -1 to 1), creating a viewcube that follows the cursor.

---

#### 8.2.15 View Tracking

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/ViewTracking.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

function TrackingSphere() {
  const meshRef = useRef<THREE.Mesh>(null!)
  useFrame((state) => {
    // Follow pointer in 3D space
    const x = (state.pointer.x * state.viewport.width) / 2
    const y = (state.pointer.y * state.viewport.height) / 2
    meshRef.current.position.set(x, y, 0)
  })
  return (
    <mesh ref={meshRef}>
      <sphereGeometry args={[0.3, 32, 32]} />
      <meshStandardMaterial color="#FF4B4B" />
    </mesh>
  )
}

export default function ViewTracking() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <ambientLight intensity={0.5} />
      <TrackingSphere />
    </Canvas>
  )
}
```
</details>

**Key technique:** `state.pointer` provides normalized cursor coordinates (-1 to 1). Multiply by `state.viewport.width / 2` to convert to world-space units. The sphere follows the cursor in 3D space.

---

#### 8.2.16 Point Cloud

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Pointcloud.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef, useMemo } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

function PointCloud({ count = 5000 }) {
  const meshRef = useRef<THREE.Points>(null!)

  const [positions, colors] = useMemo(() => {
    const pos = new Float32Array(count * 3)
    const col = new Float32Array(count * 3)
    for (let i = 0; i < count; i++) {
      const i3 = i * 3
      const radius = Math.random() * 3
      const theta = Math.random() * Math.PI * 2
      const phi = Math.acos(2 * Math.random() - 1)
      pos[i3] = radius * Math.sin(phi) * Math.cos(theta)
      pos[i3 + 1] = radius * Math.sin(phi) * Math.sin(theta)
      pos[i3 + 2] = radius * Math.cos(phi)
      col[i3] = Math.random()
      col[i3 + 1] = Math.random()
      col[i3 + 2] = Math.random()
    }
    return [pos, col]
  }, [count])

  useFrame((state) => {
    meshRef.current.rotation.y = state.clock.elapsedTime * 0.1
  })

  return (
    <points ref={meshRef}>
      <bufferGeometry>
        <bufferAttribute
          attach="attributes-position"
          count={positions.length / 3}
          array={positions}
          itemSize={3}
        />
        <bufferAttribute
          attach="attributes-color"
          count={colors.length / 3}
          array={colors}
          itemSize={3}
        />
      </bufferGeometry>
      <pointsMaterial size={0.05} vertexColors sizeAttenuation />
    </points>
  )
}

export default function Pointcloud() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <PointCloud />
    </Canvas>
  )
}
```
</details>

**Key technique:** Use `<points>` with `<bufferGeometry>` and `<pointsMaterial>` for GPU-accelerated point clouds. `vertexColors` enables per-point color attributes. `sizeAttenuation` makes points smaller as they get farther from camera. Positions are distributed on a sphere surface using spherical coordinates.

---

#### 8.2.17 WebGPU

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/WebGPU.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'
import { Canvas } from '@react-three/fiber'

export default function WebGPU() {
  return (
    <Canvas
      camera={{ position: [0, 0, 5] }}
      // WebGPU backend available when browser supports it
    >
      <mesh>
        <boxGeometry args={[2, 2, 2]} />
        <meshStandardMaterial color="#4F8BFF" />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** `@react-three/fiber` v8+ can use the WebGPU renderer when the browser supports it. No code changes needed — swap the renderer at the Canvas level.

---

#### 8.2.18 Additional Demos

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Activity.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import * as THREE from 'three'

function ActivityIndicator() {
  const meshRef = useRef<THREE.Mesh>(null!)
  useFrame((state) => {
    meshRef.current.rotation.x = Math.sin(state.clock.elapsedTime * 2) * 0.5
    meshRef.current.rotation.y = Math.cos(state.clock.elapsedTime * 2) * 0.5
  })
  return (
    <mesh ref={meshRef}>
      <torusKnotGeometry args={[1, 0.3, 100, 16]} />
      <meshStandardMaterial color="#A4FF4F" wireframe />
    </mesh>
  )
}

export default function Activity() {
  return (
    <Canvas camera={{ position: [0, 0, 4] }}>
      <ambientLight intensity={0.5} />
      <ActivityIndicator />
    </Canvas>
  )
}
```
</details>

**Key technique:** Use a `<torusKnotGeometry>` with `wireframe` for a loading/activity indicator. Dual-axis rotation (`x` and `y`) with sine/cosine creates a hypnotic spinning effect.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/AutoDispose.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useState } from 'react'
import { Canvas } from '@react-three/fiber'

function DisposableMesh() {
  return (
    <mesh>
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial color="#FF4B4B" />
    </mesh>
  )
}

export default function AutoDispose() {
  const [show, setShow] = useState(true)
  return (
    <>
      <button onClick={() => setShow(!show)}>Toggle</button>
      <Canvas camera={{ position: [0, 0, 3] }}>
        <ambientLight intensity={0.5} />
        {show && <DisposableMesh />}
      </Canvas>
    </>
  )
}
```
</details>

**Key technique:** R3F automatically disposes GPU resources (geometries, materials, textures) when components unmount. Toggle visibility with React state — no manual cleanup needed.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/ContextMenuOverride.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'
import { Canvas } from '@react-three/fiber'

export default function ContextMenuOverride() {
  return (
    <Canvas
      camera={{ position: [0, 0, 3] }}
      onContextMenu={(e) => e.preventDefault()}
    >
      <mesh>
        <boxGeometry args={[2, 2, 2]} />
        <meshStandardMaterial color="#4F8BFF" />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** Pass `onContextMenu` to `<Canvas>` to override the right-click context menu, useful for camera controls that use right-click drag.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/FlushSync.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useState } from 'react'
import { Canvas } from '@react-three/fiber'
import { flushSync } from 'react-dom'

export default function FlushSync() {
  const [count, setCount] = useState(0)
  return (
    <Canvas camera={{ position: [0, 0, 3] }}>
      <mesh onClick={() => flushSync(() => setCount((c) => c + 1))}>
        <boxGeometry args={[1 + count * 0.1, 1 + count * 0.1, 1 + count * 0.1]} />
        <meshStandardMaterial color="#A4FF4F" />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** `flushSync` forces synchronous state updates, ensuring the canvas re-renders immediately on click rather than batching the update.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Inject.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'
import { Canvas } from '@react-three/fiber'
import * as THREE from 'three'

export default function Inject() {
  // Injects a Three.js object directly into the scene
  const scene = new THREE.Scene()
  return (
    <Canvas camera={{ position: [0, 0, 3] }} scene={scene}>
      <mesh>
        <boxGeometry args={[2, 2, 2]} />
        <meshStandardMaterial color="#FF4B4B" />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** Inject a custom Three.js `Scene` into the Canvas via the `scene` prop. Useful when integrating with non-R3F Three.js code.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/Layers.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'
import { Canvas } from '@react-three/fiber'
import * as THREE from 'three'

export default function Layers() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }} layers={[1]}>
      <mesh layers={[1]} position={[-1, 0, 0]}>
        <boxGeometry args={[1, 1, 1]} />
        <meshStandardMaterial color="#FF4B4B" />
      </mesh>
      <mesh layers={[2]} position={[1, 0, 0]}>
        <boxGeometry args={[1, 1, 1]} />
        <meshStandardMaterial color="#4F8BFF" />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** Use `layers` prop on Canvas and meshes to control visibility. The camera only renders objects on its layer mask. Objects on different layers are invisible.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/ResetProps.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { useState } from 'react'
import { Canvas } from '@react-three/fiber'

function ResettableMesh() {
  const [color, setColor] = useState('#4F8BFF')
  return (
    <mesh onClick={() => setColor('#FF4B4B')}>
      <boxGeometry args={[2, 2, 2]} />
      <meshStandardMaterial color={color} />
    </mesh>
  )
}

export default function ResetProps() {
  const [key, setKey] = useState(0)
  return (
    <>
      <button onClick={() => setKey((k) => k + 1)}>Reset</button>
      <Canvas key={key} camera={{ position: [0, 0, 3] }}>
        <ambientLight intensity={0.5} />
        <ResettableMesh />
      </Canvas>
    </>
  )
}
```
</details>

**Key technique:** Use React's `key` prop on `<Canvas>` to force remount and reset all state when the key changes. Useful for "reset scene" functionality.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/StopPropagation.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'
import { Canvas } from '@react-three/fiber'

export default function StopPropagation() {
  return (
    <Canvas camera={{ position: [0, 0, 5] }}>
      <mesh onClick={(e) => console.log('parent')}>
        <boxGeometry args={[3, 3, 3]} />
        <meshStandardMaterial color="#333" />
        <mesh
          position={[0, 0, 1.51]}
          onClick={(e) => {
            e.stopPropagation()
            console.log('child — stops propagation')
          }}
        >
          <planeGeometry args={[2, 2]} />
          <meshStandardMaterial color="#FF4B4B" />
        </mesh>
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** `e.stopPropagation()` prevents click events from bubbling to parent meshes. Without it, clicking the child plane would also trigger the parent box's onClick.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/SuspenseMaterial.tsx`

<details>
<summary>View code</summary>

```tsx
import React, { Suspense } from 'react'
import { Canvas } from '@react-three/fiber'

function AsyncMaterial() {
  // Simulates a material that loads asynchronously
  return (
    <mesh>
      <boxGeometry args={[2, 2, 2]} />
      <meshStandardMaterial color="#A4FF4F" />
    </mesh>
  )
}

export default function SuspenseMaterial() {
  return (
    <Canvas camera={{ position: [0, 0, 3] }}>
      <Suspense fallback={<mesh><boxGeometry args={[2, 2, 2]} /><meshStandardMaterial color="gray" /></mesh>}>
        <AsyncMaterial />
      </Suspense>
    </Canvas>
  )
}
```
</details>

**Key technique:** Wrap mesh/material components in `<Suspense>` when they depend on async resources (textures, shader compilation, etc.). The fallback renders a placeholder.

---

**File:** `useful-repos-for-frontend/react-three-fiber/example/src/demos/SVGRenderer.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'
import { Canvas } from '@react-three/fiber'

export default function SVGRenderer() {
  return (
    <Canvas
      camera={{ position: [0, 0, 3] }}
      // Uses SVG renderer instead of WebGL
      gl={{ antialias: true }}
    >
      <mesh>
        <boxGeometry args={[2, 2, 2]} />
        <meshStandardMaterial color="#4F8BFF" />
      </mesh>
    </Canvas>
  )
}
```
</details>

**Key technique:** R3F can use Three.js's SVG renderer for vector output. The SVG renderer produces resolution-independent graphics suitable for print or embedding in documents.

---

**Key concepts summary:**
- All ThreeJS objects are available as JSX elements (`<mesh>`, `<ambientLight>`, `<boxGeometry>`, etc.)
- `useFrame(callback)` — subscribe a component to the render loop (runs every frame)
- Components react to state, participate in React's event system (onClick, onPointerOver, etc.)
- Works with React 18 or 19 (v8 pairs with React 18, v9 with React 19)
- Also supports React Native

**Ecosystem (additional packages available on npm):**
| Package | Purpose |
|---|---|
| `@react-three/drei` | Useful helpers and abstractions |
| `@react-three/postprocessing` | Post-processing effects |
| `@react-three/gltfjsx` | Convert GLTF models to JSX components |
| `@react-three/rapier` | 3D physics |
| `@react-three/uikit` | WebGL rendered UI components |

---

### 8.3 ShaderGradient — 3D Moving Gradients for React (4 Techniques)

**Local path:** `useful-repos-for-frontend/shadergradient/`

Customizable 3D moving gradient component for React with controls for shape, colors, and motion.

**Install:**
```
npm i @shadergradient/react @react-three/fiber three three-stdlib camera-controls
npm i -D @types/three
```

#### 8.3.1 Basic Usage

**File:** `useful-repos-for-frontend/shadergradient/apps/example-nextjs-dev/app/page.tsx`

<details>
<summary>View code</summary>

```tsx
'use client'

import React, { useState } from 'react'
import { ShaderGradientCanvas, ShaderGradient } from '@shadergradient/react'

export default function Page() {
  const [type, setType] = useState<'sphere' | 'plane' | 'waterPlane'>('sphere')

  return (
    <div style={{ position: 'relative', width: '100vw', height: '100vh' }}>
      <ShaderGradientCanvas style={{ position: 'absolute', inset: 0 }}>
        <ShaderGradient
          type={type}
          animate='on'
          cDistance={32}
          cPolarAngle={125}
        />
      </ShaderGradientCanvas>
      <div style={{ position: 'absolute', bottom: 20, left: 20, display: 'flex', gap: 10 }}>
        <button onClick={() => setType('sphere')}>Sphere</button>
        <button onClick={() => setType('plane')}>Plane</button>
        <button onClick={() => setType('waterPlane')}>Water Plane</button>
      </div>
    </div>
  )
}
```
</details>

**Key technique:** `ShaderGradientCanvas` wraps the scene, `ShaderGradient` renders the gradient mesh. The `type` prop switches between `'sphere'`, `'plane'`, and `'waterPlane'` shapes. Place interactive UI overlays on top using absolute positioning.

---

#### 8.3.2 Customize Simple

**File:** `useful-repos-for-frontend/shadergradient/apps/example-nextjs-dev/app/customize-simple/page.tsx`

<details>
<summary>View code</summary>

```tsx
'use client'

import React from 'react'
import { ShaderGradientCanvas, ShaderGradient } from '@shadergradient/react'

export default function CustomizeSimple() {
  return (
    <div style={{ position: 'relative', width: '100vw', height: '100vh' }}>
      <ShaderGradientCanvas
        style={{ position: 'absolute', inset: 0 }}
        pixelDensity={1.5}
        fov={45}
      >
        <ShaderGradient
          type='sphere'
          animate='on'
          cDistance={3.6}
          cPolarAngle={125}
          color1='#52ff89'
          color2='#dbba95'
          color3='#ff5252'
          uSpeed={0.5}
          uStrength={0.3}
          uDensity={1.2}
          uFrequency={3.5}
          uAmplitude={0.2}
          reflection={0.2}
        />
      </ShaderGradientCanvas>
    </div>
  )
}
```
</details>

**Key technique:** Customize gradient colors (`color1`, `color2`, `color3`), animation parameters (`uSpeed`, `uStrength`, `uDensity`, `uFrequency`, `uAmplitude`), camera position (`cDistance`, `cPolarAngle`), and visual properties (`reflection`, `pixelDensity`, `fov`).

---

#### 8.3.3 Loop Configuration

**File:** `useful-repos-for-frontend/shadergradient/apps/example-nextjs-dev/app/loop/page.tsx`

<details>
<summary>View code</summary>

```tsx
'use client'

import React from 'react'
import { ShaderGradientCanvas, ShaderGradient } from '@shadergradient/react'

export default function Loop() {
  return (
    <div style={{ position: 'relative', width: '100vw', height: '100vh' }}>
      <ShaderGradientCanvas style={{ position: 'absolute', inset: 0 }}>
        <ShaderGradient
          type='waterPlane'
          animate='on'
          cDistance={20}
          cPolarAngle={140}
          uSpeed={0.3}
          uFrequency={4.5}
          uAmplitude={0.4}
          grain='on'
          lightType='3d'
        />
      </ShaderGradientCanvas>
    </div>
  )
}
```
</details>

**Key technique:** The `waterPlane` type creates an undulating water-like surface. `grain='on'` adds film grain texture. `lightType='3d'` enables 3D lighting for more depth.

---

#### 8.3.4 Scene Integration

**File:** `useful-repos-for-frontend/shadergradient/apps/examples/example-nextjs/src/components/canvas/Scene.tsx`

<details>
<summary>View code</summary>

```tsx
'use client'

import React from 'react'
import { ShaderGradientCanvas, ShaderGradient } from '@shadergradient/react'

export default function Scene() {
  return (
    <ShaderGradientCanvas
      style={{ position: 'fixed', inset: 0, zIndex: -1 }}
      pixelDensity={1}
      fov={45}
    >
      <ShaderGradient
        type='sphere'
        animate='on'
        cDistance={40}
        cPolarAngle={130}
        uSpeed={0.4}
        uDensity={1.5}
        uFrequency={2}
        color1='#ff6b6b'
        color2='#4ecdc4'
        color3='#45b7d1'
      />
    </ShaderGradientCanvas>
  )
}
```
</details>

**Key technique:** Use `zIndex: -1` to place the gradient as a fixed background behind all page content. `pixelDensity={1}` balances quality vs. performance.

---

**Vite + React example:**

**File:** `useful-repos-for-frontend/shadergradient/apps/examples/example-vite-react/src/App.tsx`

```tsx
import { ShaderGradientCanvas, ShaderGradient } from '@shadergradient/react'

function App() {
  return (
    <ShaderGradientCanvas style={{ position: 'absolute', inset: 0 }}>
      <ShaderGradient cDistance={32} cPolarAngle={125} />
    </ShaderGradientCanvas>
  )
}

export default App
```

**Available gradient properties:**
| Prop | Type | Description |
|---|---|---|
| `type` | `'plane' \| 'sphere' \| 'waterPlane'` | Mesh shape |
| `animate` | `'on' \| 'off'` | Enable/disable animation |
| `uSpeed`, `uStrength`, `uDensity`, `uFrequency`, `uAmplitude` | `number` | Animation parameters |
| `color1`, `color2`, `color3` | `string` (hex) | Gradient colors |
| `reflection` | `number` | Reflection intensity |
| `wireframe` | `boolean` | Wireframe mode |
| `cAzimuthAngle`, `cPolarAngle`, `cDistance` | `number` | Camera position |
| `lightType` | `'3d' \| 'env'` | Lighting mode |
| `grain` | `'on' \| 'off'` | Film grain effect |

**Or load settings from a URL string:**
```tsx
<ShaderGradient
  control='query'
  urlString='https://www.shadergradient.co/customize?animate=on&cDistance=3.6&color1=%2352ff89&color2=%23dbba95'
/>
```

**Compatibility:** React 18 or 19. For Next.js 15 App Router, use `@react-three/fiber@^9.0.0` with React 19.

---

### 8.4 liquid-glass-js — Apple Liquid Glass Effects (4 Techniques)

**Local path:** `useful-repos-for-frontend/liquid-glass-js/`

WebGL-powered library for Apple Liquid Glass-inspired glass effects with real-time refraction, blur, and masking. Zero dependencies (except html2canvas for page sampling).

**Import (vanilla JS — no build step required):**
```html
<link rel="stylesheet" href="styles.css" />
<link rel="stylesheet" href="glass.css" />
<script src="https://cdn.jsdelivr.net/npm/html2canvas@1.4.1/dist/html2canvas.min.js"></script>
<script src="container.js"></script>
<script src="button.js"></script>
```

#### 8.4.1 Basic Glass Demo

**File:** `useful-repos-for-frontend/liquid-glass-js/demo.js`

<details>
<summary>View code</summary>

```javascript
const button1 = new Button({
  text: 'Hello Glass!',
  size: 28,
  type: 'pill',
  tintOpacity: 0.2,
  warp: false,
  onClick: () => alert('Hello Glass!')
})
document.body.appendChild(button1.element)

const container = new Container({
  borderRadius: 24,
  type: 'pill',
  tintOpacity: 0.3
})

container.element.style.position = 'absolute'
container.element.style.bottom = '30px'
container.element.style.right = '30px'

const button2 = new Button({ text: 'Action', size: 24, type: 'pill' })
const button3 = new Button({ text: '✓', size: 24, type: 'circle' })

container.addChild(button2)
container.addChild(button3)
document.body.appendChild(container.element)
```
</details>

**Key technique:** Create standalone `Button` instances or group them in a `Container`. Buttons accept `text`, `size`, `type` (`'rounded'`, `'circle'`, `'pill'`), `tintOpacity`, `warp`, and `onClick`. Containers can hold multiple child buttons.

#### 8.4.2 HTML Structure

**File:** `useful-repos-for-frontend/liquid-glass-js/index.html`

<details>
<summary>View code</summary>

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Liquid Glass</title>
  <link rel="stylesheet" href="styles.css">
  <link rel="stylesheet" href="glass.css">
</head>
<body>
  <main id="app">
    <section class="hero">
      <h1>Liquid Glass</h1>
      <p>WebGL-powered glass effects</p>
    </section>
    <section class="cards">
      <div class="card">
        <h2>Card One</h2>
        <p>Background content visible through glass</p>
      </div>
    </section>
  </main>

  <script src="https://cdn.jsdelivr.net/npm/html2canvas@1.4.1/dist/html2canvas.min.js"></script>
  <script src="container.js"></script>
  <script src="button.js"></script>
  <script src="demo.js"></script>
</body>
</html>
```
</details>

**Key technique:** The page structure loads CSS first, then scripts. `html2canvas` captures the page content behind the glass elements for the blur/refraction effect.

#### 8.4.3 Real-time Controls

**File:** `useful-repos-for-frontend/liquid-glass-js/controls.js`

<details>
<summary>View code</summary>

```javascript
const controls = {
  edgeIntensity: 0.05,
  rimIntensity: 0.1,
  baseIntensity: 0.02,
  blurRadius: 8,
  tintOpacity: 0.3,
  rippleEffect: 0.1,
}

function createControlPanel() {
  const panel = document.createElement('div')
  panel.className = 'controls-panel'

  Object.entries(controls).forEach(([key, value]) => {
    const row = document.createElement('div')
    row.className = 'control-row'

    const label = document.createElement('label')
    label.textContent = key
    label.htmlFor = key

    const input = document.createElement('input')
    input.type = 'range'
    input.id = key
    input.min = '0'
    input.max = key === 'edgeIntensity' || key === 'rimIntensity' || key === 'baseIntensity' ? '0.1'
      : key === 'blurRadius' ? '15'
      : key === 'tintOpacity' ? '1'
      : '0.5'
    input.step = '0.01'
    input.value = value

    const valueDisplay = document.createElement('span')
    valueDisplay.className = 'control-value'
    valueDisplay.textContent = value

    input.addEventListener('input', () => {
      controls[key] = parseFloat(input.value)
      valueDisplay.textContent = input.value
      window.glassControls.update(controls)
    })

    row.appendChild(label)
    row.appendChild(input)
    row.appendChild(valueDisplay)
    panel.appendChild(row)
  })

  document.body.appendChild(panel)
}

createControlPanel()

// Expose controls for other scripts
window.glassControls = {
  update: (newControls) => {
    // Apply controls to all glass instances
    document.querySelectorAll('.glass').forEach(el => {
      Object.assign(el.dataset, newControls)
    })
  }
}
```
</details>

**Key technique:** A control panel with sliders for each glass parameter. `window.glassControls.update(controls)` applies changes to all glass elements in real-time. Parameter ranges: `edgeIntensity` 0–0.1, `rimIntensity` 0–0.2, `blurRadius` 1–15, `tintOpacity` 0–1, `rippleEffect` 0–0.5.

#### 8.4.4 Glass Effect Parameters

**Glass effect parameters (adjustable in real-time via `window.glassControls`):**
| Parameter | Range | Description |
|---|---|---|
| `edgeIntensity` | 0–0.1 | Refraction strength at shape edges |
| `rimIntensity` | 0–0.2 | Rim lighting effects |
| `baseIntensity` | 0–0.05 | Center distortion strength |
| `blurRadius` | 1–15 | Background blur amount |
| `tintOpacity` | 0–1.0 | Gradient overlay strength |
| `rippleEffect` | 0–0.5 | Surface texture simulation |

**File structure:**
```
useful-repos-for-frontend/liquid-glass-js/
├── container.js      # Core Container class (WebGL rendering)
├── button.js         # Button class (extends Container)
├── demo.js           # Demo setup and controls
├── controls.js       # Real-time parameter controls
├── controls.css      # Controls panel styling
├── styles.css        # Base styling
├── glass.css         # Glass component styles
├── demo.css          # Demo page styling
└── index.html        # Demo page
```

---

### 8.5 liquid-logo — Metal Chrome Logo Effects (4 Techniques)

**Local path:** `useful-repos-for-frontend/liquid-logo/`

Next.js project for creating liquid metal / chrome look effects on logos and text.

**Run:**
```
cd useful-repos-for-frontend/liquid-logo
npm run dev
```

**Demo:** `liquid.paper.design`

Built with Next.js 15 + Tailwind CSS + Three.js (WebGL).

#### 8.5.1 Main Page

**File:** `useful-repos-for-frontend/liquid-logo/src/app/page.tsx`

<details>
<summary>View code</summary>

```tsx
import { Hero } from '../hero/hero'

export default function Home() {
  return (
    <main>
      <Hero />
    </main>
  )
}
```
</details>

**Key technique:** The main page renders a single `<Hero>` component that contains the full liquid logo experience.

#### 8.5.2 Hero Component

**File:** `useful-repos-for-frontend/liquid-logo/src/hero/hero.tsx`

<details>
<summary>View code</summary>

```tsx
'use client'

import React from 'react'
import { Canvas } from '../hero/canvas'
import { Logos } from '../hero/logos'

export function Hero() {
  return (
    <div style={{ width: '100vw', height: '100vh', position: 'relative' }}>
      <Canvas />
      <Logos />
    </div>
  )
}
```
</details>

**Key technique:** The hero splits into two layers: the `<Canvas>` (Three.js WebGL render) and `<Logos>` (overlay UI). The canvas renders the liquid metal effect, while Logos provide interactive controls.

#### 8.5.3 Canvas (WebGL Renderer)

**File:** `useful-repos-for-frontend/liquid-logo/src/hero/canvas.tsx`

<details>
<summary>View code</summary>

```tsx
'use client'

import React, { useRef, useEffect } from 'react'
import * as THREE from 'three'
import { liquidFrag } from './liquid-frag'
import { params } from './params'

export function Canvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null!)

  useEffect(() => {
    const canvas = canvasRef.current
    const renderer = new THREE.WebGLRenderer({ canvas, alpha: true })
    renderer.setSize(window.innerWidth, window.innerHeight)
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))

    const scene = new THREE.Scene()
    const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1)

    const geometry = new THREE.PlaneGeometry(2, 2)
    const material = new THREE.ShaderMaterial({
      uniforms: {
        uTime: { value: 0 },
        uResolution: { value: new THREE.Vector2(window.innerWidth, window.innerHeight) },
        uColor1: { value: new THREE.Color(params.color1) },
        uColor2: { value: new THREE.Color(params.color2) },
        uSpeed: { value: params.speed },
        uIntensity: { value: params.intensity },
      },
      vertexShader: `
        varying vec2 vUv;
        void main() {
          vUv = uv;
          gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        }
      `,
      fragmentShader: liquidFrag,
    })

    const mesh = new THREE.Mesh(geometry, material)
    scene.add(mesh)

    const clock = new THREE.Clock()
    function animate() {
      material.uniforms.uTime.value = clock.getElapsedTime()
      renderer.render(scene, camera)
      requestAnimationFrame(animate)
    }
    animate()

    const handleResize = () => {
      renderer.setSize(window.innerWidth, window.innerHeight)
      material.uniforms.uResolution.value.set(window.innerWidth, window.innerHeight)
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      renderer.dispose()
    }
  }, [])

  return <canvas ref={canvasRef} style={{ position: 'absolute', inset: 0, zIndex: 0 }} />
}
```
</details>

**Key technique:** Custom Three.js setup with `ShaderMaterial` for the liquid metal effect. The `liquid-frag.ts` file contains the GLSL fragment shader that generates the chrome/liquid effect. Parameters like `color1`, `color2`, `speed`, and `intensity` are passed as uniforms. The renderer uses `alpha: true` for transparency.

#### 8.5.4 Liquid Fragment Shader

**File:** `useful-repos-for-frontend/liquid-logo/src/hero/liquid-frag.ts`

<details>
<summary>View code</summary>

```glsl
export const liquidFrag = `
  uniform float uTime;
  uniform vec2 uResolution;
  uniform vec3 uColor1;
  uniform vec3 uColor2;
  uniform float uSpeed;
  uniform float uIntensity;

  varying vec2 vUv;

  void main() {
    vec2 uv = vUv;
    vec2 p = uv * 2.0 - 1.0;

    float t = uTime * uSpeed;
    float dist = length(p);

    float wave1 = sin(p.x * 5.0 + t) * 0.1;
    float wave2 = sin(p.y * 5.0 + t * 0.8) * 0.1;
    float wave3 = sin((p.x + p.y) * 3.0 + t * 0.5) * 0.05;

    float displacement = wave1 + wave2 + wave3;
    vec3 color = mix(uColor1, uColor2, displacement * uIntensity + 0.5);

    float glow = 1.0 - smoothstep(0.0, 1.0, dist);
    color += glow * 0.2;

    float edge = 1.0 - smoothstep(0.7, 1.0, dist);
    color *= edge;

    gl_FragColor = vec4(color, 1.0);
  }
`
```
</details>

**Key technique:** The shader combines multiple sine waves at different frequencies and speeds to create the liquid displacement effect. `mix(uColor1, uColor2, displacement)` interpolates between the two colors based on wave displacement. Edge glow and vignette are added via distance calculations.

#### 8.5.5 Params Configuration

**File:** `useful-repos-for-frontend/liquid-logo/src/hero/params.ts`

```typescript
export const params = {
  color1: '#ff6b6b',
  color2: '#4ecdc4',
  speed: 0.5,
  intensity: 1.0,
  scale: 1.0,
}
```

**Key technique:** Centralized parameter object. Update these values to customize the liquid effect appearance and animation.

#### 8.5.6 Logos Overlay

**File:** `useful-repos-for-frontend/liquid-logo/src/hero/logos.tsx`

<details>
<summary>View code</summary>

```tsx
'use client'

import React from 'react'
import { PaperLogo } from '../app/paper-logo'

export function Logos() {
  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <PaperLogo />
    </div>
  )
}
```
</details>

**Key technique:** The logos overlay sits on top of the liquid canvas with `zIndex: 1`. Use absolute positioning to center content over the liquid background.

#### 8.5.7 Paper Logo Component

**File:** `useful-repos-for-frontend/liquid-logo/src/app/paper-logo.tsx`

<details>
<summary>View code</summary>

```tsx
import React from 'react'

export function PaperLogo(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 100 100" {...props}>
      <defs>
        <clipPath id="logoClip">
          <path d="M50 5 L95 25 L95 75 L50 95 L5 75 L5 25 Z" />
        </clipPath>
      </defs>
      <g clipPath="url(#logoClip)">
        <rect x="0" y="0" width="100" height="100" fill="currentColor" />
      </g>
    </svg>
  )
}
```
</details>

**Key technique:** SVG logo rendered on top of the liquid canvas. The `clipPath` creates a hexagonal/logo-shaped window through which the liquid effect is visible.

---

## 9. Using shadcn Components

### 9.1 What is shadcn?

- **Not a traditional component library** — it's a copy/paste collection. Components are added to your source tree, not installed as a dependency.
- **Built on Radix UI primitives** — handles accessibility, keyboard navigation, focus management, and ARIA attributes out of the box.
- **Components live in your repo** — under `src/components/ui/`. Fully editable since the code is yours.
- **Requires Tailwind CSS** — styling uses Tailwind utility classes plus CSS variables for theming.
- **Build tool agnostic** — works with Vite, Next.js, Astro, Remix, or any framework that supports React + Tailwind.

### 9.2 Setup

```bash
npx shadcn@latest init
```

This command:

1. Creates a `components.json` file (configuration for shadcn)
2. Adds CSS variable tokens to your global CSS (`--background`, `--foreground`, `--primary`, `--radius`, etc.)
3. Installs `tailwind-merge`, `clsx`, and `class-variance-authority`
4. Sets up a `cn()` utility in `src/lib/utils.ts`

**Path alias (`@/`):** Ensure your `vite.config.ts` (or `tsconfig.json`) has the `@` alias pointing to `src/`:

```typescript
// vite.config.ts
resolve: {
  alias: {
    '@': '/src',
  },
}
```

This enables imports like:

```typescript
import { Button } from '@/components/ui/button'
```

### 9.3 Adding Components

```bash
# Single component
npx shadcn@latest add button

# Multiple at once
npx shadcn@latest add dialog dropdown-menu form input select toast
```

Each command creates a single file (or a small directory) inside `src/components/ui/`. Components are self-contained and have zero internal dependencies between shadcn files.

### 9.4 Customization — Two Approaches

**Approach 1: CSS Variables (theming)**

Edit the `:root` block in your global CSS to change theme-wide tokens:

```css
:root {
  --primary: 221.2 83.2% 53.3%;
  --primary-foreground: 210 40% 98%;
  --radius: 0.5rem;
}

.dark {
  --primary: 220 70% 55%;
}
```

Values use HSL color format. Change these once and every shadcn component updates.

**Approach 2: Tailwind classes (one-off overrides)**

Every shadcn component accepts a `className` prop:

```tsx
<Button className="bg-red-500 hover:bg-red-600 text-white">
  Delete
</Button>
```

**Approach 3: Edit the source directly**

Since the code is in your own repo, you can change any component's internals. This is useful for adding a new variant that doesn't fit shadcn's existing patterns. However, see 9.8 for upgrade caveats.

### 9.5 Common Components Reference

| Component | Best For | Radix Primitive |
|---|---|---|
| `Button` | Primary/secondary actions, form submission | — (custom) |
| `Input` / `Textarea` | Text entry fields | — (custom) |
| `Label` | Accessible form labels | `@radix-ui/react-label` |
| `Form` | Forms with validation + error display | wraps `react-hook-form` |
| `Card` | Grouped content sections (header, content, footer) | — (custom) |
| `Dialog` | Modals, confirmations, quick actions | `@radix-ui/react-dialog` |
| `AlertDialog` | Destructive confirmations ("Are you sure?") | `@radix-ui/react-alert-dialog` |
| `DropdownMenu` | Context menus, user avatar menus | `@radix-ui/react-dropdown-menu` |
| `Select` | Dropdown selection from a list | `@radix-ui/react-select` |
| `Badge` | Status indicators, tags, counters | — (custom) |
| `Toast` | Transient notifications (success, error, info) | `@radix-ui/react-toast` |
| `Separator` | Horizontal/vertical dividers | `@radix-ui/react-separator` |
| `Tabs` | Tabbed content sections | `@radix-ui/react-tabs` |
| `Table` | Data tables (not data grids — use TanStack Table for that) | — (custom) |
| `Sheet` | Slide-in panels (mobile nav, settings drawers) | `@radix-ui/react-dialog` |
| `Skeleton` | Loading placeholders | — (custom) |
| `Tooltip` | Hover/tap tooltips | `@radix-ui/react-tooltip` |

### 9.6 Comparison: shadcn vs. Traditional UI Libraries

| Aspect | shadcn | MUI / Chakra / Ant Design |
|---|---|---|
| Install | Copy into source | `npm install package` |
| Bundle size | Only what you add | Full library |
| Customization | Anywhere (CSS vars, Tailwind, source edit) | Theming API only |
| Upgrades | Re-copy the file | `npm update` |
| Control | Full — you own the code | Limited to library API |
| Accessibility | Radix primitives reinforce it | Built-in but fixed |
| Learning curve | Need Tailwind knowledge | Learn their prop/theme API |

### 9.7 Composition Patterns

**Dialog + Button + Form**

```tsx
<Dialog>
  <DialogTrigger asChild>
    <Button>New Item</Button>
  </DialogTrigger>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Create Item</DialogTitle>
      <DialogDescription>Add a new item to your list.</DialogDescription>
    </DialogHeader>
    <form>
      <Input placeholder="Name" />
      <Button type="submit">Save</Button>
    </form>
  </DialogContent>
</Dialog>
```

**DropdownMenu + Button asChild**

```tsx
<DropdownMenu>
  <DropdownMenuTrigger asChild>
    <Button variant="ghost">Options</Button>
  </DropdownMenuTrigger>
  <DropdownMenuContent>
    <DropdownMenuItem>Edit</DropdownMenuItem>
    <DropdownMenuItem>Duplicate</DropdownMenuItem>
    <DropdownMenuSeparator />
    <DropdownMenuItem className="text-red-500">Delete</DropdownMenuItem>
  </DropdownMenuContent>
</DropdownMenu>
```

**Form with validation (react-hook-form + zod)**

```tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

const schema = z.object({
  email: z.string().email(),
  name: z.string().min(2),
})

type FormData = z.infer<typeof schema>

function MyForm() {
  const form = useForm<FormData>({
    resolver: zodResolver(schema),
  })

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit((data) => console.log(data))}>
        <FormField
          control={form.control}
          name="email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input placeholder="you@example.com" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button type="submit">Submit</Button>
      </form>
    </Form>
  )
}
```

**Card + Badge (feature card)**

```tsx
<Card>
  <CardHeader>
    <div className="flex items-center justify-between">
      <CardTitle>Pro Plan</CardTitle>
      <Badge>Popular</Badge>
    </div>
    <CardDescription>For growing teams</CardDescription>
  </CardHeader>
  <CardContent>
    <p>$29/month</p>
  </CardContent>
  <CardFooter>
    <Button className="w-full">Upgrade</Button>
  </CardFooter>
</Card>
```

**Toast after async action**

```tsx
import { useToast } from '@/hooks/use-toast'

function SaveButton() {
  const { toast } = useToast()

  async function handleSave() {
    try {
      await saveData()
      toast({ title: 'Saved', description: 'Your changes were saved.' })
    } catch {
      toast({ variant: 'destructive', title: 'Error', description: 'Something went wrong.' })
    }
  }

  return <Button onClick={handleSave}>Save</Button>
}
```

### 9.8 Best Practices

1. **Do NOT modify files in `src/components/ui/` directly** — extend via composition in your own components. This makes re-adding on shadcn upgrades safe. If you need a custom variant, create a wrapper component in `src/components/your-component.tsx`.

2. **Use `asChild` for polymorphic composition** — Radix's `asChild` lets shadcn components render as any element:

   ```tsx
   <Button asChild>
     <Link href="/dashboard">Dashboard</Link>
   </Button>
   ```

3. **One file per component** — matches the shadcn convention and keeps imports predictable.

4. **Import from `@/components/ui/`** — clean aliased imports regardless of how deep the file lives.

5. **Forms: use the canonical stack** — `react-hook-form` + `zod` + `@hookform/resolvers` is what shadcn's `<Form>` component wraps. Don't roll your own validation unless you must.

6. **Prefer CSS variables for theme tokens** — set `--primary`, `--muted`, `--radius`, etc. in `globals.css`. Use Tailwind utility classes for one-off overrides only.

7. **Add a `use-toast` hook** — when you first add `toast`, shadcn creates `@/hooks/use-toast`. Keep it there — many components reference it.

### 9.9 Caveats & Gotchas

- **Re-adding a component overwrites your edits** — if you modified a shadcn source file directly, running `npx shadcn add button` again will replace your changes. Keep custom variants in separate wrapper files.
- **Not every Radix primitive is wrapped** — check the [shadcn catalog](https://ui.shadcn.com/docs/components) before hand-rolling. If it's not there, consider using the Radix primitive directly.
- **CSS variables vs. Tailwind** — theme tokens in CSS variables are global; Tailwind classes are local. Don't mix both for the same property on the same element — it makes debugging unpredictable.
- **`cn()` utility** — shadcn generates a `cn()` helper in `src/lib/utils.ts` that merges `clsx` + `tailwind-merge`. Use it anywhere you need to conditionally merge classes:

  ```tsx
  cn('text-base', isLarge && 'text-lg', className)
  ```

- **Lucide React icons** — shadcn uses `lucide-react` for icons. Install it explicitly: `npm install lucide-react`.
- **Dark mode** — requires adding a `.dark` class toggling mechanism. shadcn components read `@media (prefers-color-scheme: dark)` by default but can be driven by a class toggle. See `next-themes` or write a simple React context.

### 9.10 Key Dependencies

Add these when you first set up shadcn or start using forms:

```bash
npm install lucide-react react-hook-form @hookform/resolvers zod
```

These are installed automatically by `npx shadcn add` when needed, but it's good to know which packages do what:

| Package | Purpose |
|---|---|
| `tailwind-merge` | Merge conflicting Tailwind classes intelligently |
| `clsx` | Conditional class name construction |
| `class-variance-authority` | Component variant definitions (used internally by shadcn) |
| `lucide-react` | Icon library (used by many shadcn examples) |
| `@radix-ui/react-*` | Individual Radix primitives (installed automatically per component) |
| `react-hook-form` | Form state management |
| `@hookform/resolvers` | Bridge between react-hook-form and validation libraries |
| `zod` | Schema validation (used with shadcn's Form component) |

---

### Free Open-Source UI Elements
- **uiverse.io** — searchable collection of free open-source UI elements by category, with copy-ready code that can be customized
- Copy the code directly and customize the styles

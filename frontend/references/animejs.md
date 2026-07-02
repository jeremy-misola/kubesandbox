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

> **Note:** All following examples assume the necessary functions are imported from `'animejs'` and that standard DOM elements (canvas, common query selectors) are already available. Only the core animation/timeline logic unique to each example is shown.

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
const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+';
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
const chars = el.textContent.split('');
el.innerHTML = chars.map(c => `<span class="char">${c}</span>`).join('');

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
const chars = text.textContent.split('');
text.innerHTML = chars.map(c => `<span class="char">${c}</span>`).join('');

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
const easings = [
  'in(2)', 'out(2)', 'inOut(2)',
  'in(4)', 'out(4)', 'inOut(4)',
  'linear', 'outBounce', 'outElastic(1, .5)',
];

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
const text = 'Hello, this is a typewriter effect with irregular timing...';
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
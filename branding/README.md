# TabVM — Brand Assets

The complete visual identity for TabVM. Concept: **the tab-panel** — a live VM
console, running inside a browser tab. Cool neutrals + one teal "signal" accent.
Everything ships in light and dark cuts.

## Contents

```
branding/
├─ TabVM-Brand-Book.pdf        Complete brand book (12 pages, A4)
├─ brand-book.html             Source of the PDF (print-ready)
├─ preview.html                Interactive asset gallery with a light/dark toggle
├─ logo/
│  ├─ tabvm-logo-light.svg              full lockup, light theme
│  ├─ tabvm-logo-dark.svg               full lockup, dark theme
│  ├─ tabvm-logo-light-animated.svg     assembles once, then the caret blinks
│  └─ tabvm-logo-dark-animated.svg
├─ icon/
│  ├─ tabvm-icon-light.svg              icon / app mark, light
│  ├─ tabvm-icon-dark.svg               icon / app mark, dark
│  ├─ tabvm-icon-light-animated.svg     blinking-caret icon
│  ├─ tabvm-icon-dark-animated.svg
│  └─ favicon.svg                       theme-adaptive (follows the OS/browser)
├─ tokens/
│  ├─ tokens.css               CSS custom properties (light + dark)
│  └─ tokens.json              raw design tokens
└─ fonts/
   ├─ space-grotesk-400/500/600.woff2   display face
   └─ jetbrains-mono-400/500.woff2      mono face
```

## Colors

| Role | Light | Dark |
|------|-------|------|
| Accent (Signal) | `#0E9E8E` | `#2FE3CE` |
| Accent text on light (small) | `#0B7C6F` | — |
| Text | `#0E1116` | `#F0F3F6` |
| Background | `#FFFFFF` | `#0E1116` |
| Surface | `#F7F9FB` | `#161B22` |
| Border | `#DCE3EA` | `#21262D` |

The accent is intentionally the **only** color. Keep it under ~10% of any
surface. For small accent *text* on white, use `#0B7C6F` (Signal 700) — it clears
WCAG AA; `#0E9E8E` is for marks, icons, and large text only.

## Typography

- **Display / headings / "Tab":** Space Grotesk (600)
- **Machine / "VM" / code / labels / hex / VM IDs:** JetBrains Mono (500)

Both are open source (SIL OFL) and bundled in `fonts/`.

## Usage

```css
/* Import tokens, then theme by attribute or OS preference */
@import "branding/tokens/tokens.css";
/* <html data-theme="dark"> or let @media (prefers-color-scheme) decide */
```

- Choosing an asset: pick the file that matches the surface. Never recolor an SVG
  by hand — use the light/dark twin.
- Below 20 px, use the icon without the wordmark.
- Motion respects `prefers-reduced-motion`: the animated SVGs render static and
  complete when the user opts out.

## Regenerating the PDF

The PDF is rendered from `brand-book.html` with headless Chrome:

```bash
chrome --headless=new --no-pdf-header-footer \
  --print-to-pdf="branding/TabVM-Brand-Book.pdf" \
  "file:///ABSOLUTE/PATH/branding/brand-book.html"
```

Fonts are referenced locally from `fonts/`, so the output is self-contained.

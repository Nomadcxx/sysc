# SYSC — SVG Brand Portfolio (v1.0)

This is a production-ready **vector** asset pack based on the approved SYSC vertical lockup. All logos are SVG paths. No raster PNG images, font dependencies, linked assets, or external references are embedded. The asset geometry scales independently of pixel density.

## Use these first

- **Light webpages:** `logos/sysc-primary-transparent.svg` — black asymmetric Nested Gates mark, white vertical SYSC. wordmark. Transparent canvas and central aperture.
- **Where the site background must show through the lettering:** `logos/sysc-primary-knockout-transparent.svg` — transparent knockout lettering and aperture (one ink).
- **Dark webpages:** `logos/sysc-inverse-transparent.svg` — white logo, with transparent knockout lettering and aperture.
- **Browser favicon / small sizes:** `app-icons/sysc-icon-32.svg` — icon-only, since a four-letter vertical mark cannot remain legible at 32 px.
- **Marketing:** `banners/sysc-hero-light-1920x1080.svg` or `banners/sysc-hero-dark-1920x1080.svg` — website hero plate with copy-safe left side.
- **Inspect variants:** `preview/SYSC-logo-contact-sheet.svg` (or the PNG preview).

## Logo geometry

The asymmetrical outside curves are deliberate: tight top-left and bottom-right; generous top-right and bottom-left. The centered rounded inner aperture is open/transparent. The vertical SYSC. lettering was traced from the approved selected lockup; the aperture contour uses the supplied original design.

### Colour tokens

| Name | Hex | Purpose |
|---|---|---|
| Ink | `#080A0E` | Primary silhouette on light backgrounds |
| White | `#FFFFFF` | Reversed silhouette / lettering |
| Night | `#10151F` | Dark site UI backplate |
| Cobalt | `#174CF4` | Optional colored identity |
| Slate | `#343B4A` | Restrained neutral alternative |
| Aqua | `#52E8EC` | Experimental cyber accent |

## Usage and sizing

All logos have `viewBox` attributes and scale cleanly in CSS. Use `width: min(40vw, 28rem)` on a homepage hero for the full lockup; use the icon-only SVG for favicons and extremely small UI controls. Maintain adequate white space around the emblem. Avoid recoloring only isolated letters in the official black/white primary mark; prefer the supplied color variations.

### Simple website embed

```html
<img src="/assets/sysc/logos/sysc-primary-transparent.svg" alt="SYSC Ecosystem" width="512" height="512" />
<link rel="icon" type="image/svg+xml" href="/assets/sysc/app-icons/sysc-icon-32.svg" />
```

SVGs retain their intrinsic `width` and `height`, but you can control displayed size through CSS without any quality loss.

## Contents

- `logos/`: complete identity with white, inverse, cobalt, graphite, electric, and cyber-accent treatments.
- `marks/`: Nested Gates mark without lettering.
- `wordmark/`: standalone selected vertical SYSC. letterforms.
- `app-icons/`: 16, 32, 48, 64, 180, 192 and 512-size SVG viewports (plus cobalt/reverse app icons).
- `banners/`: 1920×1080 website compositions and 1200×630 OG images.
- `preview/`: visual comparison sheet, in both vector SVG and rendered PNG.
- `manifest.json`: asset index and colour tokens.

The brand master is intentionally monochrome. The gradient and accent variants are secondary, not substitutes for the legible primary/inverse masters.

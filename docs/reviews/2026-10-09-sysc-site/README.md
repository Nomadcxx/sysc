# SYSC documentation site review

This review covers the documentation expansion and the rendered site built from the `docs/sysc-site-design` working tree, based on `eb04891`. The site now links a guide for each of the 15 projects in the source record, including sysc-Go.

The prose pass used Stop-Slop's directness, active-voice and filler checks. It removed stock openers, empty emphasis, em dashes and formulaic contrast from the reviewed copy. Project claims keep current source behavior separate from the SYSC v0.1.1 release pins.

## Build checks

From `docs-site/`, both `npm run check` and `DOCS_BASE_PATH=/sysc npm run check` passed. They validate the catalog snapshot, content and navigation, MDX types, static export and exported links. Webpack can print a Fumadocs dynamic-import cache warning; the build and export checks still pass.

## Browser and accessibility checks

Chromium 153.0.8010.52 sampled the home page, install page, sysc-Go guide, sysc-lock guide, plugin catalog and wallpaper guide at 1440, 720 and 390 CSS pixels. Every route returned 200. None produced document-level horizontal overflow. Code samples scroll within their own containers.

The desktop article measure is 589 px. At 390 px, the article measure is 343 px. Visible buttons and inputs meet the 24 px web target check; the mobile search and navigation controls are 44 px square. Text contrast on the black canvas measured 14.9:1 for body text, 7.4:1 for links and 8.3:1 for muted text.

The first Tab reaches “Skip to main content.” Enter moves focus to the main landmark. The mobile drawer opens with Enter, closes with Escape and returns focus to its trigger. Ctrl+K opens search and a `sysc-Go` query returns the new guide. Reduced-motion mode sets animation and transition durations to 0.01 ms.

Full-page captures use 1440×1000 for desktop and 390×844 for mobile:

- Home: [desktop](home-desktop.png), [mobile](home-mobile.png)
- Install: [desktop](install-desktop.png), [mobile](install-mobile.png)
- sysc-shell: [desktop](sysc-shell-desktop.png), [mobile](sysc-shell-mobile.png)
- Plugin authoring: [desktop](plugin-authoring-desktop.png), [mobile](plugin-authoring-mobile.png)
- sysc-Go: [desktop](sysc-go-1440.png), [mobile](sysc-go-390.png)
- sysc-lock: [desktop](sysc-lock-1440.png), [mobile](sysc-lock-390.png)

The review used Chromium and DOM-level keyboard checks. It did not test a screen reader, another browser engine or a physical touch device.

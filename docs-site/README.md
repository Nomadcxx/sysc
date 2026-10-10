# SYSC documentation site

This directory holds the Next.js/Fumadocs source. The build exports HTML, local assets, and an Orama search index for GitHub Pages.

## Work locally

    npm ci
    npm run dev

Run the type and export checks at the root path and at the GitHub Pages path:

    npm run check
    DOCS_BASE_PATH=/sysc npm run check

DOCS_BASE_PATH defaults to an empty string. Set it to /sysc for Pages output. Put out/ beneath a local sysc/ directory to preview that export at /sysc/docs/.

## CI preview

The `docs-check` workflow runs on every pull request and push to `main`. A pull request run uploads `docs-preview-<PR number>` for seven days. A successful push to `main` uploads the static Pages artifact, deploys it, then checks the published docs, install page and sysc-Go guide. GitHub Pages must use **GitHub Actions** as its source.

Download a pull request ZIP from the workflow run's Artifacts section to `~/Downloads`, then extract the static files under a temporary `sysc/` directory and serve that root:

    preview_root="$(mktemp -d)"
    mkdir -p "$preview_root/sysc"
    python3 -m zipfile -e "$HOME/Downloads/docs-preview-123.zip" "$preview_root/sysc"
    python3 -m http.server 8000 --directory "$preview_root"

Open `http://localhost:8000/sysc/docs/`. Replace `123` with the pull request number in the artifact filename. The `out/` files go under `sysc/` because `/sysc` prefixes browser URLs but does not add a `sysc/` directory inside the export. Stop the server with Ctrl-C. Pull request runs build a preview but do not deploy it.

## Source and release policy

Use each component's current default branch for behavior guidance. Name the exact component releases pinned by the SYSC installer release in installation steps. Commit links identify the revision behind each documentation review. Update the affected guidance when source behavior changes. Write “unknown” when source does not establish support or qualification.

The source record covers installer release v0.1.1. Read its component pins in [internal/pin/pin.json at tag v0.1.1](https://github.com/Nomadcxx/sysc/blob/v0.1.1/internal/pin/pin.json). On 2026-10-09, we checked the default branch refs listed in the table. We excluded dirty local work and feature branches.

The sidebar has five groups: Start, Guides, Components, Developers, and Plugins. Fumadocs meta.json files hold the route inventory. Add each page to its section metadata with the content.

## Shell attribution and license

We started from sysc-greet commit [4c98abc5bdddbf6c84d2a55d71b9120cbfccad0f](https://github.com/Nomadcxx/sysc-greet/commit/4c98abc5bdddbf6c84d2a55d71b9120cbfccad0f). Its package lock has SHA-256 84e207224241e0c9cad3992ffcba24c098f6aa743ff9f70ddf50afddc012cb80. The latest successful sysc-greet docs deployment we found used commit [d927105097d79259a24493d20944dd3ae5fdd4b3](https://github.com/Nomadcxx/sysc-greet/commit/d927105097d79259a24493d20944dd3ae5fdd4b3) ([run 37809785059](https://github.com/Nomadcxx/sysc-greet/actions/runs/37809785059)). That commit records deployment history; the first commit records the code we reused.

The source project licenses the application under GPL-3.0. We kept that license in [LICENSE](./LICENSE). The site uses the SYSC logo in `public/sysc-logo.png`, the same file as `assets/logo.png` in this repository, and a favicon cut from its first letter in `app/icon.png`. We removed the greet pages and unrelated artwork, then wrote the SYSC docs. Fontsource packages supply the typefaces.

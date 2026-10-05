#!/bin/sh
# SYSC suite installer fetcher. Downloads the pinned installer for this
# architecture, verifies it against the release SHA256SUMS, and runs it.
# Do not curl this until the first release exists.
set -eu

REPO="Nomadcxx/sysc"
BASE="https://github.com/$REPO/releases/latest/download"

case "$(uname -m)" in
  x86_64|amd64) ASSET=sysc-linux-amd64 ;;
  aarch64|arm64) ASSET=sysc-linux-arm64 ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

TMP="${TMPDIR:-/tmp}/sysc-install.$$"
mkdir -p "$TMP"
curl -fsSL "$BASE/$ASSET" -o "$TMP/sysc"
curl -fsSL "$BASE/SHA256SUMS" -o "$TMP/SHA256SUMS"
grep " $ASSET\$" "$TMP/SHA256SUMS" > "$TMP/check"
(cd "$TMP" && sha256sum -c check)
chmod +x "$TMP/sysc"

if [ -t 0 ]; then
  exec "$TMP/sysc" "$@"
fi
exec "$TMP/sysc" --yes "$@"

#!/bin/sh
# SYSC suite installer fetcher. Downloads the pinned installer for this
# architecture, verifies it against the release SHA256SUMS, and runs it.
set -eu

REPO="Nomadcxx/sysc"
BASE="https://github.com/$REPO/releases/latest/download"

case "$(uname -m)" in
  x86_64|amd64) ASSET=sysc-linux-amd64 ;;
  aarch64|arm64) ASSET=sysc-linux-arm64 ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

TMP=$(mktemp -d "${TMPDIR:-/tmp}/sysc-install.XXXXXXXX")
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

BIN="$TMP/$ASSET"
curl -fsSL "$BASE/$ASSET" -o "$BIN"
curl -fsSL "$BASE/SHA256SUMS" -o "$TMP/SHA256SUMS"
grep " $ASSET\$" "$TMP/SHA256SUMS" > "$TMP/check"
(cd "$TMP" && sha256sum -c check)
chmod 700 "$BIN"

status=0
if [ -t 0 ]; then
  "$BIN" "$@" || status=$?
else
  case "${1:-}" in
    uninstall) shift; set -- uninstall --yes "$@" ;;
    *) set -- --yes "$@" ;;
  esac
  "$BIN" "$@" || status=$?
fi
exit "$status"

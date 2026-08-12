#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOL_DIR="$ROOT_DIR/executas/mini-notes-summarizer"
DIST_DIR="$ROOT_DIR/dist/release"
NAME="mini-notes-summarizer"
VERSION="0.1.0"
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

build_one() {
  local platform="$1" goos="$2" goarch="$3" ext="$4" format="$5"
  local work="$DIST_DIR/work-$platform"
  rm -rf "$work"
  mkdir -p "$work/bin"
  (cd "$TOOL_DIR" && GOCACHE="${GOCACHE:-/tmp/anna-go-build-cache}" GOOS="$goos" GOARCH="$goarch" go build -ldflags "-s -w" -o "$work/bin/$NAME$ext" ./cmd/mini-notes-summarizer)
  cat > "$work/manifest.json" <<MANIFEST
{
  "name": "$NAME",
  "version": "$VERSION",
  "runtime": {
    "binary": {
      "entrypoint": {
        "default": "bin/$NAME$ext",
        "$platform": "bin/$NAME$ext"
      },
      "permissions": {
        "bin/$NAME$ext": "0o755"
      }
    }
  }
}
MANIFEST
  local archive="$DIST_DIR/$NAME-$platform.$format"
  if [[ "$format" == "zip" ]]; then
    (cd "$work" && zip -qr "$archive" .)
  else
    chmod 755 "$work/bin/$NAME$ext"
    tar -C "$work" -czf "$archive" .
  fi
  echo "$archive"
}

build_one darwin-arm64 darwin arm64 "" tar.gz
build_one darwin-x86_64 darwin amd64 "" tar.gz
build_one windows-x86_64 windows amd64 ".exe" zip

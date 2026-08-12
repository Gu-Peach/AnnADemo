#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOL_DIR="$ROOT_DIR/executas/mini-notes-summarizer"
DIST_DIR="$ROOT_DIR/dist/executa"
NAME="mini-notes-summarizer"
VERSION="0.1.0"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$os/$arch" in
  darwin/arm64) platform="darwin-arm64"; goos="darwin"; goarch="arm64"; ext=""; format="tar.gz" ;;
  darwin/x86_64) platform="darwin-x86_64"; goos="darwin"; goarch="amd64"; ext=""; format="tar.gz" ;;
  linux/x86_64) platform="linux-x86_64"; goos="linux"; goarch="amd64"; ext=""; format="tar.gz" ;;
  linux/aarch64|linux/arm64) platform="linux-aarch64"; goos="linux"; goarch="arm64"; ext=""; format="tar.gz" ;;
  mingw*/x86_64|msys*/x86_64|cygwin*/x86_64) platform="windows-x86_64"; goos="windows"; goarch="amd64"; ext=".exe"; format="zip" ;;
  *) echo "unsupported platform: $os/$arch" >&2; exit 1 ;;
esac

rm -rf "$DIST_DIR/work-$platform"
mkdir -p "$DIST_DIR/work-$platform/bin" "$DIST_DIR/archives"

(
  cd "$TOOL_DIR"
  GOCACHE="${GOCACHE:-/private/tmp/anna-go-build-cache}" GOOS="$goos" GOARCH="$goarch" go build -ldflags "-s -w" -o "$DIST_DIR/work-$platform/bin/$NAME$ext" ./cmd/mini-notes-summarizer
)

cat > "$DIST_DIR/work-$platform/manifest.json" <<MANIFEST
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

archive="$DIST_DIR/archives/$NAME-$platform.$format"
rm -f "$archive"
if [[ "$format" == "zip" ]]; then
  (cd "$DIST_DIR/work-$platform" && zip -qr "$archive" .)
else
  chmod 755 "$DIST_DIR/work-$platform/bin/$NAME$ext"
  tar -C "$DIST_DIR/work-$platform" -czf "$archive" .
fi

printf '%s\n' "$archive"

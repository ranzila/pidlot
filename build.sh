#!/usr/bin/env bash
# pidlot cross-platform build.
# Builds every target of the matrix into ./build/<platform>/ (created here).
#
# Usage:
#   ./build.sh                  # build the full matrix
#   ./build.sh darwin           # only one platform
#   ./build.sh darwin/arm64     # only one platform/arch
#
# GO can be overridden (e.g. GO=go.exe ./build.sh in WSL).
set -euo pipefail
cd "$(dirname "$0")"

GO=${GO:-go}
LDFLAGS="-s -w"

# platform folder | GOOS | GOARCH
MATRIX=(
  "win|windows|amd64"
  "win|windows|arm64"
  "darwin|darwin|amd64"
  "darwin|darwin|arm64"
  "linux|linux|amd64"
  "linux|linux|arm64"
)

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=("") # select all
fi

built=0
for sel in "${targets[@]}"; do
  for entry in "${MATRIX[@]}"; do
    IFS='|' read -r folder os arch <<< "$entry"
    if [ -n "$sel" ]; then
      case "$sel" in
        "$os"|"$folder") ;;
        "$os/$arch"|"$folder/$arch") ;;
        *) continue ;;
      esac
    fi
    out="build/$folder/pidlot-$os-$arch"
    ext=""
    if [ "$os" = "windows" ]; then ext=".exe"; fi
    mkdir -p "build/$folder"
    echo "==> building $os/$arch -> $out$ext"
    GOOS=$os GOARCH=$arch $GO build -trimpath -ldflags "$LDFLAGS" -o "$out$ext" .
    built=$((built + 1))
  done
done

echo
echo "$built binary(ies) in ./build:"
find build -type f | sort

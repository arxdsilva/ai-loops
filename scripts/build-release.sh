#!/bin/sh
# Cross-compiles the loops binary for every supported platform into <out>
# and writes checksums.txt. The version is whatever .claude-plugin/plugin.json
# says in the checked-out tree (the binary embeds it).
# usage: scripts/build-release.sh <tag> <out-dir>
set -eu
tag=${1:?tag}
out=${2:?out dir}
mkdir -p "$out"
version=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' .claude-plugin/plugin.json | head -n 1)
if [ "v$version" != "$tag" ]; then
  echo "build-release: tag $tag does not match plugin.json version $version" >&2
  exit 1
fi
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  name="loops-$os-$arch"
  [ "$os" = windows ] && name="$name.exe"
  echo "build-release: $name"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "$out/$name" ./cmd/loops
done
cd "$out"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum loops-* > checksums.txt
else
  shasum -a 256 loops-* > checksums.txt
fi
cat checksums.txt

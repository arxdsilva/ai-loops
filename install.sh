#!/bin/sh
# loops installer: downloads the prebuilt loops binary for this OS/arch from
# the GitHub release, verifies its sha256 and installs it (no sudo, no Go).
#
#   curl -fsSL https://raw.githubusercontent.com/arxdsilva/ai-loops/main/install.sh | sh -s -- \
#     [--version vX.Y.Z] [--target <platform>] [--scope project|user] [--bin-dir dir] [--force]
#
# With --target it then runs `loops install --target <platform>` in the current
# directory. Env: LOOPS_RELEASE_BASE (asset host, default the GitHub releases
# download URL), LOOPS_LATEST_URL (redirect used to find the latest tag),
# LOOPS_SKIP_VERIFY=1 (allow installing without sha256sum/shasum).
set -e

repo='arxdsilva/ai-loops'
base="${LOOPS_RELEASE_BASE:-https://github.com/$repo/releases/download}"
latest_url="${LOOPS_LATEST_URL:-https://github.com/$repo/releases/latest}"
version=
target=
scope=
force=
bindir="$HOME/.local/bin"

die() {
  echo "loops install: $*" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case $1 in
    --version) [ $# -ge 2 ] || die "--version needs a value"; version=$2; shift 2 ;;
    --target) [ $# -ge 2 ] || die "--target needs a value"; target=$2; shift 2 ;;
    --scope) [ $# -ge 2 ] || die "--scope needs a value"; scope=$2; shift 2 ;;
    --bin-dir) [ $# -ge 2 ] || die "--bin-dir needs a value"; bindir=$2; shift 2 ;;
    --force) force=1; shift ;;
    -h | --help) sed -n '2,11p' "$0" 2>/dev/null; exit 0 ;;
    *) die "unknown option $1" ;;
  esac
done

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 2 -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    die "need curl or wget"
  fi
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

if [ "${LOOPS_SKIP_VERIFY:-}" != 1 ] && ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  die "neither sha256sum nor shasum found; refusing an unverified download (set LOOPS_SKIP_VERIFY=1 to override)"
fi

ext=
os=$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]')
case $os in
  darwin | linux) ;;
  mingw* | msys* | cygwin*) os=windows ext=.exe ;;
  *) die "unsupported OS $os; see https://github.com/$repo/blob/main/docs/GUIDE.md#install" ;;
esac
arch=$(uname -m 2>/dev/null)
case $arch in
  x86_64 | amd64 | AMD64) arch=amd64 ;;
  arm64 | aarch64 | ARM64) arch=arm64 ;;
  *) die "unsupported architecture $arch; see https://github.com/$repo/blob/main/docs/GUIDE.md#install" ;;
esac

if [ -z "$version" ]; then
  if command -v curl >/dev/null 2>&1; then
    loc=$(curl -sI "$latest_url" | awk 'tolower($1) == "location:" {print $2}' | tail -n 1)
  elif command -v wget >/dev/null 2>&1; then
    loc=$(wget -S --spider --max-redirect=0 "$latest_url" 2>&1 | awk 'tolower($1) == "location:" {print $2}' | tail -n 1)
  else
    die "need curl or wget"
  fi
  loc=$(printf '%s' "$loc" | tr -d '\r')
  version=${loc##*/}
  [ -n "$version" ] || die "could not resolve the latest release from $latest_url"
fi
case $version in v*) ;; *) version=v$version ;; esac

asset="loops-$os-$arch$ext"
url="$base/$version"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/loops-install.XXXXXX") || die "mktemp failed"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

fetch "$url/$asset" "$tmp/$asset" || die "no release binary $asset for $version ($url); see https://github.com/$repo/blob/main/docs/GUIDE.md#install"
if [ "${LOOPS_SKIP_VERIFY:-}" != 1 ]; then
  fetch "$url/checksums.txt" "$tmp/checksums.txt" || die "could not fetch checksums.txt for $version; refusing an unverified binary"
  want=$(awk -v n="$asset" '$2 == n || $2 == "*" n {print $1; exit}' "$tmp/checksums.txt")
  got=$(sha256 "$tmp/$asset")
  if [ -z "$want" ] || [ "$want" != "$got" ]; then
    die "checksum mismatch for $asset $version; refusing to install it"
  fi
fi

mkdir -p "$bindir" || die "cannot create $bindir"
chmod +x "$tmp/$asset"
mv -f "$tmp/$asset" "$bindir/.loops-new$ext"
mv -f "$bindir/.loops-new$ext" "$bindir/loops$ext"
echo "loops $version installed to $bindir/loops$ext"
"$bindir/loops$ext" version

case ":$PATH:" in
  *":$bindir:"*) ;;
  *) echo "loops install: warning: $bindir is not on your PATH; add it (for example: export PATH=\"$bindir:\$PATH\")" >&2 ;;
esac

if [ -n "$target" ]; then
  set -- install --target "$target"
  [ -z "$scope" ] || set -- "$@" --scope "$scope"
  [ -z "$force" ] || set -- "$@" --force
  "$bindir/loops$ext" "$@"
fi

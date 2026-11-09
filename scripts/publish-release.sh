#!/bin/sh
# Creates (or updates) the GitHub Release for <tag> and uploads every file in
# <dir>. Needs GH_TOKEN with contents:write.
# usage: scripts/publish-release.sh <tag> <dir>
set -eu
tag=${1:?tag}
dir=${2:?dir}
if gh release view "$tag" >/dev/null 2>&1; then
  gh release upload "$tag" "$dir"/* --clobber
else
  gh release create "$tag" "$dir"/* --title "loops $tag" --generate-notes --verify-tag
fi

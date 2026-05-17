#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s vX.Y.Z [--allow-missing-notes]\n' "$0" >&2
}

version=${1:-}
allow_missing_notes=0
if [[ "${ALLOW_MISSING_RELEASE_NOTES:-}" == "1" ]]; then
  allow_missing_notes=1
fi

if [[ -z "$version" ]]; then
  usage
  exit 2
fi

case "${2:-}" in
  "") ;;
  --allow-missing-notes) allow_missing_notes=1 ;;
  *)
    usage
    exit 2
    ;;
esac

if [[ -n "${3:-}" ]]; then
  usage
  exit 2
fi

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
  printf 'invalid version tag: %s\n' "$version" >&2
  exit 2
fi

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/.." && pwd)
cd "$repo_root"

notes_file="docs/releases/$version.md"
if [[ "$allow_missing_notes" != "1" && ! -f "$notes_file" ]]; then
  printf 'release notes file missing: %s\n' "$notes_file" >&2
  printf 'create and commit it before tagging\n' >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  printf 'working tree has uncommitted changes; commit or stash them before tagging\n' >&2
  exit 1
fi

"$repo_root/scripts/ci-local.sh" clean

git tag "$version"
SKIP_CI_LOCAL_ON_PRE_PUSH=1 git push origin "$version"

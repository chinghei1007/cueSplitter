#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if ! "$repo_dir/scripts/setup.sh" --check >/dev/null 2>&1; then
  "$repo_dir/scripts/setup.sh"
fi

cd "$repo_dir/frontend"
npm ci
npm run build

cd "$repo_dir"
mkdir -p bin
go build -o bin/cue-splitter ./cmd/cuesplitter

printf 'Built %s/bin/cue-splitter\n' "$repo_dir"

#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
app="$repo_dir/bin/cue-splitter"

cd "$repo_dir"

if [ ! -x "$app" ]; then
  printf 'Cue Splitter has not been built yet. Building it now...\n'
  "$repo_dir/scripts/build.sh"
else
  "$repo_dir/scripts/setup.sh" --runtime-only
fi

printf 'Starting Cue Splitter at http://127.0.0.1:8080\n'
printf 'Press Ctrl+C to stop it.\n'
exec "$app" "$@"

#!/bin/sh
set -eu

mode="install"
if [ "${1:-}" = "--check" ]; then
  mode="check"
elif [ "${1:-}" = "--runtime-only" ]; then
  mode="runtime"
elif [ -n "${1:-}" ]; then
  printf 'Usage: %s [--check|--runtime-only]\n' "$0" >&2
  exit 2
fi

need_go=0
need_node=0
need_ffmpeg=0

if [ "$mode" != "runtime" ]; then
  command -v go >/dev/null 2>&1 || need_go=1
  if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    need_node=1
  fi
fi
if ! command -v ffmpeg >/dev/null 2>&1 || ! command -v ffprobe >/dev/null 2>&1; then
  need_ffmpeg=1
fi

print_status() {
  if [ "$mode" != "runtime" ]; then
    if command -v go >/dev/null 2>&1; then printf 'Go: %s\n' "$(go version)"; else printf 'Go: missing\n'; fi
    if command -v node >/dev/null 2>&1; then printf 'Node: %s\n' "$(node --version)"; else printf 'Node: missing\n'; fi
    if command -v npm >/dev/null 2>&1; then printf 'npm: %s\n' "$(npm --version)"; else printf 'npm: missing\n'; fi
  fi
  if command -v ffmpeg >/dev/null 2>&1; then printf 'FFmpeg: available\n'; else printf 'FFmpeg: missing\n'; fi
  if command -v ffprobe >/dev/null 2>&1; then printf 'FFprobe: available\n'; else printf 'FFprobe: missing\n'; fi
}

if [ "$need_go" -eq 0 ] && [ "$need_node" -eq 0 ] && [ "$need_ffmpeg" -eq 0 ]; then
  print_status
  printf 'Cue Splitter dependencies are ready.\n'
  exit 0
fi

if [ "$mode" = "check" ]; then
  print_status
  exit 1
fi

run_as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    printf 'Administrator access is required to install missing packages.\n' >&2
    exit 1
  fi
}

system=$(uname -s)
case "$system" in
  Darwin)
    if ! command -v brew >/dev/null 2>&1; then
      printf 'Homebrew is required for automatic installation on macOS. Install it from https://brew.sh and rerun this script.\n' >&2
      exit 1
    fi
    packages=""
    [ "$need_go" -eq 1 ] && packages="$packages go"
    [ "$need_node" -eq 1 ] && packages="$packages node"
    [ "$need_ffmpeg" -eq 1 ] && packages="$packages ffmpeg"
    # shellcheck disable=SC2086
    brew install $packages
    ;;
  Linux)
    if command -v apt-get >/dev/null 2>&1; then
      packages=""
      [ "$need_go" -eq 1 ] && packages="$packages golang-go"
      [ "$need_node" -eq 1 ] && packages="$packages nodejs npm"
      [ "$need_ffmpeg" -eq 1 ] && packages="$packages ffmpeg"
      run_as_root apt-get update
      # shellcheck disable=SC2086
      run_as_root apt-get install -y $packages
    elif command -v dnf >/dev/null 2>&1; then
      packages=""
      [ "$need_go" -eq 1 ] && packages="$packages golang"
      [ "$need_node" -eq 1 ] && packages="$packages nodejs npm"
      [ "$need_ffmpeg" -eq 1 ] && packages="$packages ffmpeg"
      # shellcheck disable=SC2086
      run_as_root dnf install -y $packages
    elif command -v yum >/dev/null 2>&1; then
      packages=""
      [ "$need_go" -eq 1 ] && packages="$packages golang"
      [ "$need_node" -eq 1 ] && packages="$packages nodejs npm"
      [ "$need_ffmpeg" -eq 1 ] && packages="$packages ffmpeg"
      # shellcheck disable=SC2086
      run_as_root yum install -y $packages
    elif command -v pacman >/dev/null 2>&1; then
      packages=""
      [ "$need_go" -eq 1 ] && packages="$packages go"
      [ "$need_node" -eq 1 ] && packages="$packages nodejs npm"
      [ "$need_ffmpeg" -eq 1 ] && packages="$packages ffmpeg"
      # shellcheck disable=SC2086
      run_as_root pacman -Sy --needed --noconfirm $packages
    elif command -v apk >/dev/null 2>&1; then
      packages=""
      [ "$need_go" -eq 1 ] && packages="$packages go"
      [ "$need_node" -eq 1 ] && packages="$packages nodejs npm"
      [ "$need_ffmpeg" -eq 1 ] && packages="$packages ffmpeg"
      # shellcheck disable=SC2086
      run_as_root apk add $packages
    elif command -v zypper >/dev/null 2>&1; then
      packages=""
      [ "$need_go" -eq 1 ] && packages="$packages go"
      [ "$need_node" -eq 1 ] && packages="$packages nodejs npm"
      [ "$need_ffmpeg" -eq 1 ] && packages="$packages ffmpeg"
      # shellcheck disable=SC2086
      run_as_root zypper --non-interactive install $packages
    else
      printf 'No supported package manager was found. Install Go, Node.js/npm, and FFmpeg manually, then rerun.\n' >&2
      exit 1
    fi
    ;;
  *)
    printf 'Unsupported system %s. On Windows, run scripts/setup.ps1 in PowerShell.\n' "$system" >&2
    exit 1
    ;;
esac

if [ "$mode" != "runtime" ]; then
  command -v go >/dev/null 2>&1 || { printf 'Go installation did not become available on PATH.\n' >&2; exit 1; }
  command -v node >/dev/null 2>&1 || { printf 'Node.js installation did not become available on PATH.\n' >&2; exit 1; }
  command -v npm >/dev/null 2>&1 || { printf 'npm installation did not become available on PATH.\n' >&2; exit 1; }
fi
command -v ffmpeg >/dev/null 2>&1 || { printf 'FFmpeg installation did not become available on PATH.\n' >&2; exit 1; }
command -v ffprobe >/dev/null 2>&1 || { printf 'FFprobe installation did not become available on PATH.\n' >&2; exit 1; }

print_status
printf 'Cue Splitter dependencies are ready.\n'

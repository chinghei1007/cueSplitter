#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
e2e_dir=$(mktemp -d "${TMPDIR:-/tmp}/cue-splitter-e2e.XXXXXX")
server_pid=""
addr=${E2E_ADDR:-127.0.0.1:18080}
base_url="http://$addr"

cd "$repo_dir"

cleanup() {
  if [ -n "$server_pid" ]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$e2e_dir"
}
trap cleanup EXIT INT TERM

mkdir -p "$e2e_dir/root"
printf '%s\n' '01. First title' '02) Second title' '03、 Third title' > "$e2e_dir/valid.txt"
printf '%s\n' 'not numbered' '02. Second title' '03. Third title' > "$e2e_dir/malformed.txt"
iconv -f UTF-8 -t BIG5 "$repo_dir/testdata/album/tracks-big5-source.txt" > "$e2e_dir/big5.txt"
printf '%s\n' '01. 你装做不知道' '02. 相见在明天' '03. 我不再迷惘' | iconv -f UTF-8 -t GB18030 > "$e2e_dir/gb18030.txt"
printf '%s\n' '01. 第一曲' '02. 第二曲' '03. 第三曲' | iconv -f UTF-8 -t SHIFT_JIS > "$e2e_dir/shift-jis.txt"

npm --prefix frontend run build >/dev/null
go build -o "$e2e_dir/cue-splitter" ./cmd/cuesplitter
"$e2e_dir/cue-splitter" -addr "$addr" -root "$e2e_dir/root" >"$e2e_dir/server.log" 2>&1 &
server_pid=$!

attempt=0
until curl -fsS "$base_url/api/health" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 80 ]; then
    printf '%s\n' 'E2E server did not become ready.' >&2
    cat "$e2e_dir/server.log" >&2
    exit 1
  fi
  sleep 0.1
done

post_job() {
  text_file=$1
  manifest=$2
  if [ -n "$text_file" ]; then
    curl -fsS \
      -F "files=@$repo_dir/testdata/album/album.cue" \
      -F "files=@$repo_dir/testdata/album/album.flac" \
      -F "files=@$text_file;filename=music.txt" \
      -F "manifest=$manifest" \
      "$base_url/api/jobs"
  else
    curl -fsS \
      -F "files=@$repo_dir/testdata/album/album.cue" \
      -F "files=@$repo_dir/testdata/album/album.flac" \
      -F "manifest=$manifest" \
      "$base_url/api/jobs"
  fi
}

wait_job() {
  job_id=$1
  attempt=0
  while [ "$attempt" -lt 120 ]; do
    snapshot=$(curl -fsS "$base_url/api/jobs/$job_id")
    state=$(printf '%s' "$snapshot" | jq -r '.state')
    if [ "$state" = "complete" ]; then
      printf '%s' "$snapshot"
      return 0
    fi
    if [ "$state" = "error" ]; then
      printf '%s\n' "$snapshot" >&2
      return 1
    fi
    attempt=$((attempt + 1))
    sleep 0.1
  done
  printf '%s\n' "Job $job_id did not finish." >&2
  return 1
}

malformed_manifest='{"trackNames":["Manual one","Manual two","Manual three"],"nameSource":"txt","textEncoding":"utf-8","format":"mp3","bitrate":320,"destination":"project","destinationPath":"","sourceDuration":6}'
malformed_code=$(curl -sS -o "$e2e_dir/malformed-response.json" -w '%{http_code}' \
  -F "files=@$repo_dir/testdata/album/album.cue" \
  -F "files=@$repo_dir/testdata/album/album.flac" \
  -F "files=@$e2e_dir/malformed.txt;filename=music.txt" \
  -F "manifest=$malformed_manifest" \
  "$base_url/api/jobs")
test "$malformed_code" = "400"
grep -q 'song-list line 1' "$e2e_dir/malformed-response.json"

prefixed_manifest='{"trackNames":["01. Manual one","Manual two","Manual three"],"nameSource":"txt","textEncoding":"utf-8","format":"mp3","bitrate":320,"destination":"project","destinationPath":"","sourceDuration":6}'
prefixed_code=$(curl -sS -o "$e2e_dir/prefixed-response.json" -w '%{http_code}' \
  -F "files=@$repo_dir/testdata/album/album.cue" \
  -F "files=@$repo_dir/testdata/album/album.flac" \
  -F "files=@$e2e_dir/valid.txt;filename=music.txt" \
  -F "manifest=$prefixed_manifest" \
  "$base_url/api/jobs")
test "$prefixed_code" = "400"
grep -q 'already has its row number' "$e2e_dir/prefixed-response.json"

unselected_manifest='{"trackNames":["Cue edited one","Cue edited two","Cue edited three"],"nameSource":"cue","textEncoding":"utf-8","albumTitle":"","artist":"","format":"original","bitrate":0,"destination":"project","destinationPath":"","sourceDuration":6}'
unselected_response=$(post_job "$e2e_dir/shift-jis.txt" "$unselected_manifest")
unselected_id=$(printf '%s' "$unselected_response" | jq -r '.id')
unselected_snapshot=$(wait_job "$unselected_id")
unselected_output=$(printf '%s' "$unselected_snapshot" | jq -r '.outputPath')
test -f "$unselected_output/Cue edited one.flac"

valid_manifest='{"trackNames":["First edited","Second title","Third title"],"nameSource":"txt","textEncoding":"utf-8","albumTitle":"Edited Album","artist":"Edited Artist","format":"original","bitrate":0,"destination":"project","destinationPath":"","sourceDuration":6}'
valid_response=$(post_job "$e2e_dir/valid.txt" "$valid_manifest")
valid_id=$(printf '%s' "$valid_response" | jq -r '.id')
valid_snapshot=$(wait_job "$valid_id")
valid_output=$(printf '%s' "$valid_snapshot" | jq -r '.outputPath')
test -f "$valid_output/First edited.flac"
test -f "$valid_output/Second title.flac"
test -f "$valid_output/Third title.flac"
test -f "$valid_output/cover.png"
test "$(od -An -t x1 -N3 "$valid_output/tracklist.txt" | tr -d ' \n')" = "efbbbf"
test "$(tail -c +4 "$valid_output/tracklist.txt")" = "$(printf '01. First edited\n02. Second title\n03. Third title')"
ffprobe -v error -show_entries format_tags=title,album,artist,album_artist,track -of json "$valid_output/First edited.flac" > "$e2e_dir/tags.json"
jq -e '(.format.tags.title // .format.tags.TITLE) == "First edited"' "$e2e_dir/tags.json" >/dev/null
jq -e '(.format.tags.album // .format.tags.ALBUM) == "Edited Album"' "$e2e_dir/tags.json" >/dev/null
jq -e '(.format.tags.artist // .format.tags.ARTIST) == "Edited Artist"' "$e2e_dir/tags.json" >/dev/null
jq -e '(.format.tags.album_artist // .format.tags.ALBUM_ARTIST) == "Edited Artist"' "$e2e_dir/tags.json" >/dev/null
jq -e '((.format.tags.track // .format.tags.TRACKNUMBER) | startswith("1"))' "$e2e_dir/tags.json" >/dev/null

generated_manifest='{"trackNames":[],"nameSource":"generated","textEncoding":"auto","format":"original","bitrate":0,"destination":"project","destinationPath":"","sourceDuration":6}'
generated_response=$(post_job "" "$generated_manifest")
generated_id=$(printf '%s' "$generated_response" | jq -r '.id')
generated_snapshot=$(wait_job "$generated_id")
generated_output=$(printf '%s' "$generated_snapshot" | jq -r '.outputPath')
test "$(tail -c +4 "$generated_output/tracklist.txt")" = "$(printf '01. track01\n02. track02\n03. track03')"

big5_manifest='{"trackNames":[],"nameSource":"txt","textEncoding":"big5","format":"original","bitrate":0,"destination":"project","destinationPath":"","sourceDuration":6}'
big5_response=$(post_job "$e2e_dir/big5.txt" "$big5_manifest")
big5_id=$(printf '%s' "$big5_response" | jq -r '.id')
big5_snapshot=$(wait_job "$big5_id")
big5_output=$(printf '%s' "$big5_snapshot" | jq -r '.outputPath')
test "$(tail -c +4 "$big5_output/tracklist.txt")" = "$(printf '01. 第一首歌\n02. 第二首歌\n03. 第三首歌')"

gb18030_manifest='{"trackNames":[],"nameSource":"txt","textEncoding":"gb18030","format":"original","bitrate":0,"destination":"project","destinationPath":"","sourceDuration":6}'
gb18030_response=$(post_job "$e2e_dir/gb18030.txt" "$gb18030_manifest")
gb18030_id=$(printf '%s' "$gb18030_response" | jq -r '.id')
gb18030_snapshot=$(wait_job "$gb18030_id")
gb18030_output=$(printf '%s' "$gb18030_snapshot" | jq -r '.outputPath')
test "$(tail -c +4 "$gb18030_output/tracklist.txt")" = "$(printf '01. 你装做不知道\n02. 相见在明天\n03. 我不再迷惘')"

shift_jis_manifest='{"trackNames":[],"nameSource":"txt","textEncoding":"shift_jis","format":"original","bitrate":0,"destination":"project","destinationPath":"","sourceDuration":6}'
shift_jis_response=$(post_job "$e2e_dir/shift-jis.txt" "$shift_jis_manifest")
shift_jis_id=$(printf '%s' "$shift_jis_response" | jq -r '.id')
shift_jis_snapshot=$(wait_job "$shift_jis_id")
shift_jis_output=$(printf '%s' "$shift_jis_snapshot" | jq -r '.outputPath')
test "$(tail -c +4 "$shift_jis_output/tracklist.txt")" = "$(printf '01. 第一曲\n02. 第二曲\n03. 第三曲')"

printf '%s\n' 'E2E passed: selected-source validation, UTF-8, GB18030, Big5, Shift_JIS, generated names, Original export, normalized TXT, and audio metadata tags passed.'

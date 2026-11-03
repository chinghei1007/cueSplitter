package jobs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cuesplitter/internal/cue"
)

func TestResolveTrackNamesUsesValidatedTXT(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "music.txt")
	if err := os.WriteFile(path, []byte("01. First song\n02) Second song\n03、 Third song\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tracks := []cue.Track{{Number: 1}, {Number: 2}, {Number: 3}}
	got, err := resolveTrackNames(Manifest{NameSource: "txt", TextEncoding: "utf-8"}, InputFiles{Text: path}, tracks)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"First song", "Second song", "Third song"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestResolveTrackNamesIgnoresMalformedUnselectedTXT(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "music.txt")
	if err := os.WriteFile(path, []byte("not numbered\n02. Second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveTrackNames(
		Manifest{TrackNames: []string{"Manual one", "Manual two"}, NameSource: "cue", TextEncoding: "utf-8"},
		InputFiles{Cue: writeCue(t, directory), Text: path},
		[]cue.Track{{Number: 1, Title: "Cue one"}, {Number: 2, Title: "Cue two"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"Manual one", "Manual two"}) {
		t.Fatalf("unexpected names: %#v", got)
	}
}

func TestResolveTrackNamesRejectsMalformedSelectedTXT(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "music.txt")
	if err := os.WriteFile(path, []byte("not numbered\n02. Second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveTrackNames(
		Manifest{TrackNames: []string{"Manual one", "Manual two"}, NameSource: "txt", TextEncoding: "utf-8"},
		InputFiles{Text: path},
		[]cue.Track{{Number: 1}, {Number: 2}},
	)
	if err == nil || !strings.Contains(err.Error(), "song-list line 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveAlbumMetadataUsesCueFallbackAndEdits(t *testing.T) {
	directory := t.TempDir()
	cuePath := writeCue(t, directory)
	tracks := []cue.Track{{Number: 1, Title: "Cue one"}, {Number: 2, Title: "Cue two"}}
	album, artist, err := resolveAlbumMetadata(Manifest{NameSource: "cue", TextEncoding: "utf-8"}, InputFiles{Cue: cuePath}, tracks)
	if err != nil {
		t.Fatal(err)
	}
	if album != "Cue Album" || artist != "Cue Artist" {
		t.Fatalf("unexpected CUE metadata: %q / %q", album, artist)
	}
	album, artist, err = resolveAlbumMetadata(Manifest{NameSource: "cue", TextEncoding: "utf-8", AlbumTitle: "Edited Album", Artist: "Edited Artist"}, InputFiles{Cue: cuePath}, tracks)
	if err != nil || album != "Edited Album" || artist != "Edited Artist" {
		t.Fatalf("unexpected edited metadata: %q / %q / %v", album, artist, err)
	}
}

func writeCue(t *testing.T, directory string) string {
	t.Helper()
	path := filepath.Join(directory, "album.cue")
	data := "PERFORMER \"Cue Artist\"\nTITLE \"Cue Album\"\nTRACK 01 AUDIO\nTITLE \"Cue one\"\nINDEX 01 00:00:00\nTRACK 02 AUDIO\nTITLE \"Cue two\"\nINDEX 01 00:01:00\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteTrackList(t *testing.T) {
	directory := t.TempDir()
	if err := writeTrackList([]TrackStatus{{Name: "First"}, {Name: "Second"}}, directory); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "tracklist.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "\ufeff01. First\n02. Second\n" {
		t.Fatalf("unexpected track list %q", data)
	}
}

func TestCleanName(t *testing.T) {
	if got := cleanName("../bad:name?.mp3", "fallback"); got != "bad-name-.mp3" {
		t.Fatalf("unexpected cleaned name %q", got)
	}
}

func TestUniqueFilename(t *testing.T) {
	used := make(map[string]int)
	if got := uniqueFilename("Song", ".mp3", used); got != "Song.mp3" {
		t.Fatal(got)
	}
	if got := uniqueFilename("song", ".mp3", used); got != "song-2.mp3" {
		t.Fatal(got)
	}
}

package media

import (
	"reflect"
	"testing"
)

func TestExtension(t *testing.T) {
	for input, expected := range map[string]string{
		"mp3":  ".mp3",
		"FLAC": ".flac",
		"aac":  ".aac",
		"aiff": ".aiff",
	} {
		got, err := Extension(input)
		if err != nil || got != expected {
			t.Fatalf("Extension(%q) = %q, %v", input, got, err)
		}
	}
}

func TestResolveExportRejectsUnsupportedRequestedBitrate(t *testing.T) {
	processor := NewProcessor("ffmpeg", "ffprobe")
	if _, err := processor.ResolveExport(t.Context(), "album.flac", ExportOptions{Format: "mp3", Bitrate: 10}); err == nil {
		t.Fatal("expected bitrate rejection")
	}
}

func TestResolveOriginalLosslessFormat(t *testing.T) {
	processor := NewProcessor("ffmpeg", "ffprobe")
	got, err := processor.ResolveExport(t.Context(), "album.flac", ExportOptions{Format: "original"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != "flac" || got.Bitrate != 0 {
		t.Fatalf("unexpected resolved export: %#v", got)
	}
}

func TestCodecArgumentsRejectsUnknownFormat(t *testing.T) {
	if _, err := codecArguments(ExportOptions{Format: "ape"}); err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestMetadataArguments(t *testing.T) {
	got := metadataArguments(TrackMetadata{
		Title: "Song", Album: "Album", Artist: "Artist", AlbumArtist: "Artist", TrackNumber: 2, TrackTotal: 12,
	})
	want := []string{
		"-map_metadata", "-1",
		"-metadata", "title=Song",
		"-metadata", "album=Album",
		"-metadata", "artist=Artist",
		"-metadata", "album_artist=Artist",
		"-metadata", "track=2/12",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestSupportsEmbeddedCover(t *testing.T) {
	for _, format := range []string{"flac", "mp3", "m4a"} {
		if !supportsEmbeddedCover(format) {
			t.Fatalf("expected embedded cover support for %s", format)
		}
	}
	for _, format := range []string{"wav", "aiff", "aac", "ogg", "opus", "wma"} {
		if supportsEmbeddedCover(format) {
			t.Fatalf("did not expect embedded cover support for %s", format)
		}
	}
}

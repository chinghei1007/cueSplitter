package cue

import (
	"math"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	data := `REM GENRE Jazz
PERFORMER "Album Artist"
TITLE "Album Name"
TRACK 01 AUDIO
  TITLE "Ignored title"
  INDEX 01 00:00:00
TRACK 02 AUDIO
  INDEX 00 04:09:00
  INDEX 01 04:11:37
TRACK 03 AUDIO
  INDEX 01 10:00:00`
	sheet, err := ParseSheet(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tracks := sheet.Tracks
	if len(tracks) != 3 {
		t.Fatalf("got %d tracks", len(tracks))
	}
	wantBoundary := 4*60 + 11 + 37.0/75
	if math.Abs(tracks[0].EndSeconds-wantBoundary) > 0.000001 || math.Abs(tracks[1].StartSeconds-wantBoundary) > 0.000001 {
		t.Fatalf("unexpected boundary: %#v", tracks)
	}
	if tracks[2].EndSeconds != -1 {
		t.Fatalf("last end should be unknown: %#v", tracks[2])
	}
	if tracks[0].Title != "Ignored title" {
		t.Fatalf("unexpected CUE title: %#v", tracks[0])
	}
	if sheet.Title != "Album Name" || sheet.Performer != "Album Artist" {
		t.Fatalf("unexpected sheet metadata: %#v", sheet)
	}
}

func TestParseRejectsInvalidOrder(t *testing.T) {
	_, err := Parse(strings.NewReader("TRACK 01 AUDIO\nINDEX 01 01:00:00\nTRACK 02 AUDIO\nINDEX 01 00:30:00"))
	if err == nil {
		t.Fatal("expected invalid order error")
	}
}

func TestWithDuration(t *testing.T) {
	tracks := []Track{{Number: 1, StartSeconds: 0, EndSeconds: -1}}
	got := WithDuration(tracks, 123.5)
	if got[0].EndSeconds != 123.5 || tracks[0].EndSeconds != -1 {
		t.Fatalf("unexpected duration result: got=%#v original=%#v", got, tracks)
	}
}

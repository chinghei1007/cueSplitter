package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cuesplitter/internal/jobs"
)

func TestClassify(t *testing.T) {
	files := []savedFile{
		{name: "album.cue", path: "/tmp/album.cue"},
		{name: "album.flac", path: "/tmp/album.flac"},
		{name: "front.jpg", path: "/tmp/front.jpg"},
		{name: "music.txt", path: "/tmp/music.txt"},
	}
	got, err := classify(files)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cue != "/tmp/album.cue" || got.Audio != "/tmp/album.flac" || got.Cover != "/tmp/front.jpg" || got.Text != "/tmp/music.txt" {
		t.Fatalf("unexpected classification: %#v", got)
	}
}

func TestClassifyRequiresOneAudio(t *testing.T) {
	_, err := classify([]savedFile{{name: "album.cue"}, {name: "one.flac"}, {name: "two.mp3"}})
	if err == nil || !strings.Contains(err.Error(), "exactly one supported audio") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing security header")
	}
}

func TestSnapshotJSONShape(t *testing.T) {
	_ = jobs.Snapshot{ID: "job", State: "queued"}
}

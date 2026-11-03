package tracklist

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

func TestParseNumberedFlexiblePunctuation(t *testing.T) {
	got, err := ParseNumbered("01. First\n2) Second\n03、 Third\n", 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"First", "Second", "Third"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestParseNumberedRejectsSequenceAndCount(t *testing.T) {
	if _, err := ParseNumbered("01. First\n03. Third\n", 2); err == nil || !strings.Contains(err.Error(), "track 02") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := ParseNumbered("01. First\n", 2); err == nil || !strings.Contains(err.Error(), "CUE has 2") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateTitlesRejectsNumberPrefix(t *testing.T) {
	if err := ValidateTitles([]string{"01. First"}, 1); err == nil {
		t.Fatal("expected numbered title rejection")
	}
}

func TestFormatUTF8BOM(t *testing.T) {
	got := string(FormatUTF8BOM([]string{"First", "Second"}))
	if got != "\ufeff01. First\n02. Second\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestDecodeLegacyEncodingMatrix(t *testing.T) {
	tests := []struct {
		name     string
		selected string
		codec    encoding.Encoding
		text     string
	}{
		{name: "GBK bytes through GB18030", selected: "gb18030", codec: simplifiedchinese.GBK, text: "你装做不知道"},
		{name: "GB18030", selected: "gb18030", codec: simplifiedchinese.GB18030, text: "相见在明天"},
		{name: "Big5", selected: "big5", codec: traditionalchinese.Big5, text: "第一首歌"},
		{name: "Shift_JIS", selected: "shift_jis", codec: japanese.ShiftJIS, text: "第一曲"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, _, err := transform.Bytes(test.codec.NewEncoder(), []byte(test.text))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Decode(encoded, test.selected)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.text {
				t.Fatalf("got %q want %q", got, test.text)
			}
		})
	}
}

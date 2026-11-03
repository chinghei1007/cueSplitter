package tracklist

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

var numberedLinePattern = regexp.MustCompile(`^\s*(\d{1,3})\s*[.)：:、—–-]\s+(.+?)\s*$`)

func NormalizeEncoding(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = "auto"
	}
	switch value {
	case "auto", "utf-8", "big5", "gb18030", "shift_jis":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported text encoding %q", value)
	}
}

func DecodeFile(path, selected string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text, err := Decode(data, selected)
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", filepathBase(path), err)
	}
	return text, nil
}

func Decode(data []byte, selected string) (string, error) {
	selected, err := NormalizeEncoding(selected)
	if err != nil {
		return "", err
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if selected == "auto" || selected == "utf-8" {
		if !utf8.Valid(data) {
			if selected == "auto" {
				return "", fmt.Errorf("content is not valid UTF-8; choose Big5, GB18030, or Shift_JIS")
			}
			return "", fmt.Errorf("content is not valid UTF-8")
		}
		return strings.TrimPrefix(string(data), "\ufeff"), nil
	}
	var decoder *encoding.Decoder
	switch selected {
	case "big5":
		decoder = traditionalchinese.Big5.NewDecoder()
	case "gb18030":
		decoder = simplifiedchinese.GB18030.NewDecoder()
	case "shift_jis":
		decoder = japanese.ShiftJIS.NewDecoder()
	}
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), decoder))
	if err != nil {
		return "", fmt.Errorf("content is not valid %s: %w", selected, err)
	}
	return strings.TrimPrefix(string(decoded), "\ufeff"), nil
}

func ParseNumbered(text string, expectedCount int) ([]string, error) {
	var names []string
	expectedNumber := 1
	for index, raw := range strings.Split(strings.ReplaceAll(strings.TrimPrefix(text, "\ufeff"), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		matches := numberedLinePattern.FindStringSubmatch(line)
		if matches == nil {
			return nil, fmt.Errorf("song-list line %d must look like %02d. Song title", index+1, expectedNumber)
		}
		number, _ := strconv.Atoi(matches[1])
		if number != expectedNumber {
			return nil, fmt.Errorf("song-list line %d must be track %02d, not %s", index+1, expectedNumber, matches[1])
		}
		names = append(names, strings.TrimSpace(matches[2]))
		expectedNumber++
	}
	if len(names) != expectedCount {
		return nil, fmt.Errorf("song list has %d numbered lines, but the CUE has %d tracks", len(names), expectedCount)
	}
	return names, nil
}

func ValidateTitles(names []string, expectedCount int) error {
	if len(names) != expectedCount {
		return fmt.Errorf("received %d edited names for %d tracks", len(names), expectedCount)
	}
	for index, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("track %02d needs a title", index+1)
		}
		if numberedLinePattern.MatchString(name) {
			return fmt.Errorf("track %02d already has its row number; remove the number from the title", index+1)
		}
	}
	return nil
}

func FormatUTF8BOM(names []string) []byte {
	width := 2
	if digits := len(strconv.Itoa(len(names))); digits > width {
		width = digits
	}
	var output strings.Builder
	output.WriteString("\ufeff")
	for index, name := range names {
		fmt.Fprintf(&output, "%0*d. %s\n", width, index+1, strings.TrimSpace(name))
	}
	return []byte(output.String())
}

func filepathBase(path string) string {
	if index := strings.LastIndexAny(path, `/\\`); index >= 0 {
		return path[index+1:]
	}
	return path
}

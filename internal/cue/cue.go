package cue

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Track struct {
	Number       int     `json:"number"`
	Title        string  `json:"title"`
	Performer    string  `json:"performer,omitempty"`
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
}

type Sheet struct {
	Title     string  `json:"title"`
	Performer string  `json:"performer"`
	Tracks    []Track `json:"tracks"`
}

var (
	trackPattern     = regexp.MustCompile(`(?i)^TRACK\s+(\d+)\s+AUDIO\s*$`)
	indexPattern     = regexp.MustCompile(`(?i)^INDEX\s+01\s+(\d+):(\d+):(\d+)\s*$`)
	titlePattern     = regexp.MustCompile(`(?i)^TITLE\s+(?:"(.*)"|(.*))\s*$`)
	performerPattern = regexp.MustCompile(`(?i)^PERFORMER\s+(?:"(.*)"|(.*))\s*$`)
)

func Parse(reader io.Reader) ([]Track, error) {
	sheet, err := ParseSheet(reader)
	if err != nil {
		return nil, err
	}
	return sheet.Tracks, nil
}

func ParseSheet(reader io.Reader) (Sheet, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var sheet Sheet
	var hasIndex []bool
	current := -1
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if matches := trackPattern.FindStringSubmatch(line); matches != nil {
			number, _ := strconv.Atoi(matches[1])
			sheet.Tracks = append(sheet.Tracks, Track{Number: number, EndSeconds: -1})
			hasIndex = append(hasIndex, false)
			current = len(sheet.Tracks) - 1
			continue
		}
		if matches := indexPattern.FindStringSubmatch(line); matches != nil {
			if current < 0 {
				return Sheet{}, fmt.Errorf("line %d: INDEX 01 appears before an AUDIO track", lineNumber)
			}
			minutes, _ := strconv.Atoi(matches[1])
			seconds, _ := strconv.Atoi(matches[2])
			frames, _ := strconv.Atoi(matches[3])
			if seconds > 59 || frames > 74 {
				return Sheet{}, fmt.Errorf("line %d: invalid CUE timestamp", lineNumber)
			}
			sheet.Tracks[current].StartSeconds = float64(minutes*60+seconds) + float64(frames)/75
			hasIndex[current] = true
			continue
		}
		if matches := titlePattern.FindStringSubmatch(line); matches != nil {
			title := matches[1]
			if title == "" {
				title = matches[2]
			}
			title = strings.TrimSpace(strings.ReplaceAll(title, `\"`, `"`))
			if current >= 0 {
				sheet.Tracks[current].Title = title
			} else {
				sheet.Title = title
			}
			continue
		}
		if matches := performerPattern.FindStringSubmatch(line); matches != nil {
			performer := matches[1]
			if performer == "" {
				performer = matches[2]
			}
			performer = strings.TrimSpace(strings.ReplaceAll(performer, `\"`, `"`))
			if current >= 0 {
				sheet.Tracks[current].Performer = performer
			} else {
				sheet.Performer = performer
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Sheet{}, fmt.Errorf("read CUE: %w", err)
	}
	if len(sheet.Tracks) == 0 {
		return Sheet{}, fmt.Errorf("CUE contains no AUDIO tracks")
	}
	for i := range sheet.Tracks {
		if !hasIndex[i] {
			return Sheet{}, fmt.Errorf("track %d has no INDEX 01 timestamp", sheet.Tracks[i].Number)
		}
		if i > 0 && sheet.Tracks[i].StartSeconds <= sheet.Tracks[i-1].StartSeconds {
			return Sheet{}, fmt.Errorf("track %d does not start after track %d", sheet.Tracks[i].Number, sheet.Tracks[i-1].Number)
		}
		if i+1 < len(sheet.Tracks) {
			sheet.Tracks[i].EndSeconds = sheet.Tracks[i+1].StartSeconds
		}
	}
	return sheet, nil
}

func WithDuration(tracks []Track, duration float64) []Track {
	result := append([]Track(nil), tracks...)
	if len(result) > 0 && duration > result[len(result)-1].StartSeconds {
		result[len(result)-1].EndSeconds = duration
	}
	return result
}

func SortedCopy(tracks []Track) []Track {
	result := append([]Track(nil), tracks...)
	sort.SliceStable(result, func(i, j int) bool { return result[i].StartSeconds < result[j].StartSeconds })
	return result
}

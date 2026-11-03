package jobs

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"cuesplitter/internal/cue"
	"cuesplitter/internal/media"
	"cuesplitter/internal/tracklist"
)

type Manifest struct {
	TrackNames      []string `json:"trackNames"`
	NameSource      string   `json:"nameSource"`
	TextEncoding    string   `json:"textEncoding"`
	AlbumTitle      string   `json:"albumTitle"`
	Artist          string   `json:"artist"`
	Format          string   `json:"format"`
	Bitrate         int      `json:"bitrate"`
	Destination     string   `json:"destination"`
	DestinationPath string   `json:"destinationPath"`
	SourceDuration  float64  `json:"sourceDuration"`
}

type InputFiles struct {
	Cue   string
	Audio string
	Cover string
	Text  string
}

type TrackStatus struct {
	Index        int     `json:"index"`
	Name         string  `json:"name"`
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
	Progress     float64 `json:"progress"`
	Status       string  `json:"status"`
}

type Snapshot struct {
	ID          string        `json:"id"`
	State       string        `json:"state"`
	Message     string        `json:"message,omitempty"`
	Tracks      []TrackStatus `json:"tracks"`
	DownloadURL string        `json:"downloadUrl,omitempty"`
	OutputPath  string        `json:"outputPath,omitempty"`
	Error       string        `json:"error,omitempty"`
}

type Manager struct {
	mu          sync.RWMutex
	jobs        map[string]*job
	processor   *media.Processor
	projectRoot string
}

type job struct {
	mu          sync.Mutex
	id          string
	state       string
	message     string
	tracks      []TrackStatus
	downloadURL string
	outputPath  string
	err         string
	tempDir     string
	zipPath     string
	inputs      InputFiles
	manifest    Manifest
	export      media.ExportOptions
	albumName   string
	albumTitle  string
	artist      string
	subscribers map[chan Snapshot]struct{}
}

func NewManager(processor *media.Processor, projectRoot string) *Manager {
	return &Manager{
		jobs:        make(map[string]*job),
		processor:   processor,
		projectRoot: projectRoot,
	}
}

func (m *Manager) Create(tempDir string, inputs InputFiles, manifest Manifest) (Snapshot, error) {
	if manifest.Destination != "browser" && manifest.Destination != "project" && manifest.Destination != "custom" {
		return Snapshot{}, fmt.Errorf("unsupported destination %q", manifest.Destination)
	}
	if manifest.Destination == "custom" && strings.TrimSpace(manifest.DestinationPath) == "" {
		return Snapshot{}, fmt.Errorf("custom output path is required")
	}

	cueFile, err := os.Open(inputs.Cue)
	if err != nil {
		return Snapshot{}, fmt.Errorf("open CUE: %w", err)
	}
	tracks, parseErr := cue.Parse(cueFile)
	cueFile.Close()
	if parseErr != nil {
		return Snapshot{}, parseErr
	}

	duration, probeErr := m.processor.ProbeDuration(context.Background(), inputs.Audio)
	if probeErr != nil {
		duration = manifest.SourceDuration
	}
	tracks = cue.WithDuration(tracks, duration)
	names, err := resolveTrackNames(manifest, inputs, tracks)
	if err != nil {
		return Snapshot{}, err
	}
	albumTitle, artist, err := resolveAlbumMetadata(manifest, inputs, tracks)
	if err != nil {
		return Snapshot{}, err
	}
	resolvedExport, err := m.processor.ResolveExport(context.Background(), inputs.Audio, media.ExportOptions{Format: manifest.Format, Bitrate: manifest.Bitrate})
	if err != nil {
		return Snapshot{}, err
	}

	statuses := make([]TrackStatus, len(tracks))
	for index, track := range tracks {
		statuses[index] = TrackStatus{
			Index:        index,
			Name:         names[index],
			StartSeconds: track.StartSeconds,
			EndSeconds:   track.EndSeconds,
			Status:       "waiting",
		}
	}

	id, err := randomID()
	if err != nil {
		return Snapshot{}, err
	}
	item := &job{
		id:          id,
		state:       "queued",
		message:     "Upload complete. Waiting to split tracks.",
		tracks:      statuses,
		tempDir:     tempDir,
		inputs:      inputs,
		manifest:    manifest,
		export:      resolvedExport,
		albumName:   cleanName(albumTitle, cleanName(strings.TrimSuffix(filepath.Base(inputs.Cue), filepath.Ext(inputs.Cue)), "album")),
		albumTitle:  albumTitle,
		artist:      artist,
		subscribers: make(map[chan Snapshot]struct{}),
	}
	if manifest.Destination == "browser" {
		item.downloadURL = "/api/jobs/" + id + "/download"
	}
	m.mu.Lock()
	m.jobs[id] = item
	m.mu.Unlock()

	go m.process(item)
	return item.snapshot(), nil
}

func (m *Manager) Snapshot(id string) (Snapshot, bool) {
	item, ok := m.get(id)
	if !ok {
		return Snapshot{}, false
	}
	return item.snapshot(), true
}

func (m *Manager) Subscribe(id string) (<-chan Snapshot, func(), bool) {
	item, ok := m.get(id)
	if !ok {
		return nil, nil, false
	}
	updates := make(chan Snapshot, 1)
	item.mu.Lock()
	item.subscribers[updates] = struct{}{}
	updates <- item.snapshotLocked()
	item.mu.Unlock()
	cancel := func() {
		item.mu.Lock()
		if _, exists := item.subscribers[updates]; exists {
			delete(item.subscribers, updates)
			close(updates)
		}
		item.mu.Unlock()
	}
	return updates, cancel, true
}

func (m *Manager) Download(id string) (string, string, bool) {
	item, ok := m.get(id)
	if !ok {
		return "", "", false
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.state != "complete" || item.zipPath == "" {
		return "", "", false
	}
	return item.zipPath, item.albumName + "-tracks.zip", true
}

func (m *Manager) get(id string) (*job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.jobs[id]
	return item, ok
}

func (m *Manager) process(item *job) {
	item.update(func() {
		item.state = "processing"
		item.message = "Splitting tracks."
	})

	outputDir, err := m.outputDirectory(item)
	if err != nil {
		item.fail(err)
		return
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		item.fail(fmt.Errorf("create output directory: %w", err))
		return
	}
	coverPath, err := m.writeCover(item.inputs.Cover, outputDir)
	if err != nil {
		item.fail(err)
		return
	}
	if err := writeTrackList(item.tracks, outputDir); err != nil {
		item.fail(err)
		return
	}

	extension, _ := media.Extension(item.export.Format)
	usedNames := make(map[string]int)
	for index := range item.tracks {
		track := item.track(index)
		filename := uniqueFilename(cleanName(track.Name, fmt.Sprintf("track%02d", index+1)), extension, usedNames)
		output := filepath.Join(outputDir, filename)
		duration := track.EndSeconds - track.StartSeconds
		if track.EndSeconds < 0 {
			duration = 0
		}
		item.setTrack(index, "processing", 0)
		err := m.processor.Split(
			context.Background(),
			item.inputs.Audio,
			output,
			track.StartSeconds,
			duration,
			item.export,
			media.TrackMetadata{
				Title:       track.Name,
				Album:       item.albumTitle,
				Artist:      item.artist,
				AlbumArtist: item.artist,
				TrackNumber: index + 1,
				TrackTotal:  len(item.tracks),
			},
			coverPath,
			func(progress float64) { item.setTrack(index, "processing", progress) },
		)
		if err != nil {
			item.setTrack(index, "error", track.Progress)
			item.fail(err)
			return
		}
		item.setTrack(index, "complete", 1)
	}

	if item.manifest.Destination == "browser" {
		zipPath := filepath.Join(item.tempDir, item.albumName+"-tracks.zip")
		if err := zipDirectory(outputDir, zipPath); err != nil {
			item.fail(err)
			return
		}
		item.mu.Lock()
		item.zipPath = zipPath
		item.mu.Unlock()
	}

	item.update(func() {
		item.state = "complete"
		item.message = "All tracks are ready."
		if item.manifest.Destination != "browser" {
			item.outputPath = outputDir
		}
	})
}

func (m *Manager) outputDirectory(item *job) (string, error) {
	if item.manifest.Destination == "browser" {
		return filepath.Join(item.tempDir, "result"), nil
	}
	base := filepath.Join(m.projectRoot, "output")
	if item.manifest.Destination == "custom" {
		base = strings.TrimSpace(item.manifest.DestinationPath)
		if strings.HasPrefix(base, "~"+string(os.PathSeparator)) {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home directory: %w", err)
			}
			base = filepath.Join(home, strings.TrimPrefix(base, "~"+string(os.PathSeparator)))
		}
		if !filepath.IsAbs(base) {
			base = filepath.Join(m.projectRoot, base)
		}
	}
	base = filepath.Clean(base)
	candidate := filepath.Join(base, item.albumName)
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		candidate += "-" + item.id[:6]
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect output path: %w", err)
	}
	return candidate, nil
}

func (item *job) snapshot() Snapshot {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.snapshotLocked()
}

func (item *job) snapshotLocked() Snapshot {
	return Snapshot{
		ID:          item.id,
		State:       item.state,
		Message:     item.message,
		Tracks:      append([]TrackStatus(nil), item.tracks...),
		DownloadURL: item.downloadURL,
		OutputPath:  item.outputPath,
		Error:       item.err,
	}
}

func (item *job) update(change func()) {
	item.mu.Lock()
	change()
	item.publishLocked()
	item.mu.Unlock()
}

func (item *job) track(index int) TrackStatus {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.tracks[index]
}

func (item *job) setTrack(index int, status string, progress float64) {
	item.update(func() {
		item.tracks[index].Status = status
		item.tracks[index].Progress = progress
		item.message = fmt.Sprintf("Processing track %d of %d.", index+1, len(item.tracks))
	})
}

func (item *job) fail(err error) {
	item.update(func() {
		item.state = "error"
		item.err = err.Error()
		item.message = "Processing stopped."
	})
}

func (item *job) publishLocked() {
	snapshot := item.snapshotLocked()
	for subscriber := range item.subscribers {
		select {
		case subscriber <- snapshot:
		default:
			select {
			case <-subscriber:
			default:
			}
			subscriber <- snapshot
		}
	}
}

func resolveTrackNames(manifest Manifest, inputs InputFiles, tracks []cue.Track) ([]string, error) {
	count := len(tracks)
	selectedEncoding, err := tracklist.NormalizeEncoding(manifest.TextEncoding)
	if err != nil {
		return nil, err
	}
	source := resolveNameSource(manifest.NameSource, inputs.Text != "", tracks)

	var sourceNames []string
	switch source {
	case "txt":
		if inputs.Text == "" {
			return nil, fmt.Errorf("TXT song list is selected, but no TXT file was supplied")
		}
		decoded, err := tracklist.DecodeFile(inputs.Text, selectedEncoding)
		if err != nil {
			return nil, fmt.Errorf("read track-name TXT: %w", err)
		}
		sourceNames, err = tracklist.ParseNumbered(decoded, count)
		if err != nil {
			return nil, err
		}
	case "cue":
		decoded, err := tracklist.DecodeFile(inputs.Cue, selectedEncoding)
		if err != nil {
			return nil, fmt.Errorf("read CUE titles: %w", err)
		}
		decodedSheet, err := cue.ParseSheet(strings.NewReader(decoded))
		if err != nil {
			return nil, err
		}
		decodedTracks := decodedSheet.Tracks
		if len(decodedTracks) != count {
			return nil, fmt.Errorf("decoded CUE has %d tracks; expected %d", len(decodedTracks), count)
		}
		sourceNames = make([]string, count)
		for index, track := range decodedTracks {
			sourceNames[index] = strings.TrimSpace(track.Title)
		}
		if err := tracklist.ValidateTitles(sourceNames, count); err != nil {
			return nil, fmt.Errorf("CUE TITLE fields: %w", err)
		}
	case "generated":
		sourceNames = make([]string, count)
		for index := range sourceNames {
			sourceNames[index] = fmt.Sprintf("track%02d", index+1)
		}
	default:
		return nil, fmt.Errorf("unsupported track-name source %q", source)
	}

	if len(manifest.TrackNames) > 0 {
		if err := tracklist.ValidateTitles(manifest.TrackNames, count); err != nil {
			return nil, err
		}
		sourceNames = append([]string(nil), manifest.TrackNames...)
	}
	for index := range sourceNames {
		sourceNames[index] = strings.TrimSpace(sourceNames[index])
	}
	return sourceNames, nil
}

func resolveNameSource(requested string, hasText bool, tracks []cue.Track) string {
	source := strings.ToLower(strings.TrimSpace(requested))
	if source != "" {
		return source
	}
	if hasText {
		return "txt"
	}
	for _, track := range tracks {
		if strings.TrimSpace(track.Title) != "" {
			return "cue"
		}
	}
	return "generated"
}

func resolveAlbumMetadata(manifest Manifest, inputs InputFiles, tracks []cue.Track) (string, string, error) {
	albumTitle, err := validateMetadataValue("album", manifest.AlbumTitle)
	if err != nil {
		return "", "", err
	}
	artist, err := validateMetadataValue("artist", manifest.Artist)
	if err != nil {
		return "", "", err
	}
	if resolveNameSource(manifest.NameSource, inputs.Text != "", tracks) != "cue" || (albumTitle != "" && artist != "") {
		return albumTitle, artist, nil
	}
	selectedEncoding, err := tracklist.NormalizeEncoding(manifest.TextEncoding)
	if err != nil {
		return "", "", err
	}
	decoded, err := tracklist.DecodeFile(inputs.Cue, selectedEncoding)
	if err != nil {
		return "", "", fmt.Errorf("read CUE metadata: %w", err)
	}
	sheet, err := cue.ParseSheet(strings.NewReader(decoded))
	if err != nil {
		return "", "", err
	}
	if albumTitle == "" {
		albumTitle = strings.TrimSpace(sheet.Title)
	}
	if artist == "" {
		artist = strings.TrimSpace(sheet.Performer)
	}
	return albumTitle, artist, nil
}

func validateMetadataValue(label, value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("%s metadata must be one line", label)
	}
	if utf8.RuneCountInString(value) > 500 {
		return "", fmt.Errorf("%s metadata is too long", label)
	}
	return value, nil
}

var unsafeFilename = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)

func cleanName(value, fallback string) string {
	value = strings.TrimSpace(filepath.Base(value))
	value = unsafeFilename.ReplaceAllString(value, "-")
	value = strings.Trim(value, ". ")
	if value == "" {
		return fallback
	}
	return value
}

func uniqueFilename(name, extension string, used map[string]int) string {
	key := strings.ToLower(name)
	used[key]++
	if used[key] == 1 {
		return name + extension
	}
	return fmt.Sprintf("%s-%d%s", name, used[key], extension)
}

func (m *Manager) writeCover(source, outputDir string) (string, error) {
	destination := filepath.Join(outputDir, "cover.jpg")
	if source != "" {
		if err := m.processor.NormalizeCover(context.Background(), source, destination); err != nil {
			return "", err
		}
		return destination, nil
	}
	file, err := os.Create(destination)
	if err != nil {
		return "", fmt.Errorf("create blank cover: %w", err)
	}
	defer file.Close()
	blank := image.NewRGBA(image.Rect(0, 0, 1200, 1200))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for y := 0; y < 1200; y++ {
		for x := 0; x < 1200; x++ {
			blank.SetRGBA(x, y, white)
		}
	}
	if err := jpeg.Encode(file, blank, &jpeg.Options{Quality: 92}); err != nil {
		return "", fmt.Errorf("encode blank cover: %w", err)
	}
	return destination, nil
}

func writeTrackList(tracks []TrackStatus, outputDir string) error {
	names := make([]string, len(tracks))
	for index, track := range tracks {
		names[index] = track.Name
	}
	if err := os.WriteFile(filepath.Join(outputDir, "tracklist.txt"), tracklist.FormatUTF8BOM(names), 0o644); err != nil {
		return fmt.Errorf("write normalized track list: %w", err)
	}
	return nil
}

func zipDirectory(sourceDir, destination string) error {
	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create ZIP: %w", err)
	}
	archive := zip.NewWriter(file)
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		entry, err := archive.Create(filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(entry, input)
		input.Close()
		return copyErr
	})
	closeErr := archive.Close()
	fileErr := file.Close()
	if err != nil {
		return fmt.Errorf("build ZIP: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("finish ZIP: %w", closeErr)
	}
	if fileErr != nil {
		return fmt.Errorf("close ZIP: %w", fileErr)
	}
	return nil
}

func randomID() (string, error) {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("create job ID: %w", err)
	}
	return hex.EncodeToString(data), nil
}

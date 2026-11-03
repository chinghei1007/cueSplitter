package media

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Processor struct {
	FFmpeg  string
	FFprobe string
}

type ExportOptions struct {
	Format  string
	Bitrate int
}

type AudioInfo struct {
	Format  string
	Bitrate int
}

type TrackMetadata struct {
	Title       string
	Album       string
	Artist      string
	AlbumArtist string
	TrackNumber int
	TrackTotal  int
}

func NewProcessor(ffmpeg, ffprobe string) *Processor {
	return &Processor{FFmpeg: ffmpeg, FFprobe: ffprobe}
}

func (p *Processor) Available() (bool, bool) {
	_, ffmpegErr := exec.LookPath(p.FFmpeg)
	_, ffprobeErr := exec.LookPath(p.FFprobe)
	return ffmpegErr == nil, ffprobeErr == nil
}

func (p *Processor) ProbeDuration(ctx context.Context, input string) (float64, error) {
	output, err := exec.CommandContext(ctx, p.FFprobe,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		input,
	).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration: %w", err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("ffprobe returned an invalid duration")
	}
	return duration, nil
}

func (p *Processor) NormalizeCover(ctx context.Context, input, output string) error {
	command := exec.CommandContext(ctx, p.FFmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", input,
		"-frames:v", "1",
		"-vf", "scale=1600:1600:force_original_aspect_ratio=decrease",
		"-q:v", "2",
		"-update", "1",
		output,
	)
	var stderr strings.Builder
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("normalize cover: %s", message)
	}
	return nil
}

func (p *Processor) ProbeAudioInfo(ctx context.Context, input string) (AudioInfo, error) {
	output, err := exec.CommandContext(ctx, p.FFprobe,
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name,bit_rate:format=bit_rate",
		"-of", "json",
		input,
	).Output()
	if err != nil {
		return AudioInfo{}, fmt.Errorf("ffprobe audio info: %w", err)
	}
	var result struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			Bitrate   string `json:"bit_rate"`
		} `json:"streams"`
		Format struct {
			Bitrate string `json:"bit_rate"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return AudioInfo{}, fmt.Errorf("read ffprobe audio info: %w", err)
	}
	bitrateValue := result.Format.Bitrate
	if len(result.Streams) > 0 && result.Streams[0].Bitrate != "" {
		bitrateValue = result.Streams[0].Bitrate
	}
	bitsPerSecond, _ := strconv.Atoi(bitrateValue)
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(input)), ".")
	return AudioInfo{Format: format, Bitrate: (bitsPerSecond + 500) / 1000}, nil
}

func (p *Processor) ResolveExport(ctx context.Context, input string, requested ExportOptions) (ExportOptions, error) {
	requested.Format = strings.ToLower(strings.TrimSpace(requested.Format))
	if requested.Bitrate != 0 && requested.Bitrate != 128 && requested.Bitrate != 192 && requested.Bitrate != 256 && requested.Bitrate != 320 {
		return ExportOptions{}, fmt.Errorf("unsupported export bitrate %d", requested.Bitrate)
	}
	if requested.Format == "original" {
		requested.Format = strings.TrimPrefix(strings.ToLower(filepath.Ext(input)), ".")
		if requested.Format == "aif" {
			requested.Format = "aiff"
		}
		if requested.Format == "ape" {
			return ExportOptions{}, fmt.Errorf("Original export is unavailable for APE because FFmpeg has no compatible APE encoder; choose FLAC or WAV")
		}
	}
	if _, err := Extension(requested.Format); err != nil {
		return ExportOptions{}, err
	}
	if isLossless(requested.Format) {
		requested.Bitrate = 0
		return requested, nil
	}
	if requested.Bitrate == 0 {
		info, err := p.ProbeAudioInfo(ctx, input)
		if err != nil {
			return ExportOptions{}, err
		}
		if info.Bitrate <= 0 {
			return ExportOptions{}, fmt.Errorf("source bitrate is unavailable; choose 128, 192, 256, or 320 kbps")
		}
		requested.Bitrate = info.Bitrate
	}
	return requested, nil
}

func (p *Processor) Split(
	ctx context.Context,
	input string,
	output string,
	start float64,
	duration float64,
	options ExportOptions,
	metadata TrackMetadata,
	cover string,
	onProgress func(float64),
) error {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", input,
	}
	embedCover := cover != "" && supportsEmbeddedCover(options.Format)
	if embedCover {
		args = append(args, "-i", cover)
	}
	args = append(args, "-ss", formatSeconds(start))
	if duration > 0 {
		args = append(args, "-t", formatSeconds(duration))
	}
	args = append(args, "-map", "0:a:0")
	if embedCover {
		args = append(args, "-map", "1:v:0")
	}
	codecArgs, err := codecArguments(options)
	if err != nil {
		return err
	}
	args = append(args, codecArgs...)
	args = append(args, metadataArguments(metadata)...)
	if embedCover {
		args = append(args,
			"-c:v", "copy",
			"-frames:v", "1",
			"-metadata:s:v", "title=Album cover",
			"-metadata:s:v", "comment=Cover (front)",
			"-disposition:v:0", "attached_pic",
		)
		if strings.EqualFold(options.Format, "mp3") {
			args = append(args, "-id3v2_version", "3")
		}
	}
	args = append(args, "-progress", "pipe:1", "-nostats", output)

	cmd := exec.CommandContext(ctx, p.FFmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("read ffmpeg progress: %w", err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	parseProgress(stdout, duration, onProgress)
	if err := cmd.Wait(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("ffmpeg failed for %s: %s", filepath.Base(output), message)
	}
	if onProgress != nil {
		onProgress(1)
	}
	return nil
}

func metadataArguments(metadata TrackMetadata) []string {
	args := []string{"-map_metadata", "-1"}
	values := [][2]string{
		{"title", strings.TrimSpace(metadata.Title)},
		{"album", strings.TrimSpace(metadata.Album)},
		{"artist", strings.TrimSpace(metadata.Artist)},
		{"album_artist", strings.TrimSpace(metadata.AlbumArtist)},
	}
	for _, value := range values {
		if value[1] != "" {
			args = append(args, "-metadata", value[0]+"="+value[1])
		}
	}
	if metadata.TrackNumber > 0 {
		track := strconv.Itoa(metadata.TrackNumber)
		if metadata.TrackTotal > 0 {
			track += "/" + strconv.Itoa(metadata.TrackTotal)
		}
		args = append(args, "-metadata", "track="+track)
	}
	return args
}

func codecArguments(options ExportOptions) ([]string, error) {
	switch strings.ToLower(options.Format) {
	case "mp3":
		return []string{"-c:a", "libmp3lame", "-b:a", bitrate(options.Bitrate)}, nil
	case "flac":
		return []string{"-c:a", "flac"}, nil
	case "wav":
		return []string{"-c:a", "pcm_s16le"}, nil
	case "m4a", "aac":
		return []string{"-c:a", "aac", "-b:a", bitrate(options.Bitrate)}, nil
	case "aiff":
		return []string{"-c:a", "pcm_s16be"}, nil
	case "ogg":
		return []string{"-c:a", "libvorbis", "-b:a", bitrate(options.Bitrate)}, nil
	case "opus":
		return []string{"-c:a", "libopus", "-b:a", bitrate(options.Bitrate)}, nil
	case "wma":
		return []string{"-c:a", "wmav2", "-b:a", bitrate(options.Bitrate)}, nil
	default:
		return nil, fmt.Errorf("unsupported export format %q", options.Format)
	}
}

func supportsEmbeddedCover(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "flac", "mp3", "m4a":
		return true
	default:
		return false
	}
}

func Extension(format string) (string, error) {
	switch strings.ToLower(format) {
	case "mp3", "flac", "wav", "m4a", "aiff", "ogg", "opus", "wma":
		return "." + strings.ToLower(format), nil
	case "aac":
		return ".aac", nil
	default:
		return "", fmt.Errorf("unsupported export format %q", format)
	}
}

func bitrate(value int) string {
	if value <= 0 {
		value = 320
	}
	return strconv.Itoa(value) + "k"
}

func isLossless(format string) bool {
	switch strings.ToLower(format) {
	case "flac", "wav", "aiff":
		return true
	default:
		return false
	}
}

func formatSeconds(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func parseProgress(reader io.Reader, duration float64, onProgress func(float64)) {
	if onProgress == nil {
		io.Copy(io.Discard, reader)
		return
	}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "progress=end" {
			onProgress(1)
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || duration <= 0 || (key != "out_time_us" && key != "out_time_ms") {
			continue
		}
		microseconds, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		progress := (microseconds / 1_000_000) / duration
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		onProgress(progress)
	}
}

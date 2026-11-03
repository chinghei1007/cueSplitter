package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cuesplitter/internal/jobs"
	"cuesplitter/internal/media"
	"cuesplitter/web"
)

const maxUploadSize int64 = 20 << 30

type api struct {
	manager   *jobs.Manager
	processor *media.Processor
}

type savedFile struct {
	name string
	path string
}

var audioExtensions = map[string]bool{
	".aac": true, ".aiff": true, ".ape": true, ".flac": true, ".m4a": true,
	".mp3": true, ".ogg": true, ".opus": true, ".wav": true, ".wma": true,
}

var imageExtensions = map[string]bool{
	".jpeg": true, ".jpg": true, ".png": true, ".webp": true,
}

func New(manager *jobs.Manager, processor *media.Processor) (http.Handler, error) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded frontend: %w", err)
	}
	service := &api{manager: manager, processor: processor}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", service.health)
	mux.HandleFunc("POST /api/jobs", service.createJob)
	mux.HandleFunc("GET /api/jobs/{id}", service.jobStatus)
	mux.HandleFunc("GET /api/jobs/{id}/events", service.jobEvents)
	mux.HandleFunc("GET /api/jobs/{id}/download", service.download)
	mux.Handle("/", spaHandler(dist))
	return securityHeaders(mux), nil
}

func (a *api) health(w http.ResponseWriter, _ *http.Request) {
	ffmpeg, ffprobe := a.processor.Available()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"ffmpeg":  ffmpeg,
		"ffprobe": ffprobe,
	})
}

func (a *api) createJob(w http.ResponseWriter, r *http.Request) {
	ffmpeg, ffprobe := a.processor.Available()
	if !ffmpeg || !ffprobe {
		writeError(w, http.StatusServiceUnavailable, "FFmpeg and FFprobe must be installed and available on PATH.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Could not read the uploaded files: "+err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()
	var manifest jobs.Manifest
	if err := json.Unmarshal([]byte(r.FormValue("manifest")), &manifest); err != nil {
		writeError(w, http.StatusBadRequest, "The export settings are invalid.")
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "Drop a folder or select files before starting.")
		return
	}

	tempDir, err := os.MkdirTemp("", "cue-splitter-job-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create a temporary job folder.")
		return
	}
	keepTemp := false
	defer func() {
		if !keepTemp {
			os.RemoveAll(tempDir)
		}
	}()

	saved := make([]savedFile, 0, len(files))
	for _, header := range files {
		input, err := header.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "Could not read "+header.Filename+".")
			return
		}
		name := filepath.Base(header.Filename)
		outputPath := filepath.Join(tempDir, safeUploadName(name))
		output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			input.Close()
			writeError(w, http.StatusInternalServerError, "Could not save "+name+".")
			return
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		input.Close()
		if copyErr != nil || closeErr != nil {
			writeError(w, http.StatusInternalServerError, "Could not finish saving "+name+".")
			return
		}
		saved = append(saved, savedFile{name: name, path: outputPath})
	}

	inputs, err := classify(saved)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	snapshot, err := a.manager.Create(tempDir, inputs, manifest)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	keepTemp = true
	writeJSON(w, http.StatusAccepted, snapshot)
}

func (a *api) jobStatus(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := a.manager.Snapshot(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "Job not found.")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (a *api) jobEvents(w http.ResponseWriter, r *http.Request) {
	updates, cancel, ok := a.manager.Subscribe(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "Job not found.")
		return
	}
	defer cancel()
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming is not supported by this server.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case snapshot, open := <-updates:
			if !open {
				return
			}
			data, _ := json.Marshal(snapshot)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if snapshot.State == "complete" || snapshot.State == "error" {
				return
			}
		case <-heartbeat.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func (a *api) download(w http.ResponseWriter, r *http.Request) {
	filePath, filename, ok := a.manager.Download(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "The ZIP is not ready or this job writes to a server folder.")
		return
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Type", "application/zip")
	http.ServeFile(w, r, filePath)
}

func classify(files []savedFile) (jobs.InputFiles, error) {
	var cues, audio, covers, texts []savedFile
	for _, file := range files {
		extension := strings.ToLower(filepath.Ext(file.name))
		switch {
		case extension == ".cue":
			cues = append(cues, file)
		case audioExtensions[extension]:
			audio = append(audio, file)
		case imageExtensions[extension]:
			covers = append(covers, file)
		case extension == ".txt":
			texts = append(texts, file)
		}
	}
	if len(cues) != 1 {
		return jobs.InputFiles{}, fmt.Errorf("expected exactly one CUE file; found %d", len(cues))
	}
	if len(audio) != 1 {
		return jobs.InputFiles{}, fmt.Errorf("expected exactly one supported audio file; found %d", len(audio))
	}
	result := jobs.InputFiles{Cue: cues[0].path, Audio: audio[0].path}
	if len(covers) > 0 {
		result.Cover = preferredOptional(covers, []string{"cover", "folder", "front"}).path
	}
	if len(texts) > 0 {
		result.Text = preferredOptional(texts, []string{"music", "tracks", "titles"}).path
	}
	return result, nil
}

func preferredOptional(files []savedFile, preferred []string) savedFile {
	for _, word := range preferred {
		for _, file := range files {
			base := strings.ToLower(strings.TrimSuffix(file.name, filepath.Ext(file.name)))
			if strings.Contains(base, word) {
				return file
			}
		}
	}
	return files[0]
}

func safeUploadName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.NewReplacer("/", "-", "\\", "-", "\x00", "-").Replace(name)
	if name == "" || name == "." {
		return "upload"
	}
	return name
}

func spaHandler(dist fs.FS) http.Handler {
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested == "." || requested == "" || requested == "index.html" {
			serveIndex(w, dist)
			return
		}
		if info, err := fs.Stat(dist, requested); err == nil && !info.IsDir() {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/" + requested
			files.ServeHTTP(w, clone)
			return
		}
		serveIndex(w, dist)
	})
}

func serveIndex(w http.ResponseWriter, dist fs.FS) {
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "embedded frontend is unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(index)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; media-src 'self' blob:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

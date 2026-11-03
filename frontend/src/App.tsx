import { useEffect, useMemo, useRef, useState } from "react";
import { DetectedInputs } from "./components/DetectedInputs";
import { DropArea } from "./components/DropArea";
import { ExportSettings } from "./components/ExportSettings";
import { TrackList } from "./components/TrackList";
import { createPreview } from "./lib/files";
import { titleOnlyError } from "./lib/tracklist";
import type { ExportSettings as Settings, Health, InputPreview, JobSnapshot, NameSource, TextEncoding, TrackDraft } from "./types";

const initialSettings: Settings = {
  format: "mp3",
  bitrate: 320,
  destination: "browser",
  destinationPath: "",
};

type Phase = "ready" | "uploading" | "processing" | "complete" | "error";

export default function App() {
  const [preview, setPreview] = useState<InputPreview>();
  const [tracks, setTracks] = useState<TrackDraft[]>([]);
  const [settings, setSettings] = useState(initialSettings);
  const [phase, setPhase] = useState<Phase>("ready");
  const [uploadProgress, setUploadProgress] = useState(0);
  const [message, setMessage] = useState("");
  const [outputPath, setOutputPath] = useState("");
  const [health, setHealth] = useState<Health>();
  const [albumTitle, setAlbumTitle] = useState("");
  const [artist, setArtist] = useState("");
  const [readingFiles, setReadingFiles] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);
  const downloadStartedRef = useRef(false);
  const metadataDirtyRef = useRef(false);

  useEffect(() => {
    fetch("/api/health")
      .then((response) => response.json())
      .then((value: Health) => setHealth(value))
      .catch(() => setHealth({ status: "error", ffmpeg: false, ffprobe: false }));
    return () => eventSourceRef.current?.close();
  }, []);

  const busy = phase === "uploading" || phase === "processing";
  const errors = useMemo(() => {
    const items = [...(preview?.errors ?? [])];
    tracks.forEach((track, index) => {
      const error = titleOnlyError(track.name, index);
      if (error) items.push(error);
    });
    if (settings.destination === "custom" && !settings.destinationPath.trim()) items.push("Enter the other server output path.");
    if (health && (!health.ffmpeg || !health.ffprobe)) items.push("FFmpeg and FFprobe must be installed and available on PATH.");
    return items;
  }, [health, preview, settings.destination, settings.destinationPath, tracks]);

  async function acceptFiles(files: File[]) {
    eventSourceRef.current?.close();
    setReadingFiles(true);
    setPhase("ready");
    setUploadProgress(0);
    setMessage("");
    setOutputPath("");
    downloadStartedRef.current = false;
    const nextPreview = await createPreview(files);
    setPreview(nextPreview);
    setTracks(nextPreview.tracks);
    setAlbumTitle(nextPreview.albumTitle);
    setArtist(nextPreview.artist);
    metadataDirtyRef.current = false;
    setReadingFiles(false);
  }

  async function rebuildPreview(nameSource: NameSource, textEncoding: TextEncoding) {
    if (!preview) return;
    setReadingFiles(true);
    setPhase("ready");
    setMessage("");
    setOutputPath("");
    const nextPreview = await createPreview(preview.files.all, { nameSource, textEncoding });
    setPreview(nextPreview);
    setTracks(nextPreview.tracks);
    if (!metadataDirtyRef.current) {
      setAlbumTitle(nextPreview.albumTitle);
      setArtist(nextPreview.artist);
    }
    setReadingFiles(false);
  }

  function renameTrack(index: number, name: string) {
    setTracks((current) => current.map((track) => track.index === index ? { ...track, name } : track));
  }

  function applySnapshot(snapshot: JobSnapshot) {
    setMessage(snapshot.error || snapshot.message || "");
    setOutputPath(snapshot.outputPath || "");
    setTracks((current) => current.map((track) => {
      const update = snapshot.tracks.find((candidate) => candidate.index === track.index);
      return update ? { ...track, progress: update.progress, status: update.status } : track;
    }));
    if (snapshot.state === "error") {
      setPhase("error");
      eventSourceRef.current?.close();
    } else if (snapshot.state === "complete") {
      setPhase("complete");
      eventSourceRef.current?.close();
      if (settings.destination === "browser" && snapshot.downloadUrl && !downloadStartedRef.current) {
        downloadStartedRef.current = true;
        const link = document.createElement("a");
        link.href = snapshot.downloadUrl;
        link.download = "";
        document.body.appendChild(link);
        link.click();
        link.remove();
      }
    } else {
      setPhase("processing");
    }
  }

  function watchJob(snapshot: JobSnapshot) {
    applySnapshot(snapshot);
    const events = new EventSource(`/api/jobs/${snapshot.id}/events`);
    eventSourceRef.current = events;
    events.onmessage = (event) => applySnapshot(JSON.parse(event.data) as JobSnapshot);
    events.onerror = () => {
      if (phase !== "complete") setMessage("The progress stream disconnected. The job may still be running.");
      events.close();
    };
  }

  function start() {
    if (!preview || errors.length > 0 || busy) return;
    setPhase("uploading");
    setUploadProgress(0);
    setMessage("Uploading the album files.");
    setOutputPath("");
    downloadStartedRef.current = false;

    const payload = new FormData();
    for (const file of preview.files.all) payload.append("files", file, file.name);
    payload.append("manifest", JSON.stringify({
      trackNames: tracks.map((track) => track.name.trim()),
      nameSource: preview.nameSource,
      textEncoding: preview.textEncoding,
      albumTitle: albumTitle.trim(),
      artist: artist.trim(),
      format: settings.format,
      bitrate: settings.bitrate === "original" ? 0 : settings.bitrate,
      destination: settings.destination,
      destinationPath: settings.destinationPath,
      sourceDuration: preview.duration,
    }));

    const request = new XMLHttpRequest();
    request.open("POST", "/api/jobs");
    request.upload.onprogress = (event) => {
      if (event.lengthComputable) setUploadProgress(event.loaded / event.total);
    };
    request.onerror = () => {
      setPhase("error");
      setMessage("The upload could not reach the local Go server.");
    };
    request.onload = () => {
      let body: JobSnapshot | { error?: string };
      try {
        body = JSON.parse(request.responseText) as JobSnapshot | { error?: string };
      } catch {
        body = { error: "The server returned an unreadable response." };
      }
      if (request.status < 200 || request.status >= 300 || !("id" in body)) {
        setPhase("error");
        setMessage(body.error || `The server rejected the job (${request.status}).`);
        return;
      }
      setUploadProgress(1);
      watchJob(body);
    };
    request.send(payload);
  }

  const sourceExtension = preview?.files.audio?.name.toLowerCase().split(".").pop() ?? "";
  const lossless = settings.format === "flac" || settings.format === "wav" || (settings.format === "original" && ["flac", "wav", "aiff", "aif", "ape"].includes(sourceExtension));
  const visibleErrors = errors.filter((error) => !preview?.nameErrors.includes(error));
  const summary = preview
    ? `Ready to export ${tracks.length} tracks as ${settings.format === "original" ? "Original format" : settings.format.toUpperCase()}${lossless ? " · Original / lossless" : settings.bitrate === "original" ? " · Original bitrate" : ` · ${settings.bitrate} kbps`}`
    : "Drop the album files to begin.";

  return (
    <main className="page">
      <header className="page-header">
        <h1>Cue Splitter</h1>
        <p>Split one album image into editable tracks with the local Go server.</p>
      </header>

      <DropArea busy={busy || readingFiles} onFiles={acceptFiles} />
      {readingFiles && <p className="status-message">Reading the folder and preparing the preview.</p>}

      {preview && !readingFiles && <>
        <DetectedInputs
          preview={preview}
          uploadProgress={uploadProgress}
          phase={phase}
          busy={busy || readingFiles}
          albumTitle={albumTitle}
          artist={artist}
          onNameSource={(nameSource) => rebuildPreview(nameSource, preview.textEncoding)}
          onTextEncoding={(textEncoding) => rebuildPreview(preview.nameSource, textEncoding)}
          onAlbumTitle={(value) => { metadataDirtyRef.current = true; setAlbumTitle(value); }}
          onArtist={(value) => { metadataDirtyRef.current = true; setArtist(value); }}
        />
        {tracks.length > 0 && <TrackList tracks={tracks} busy={busy} onRename={renameTrack} />}
        <ExportSettings value={settings} busy={busy} sourceAudioName={preview.files.audio?.name} onChange={setSettings} />

        {visibleErrors.length > 0 && (
          <div className="message-list" role="alert">
            {visibleErrors.map((error) => <p key={error}>{error}</p>)}
          </div>
        )}

        {(message || outputPath) && (
          <div className={`result-message${phase === "error" ? " is-error" : ""}`} role="status">
            {message && <p>{message}</p>}
            {outputPath && <p>Output: <code>{outputPath}</code></p>}
          </div>
        )}

        <div className="start-row">
          <p>{summary}</p>
          <button type="button" className="start-button" disabled={busy || errors.length > 0 || tracks.length === 0} onClick={start}>
            {phase === "uploading" ? "Uploading" : phase === "processing" ? "Processing" : phase === "complete" ? "Start again" : "Start"}
          </button>
        </div>
      </>}
    </main>
  );
}

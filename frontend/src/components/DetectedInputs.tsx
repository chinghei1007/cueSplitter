import type { InputPreview, NameSource, TextEncoding } from "../types";
import { formatDuration } from "../lib/files";
import { ProgressBar } from "./ProgressBar";

type Props = {
  preview: InputPreview;
  uploadProgress: number;
  phase: "ready" | "uploading" | "processing" | "complete" | "error";
  busy: boolean;
  albumTitle: string;
  artist: string;
  onNameSource: (value: NameSource) => void;
  onTextEncoding: (value: TextEncoding) => void;
  onAlbumTitle: (value: string) => void;
  onArtist: (value: string) => void;
};

export function DetectedInputs({ preview, uploadProgress, phase, busy, albumTitle, artist, onNameSource, onTextEncoding, onAlbumTitle, onArtist }: Props) {
  const { files } = preview;
  const audioProgress = phase === "ready" ? 0 : phase === "uploading" ? uploadProgress : 1;
  const audioLabel = phase === "ready" ? "ready" : phase === "uploading" ? `${Math.round(uploadProgress * 100)}% uploaded` : "uploaded";
  return (
    <section className="section-block" aria-labelledby="detected-title">
      <div className="section-heading">
        <h2 id="detected-title">Detected input</h2>
        <span>Original length {preview.duration > 0 ? formatDuration(preview.duration) : "—"}</span>
      </div>
      <div className="rows">
        <div className="input-row"><span>Cue sheet</span><span>{files.cue?.name ?? "Missing"}</span><span>{files.cue ? "ready" : "required"}</span></div>
        <div className="input-row"><span>Source audio</span><span>{files.audio ? `${files.audio.name} · ${preview.duration > 0 ? formatDuration(preview.duration) : "duration pending"}` : "Missing"}</span><ProgressBar value={audioProgress} label={audioLabel} /></div>
        <div className="input-row"><span>Cover</span><span className="cover-value"><i className="cover-swatch" />{files.cover?.name ?? "No image · blank cover will be used"}</span><span>{files.cover ? "ready" : "fallback"}</span></div>
        <div className="input-row"><span>Track names</span><span>{files.text?.name ?? "No TXT · CUE TITLE or trackNN fallback"}</span><span>{files.text ? "ready" : "fallback"}</span></div>
      </div>
      <div className="name-controls">
        <label>
          <span>Name source</span>
          <select aria-label="Name source" disabled={busy} value={preview.nameSource} onChange={(event) => onNameSource(event.target.value as NameSource)}>
            <option value="txt" disabled={!files.text}>TXT song list</option>
            <option value="cue">CUE TITLE fields</option>
            <option value="generated">Generated trackNN</option>
          </select>
        </label>
        <label>
          <span>Selected source encoding</span>
          <select aria-label="Text encoding" disabled={busy} value={preview.textEncoding} onChange={(event) => onTextEncoding(event.target.value as TextEncoding)}>
            <option value="auto">Auto</option>
            <option value="utf-8">UTF-8</option>
            <option value="big5">Big5</option>
            <option value="gb18030">GB18030</option>
            <option value="shift_jis">Shift_JIS</option>
          </select>
        </label>
      </div>
      <div className="metadata-controls">
        <label>
          <span>Album</span>
          <input aria-label="Album" disabled={busy} value={albumTitle} onChange={(event) => onAlbumTitle(event.target.value)} />
        </label>
        <label>
          <span>Artist</span>
          <input aria-label="Artist" disabled={busy} value={artist} onChange={(event) => onArtist(event.target.value)} />
        </label>
      </div>
      <p className="quiet-note">CUE album TITLE and PERFORMER fill these fields when available. Both remain editable and are written into the exported audio tags.</p>
      {preview.nameErrors.length > 0 && (
        <div className="inline-errors" role="alert">
          {preview.nameErrors.map((error) => <p key={error}>{error}</p>)}
        </div>
      )}
      {files.ignored.length > 0 && <p className="quiet-note">Ignored {files.ignored.length} unrelated file{files.ignored.length === 1 ? "" : "s"}.</p>}
    </section>
  );
}

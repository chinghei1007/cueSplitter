import type { ExportSettings as Settings } from "../types";

type Props = {
  value: Settings;
  busy: boolean;
  sourceAudioName?: string;
  onChange: (value: Settings) => void;
};

const losslessExtensions = new Set(["flac", "wav", "aiff", "aif", "ape"]);

export function ExportSettings({ value, busy, sourceAudioName, onChange }: Props) {
  const sourceExtension = sourceAudioName?.toLowerCase().split(".").pop() ?? "";
  const lossless = value.format === "flac" || value.format === "wav" || (value.format === "original" && losslessExtensions.has(sourceExtension));

  function changeFormat(format: Settings["format"]) {
    const nextLossless = format === "flac" || format === "wav" || (format === "original" && losslessExtensions.has(sourceExtension));
    onChange({ ...value, format, bitrate: nextLossless ? "original" : value.bitrate });
  }

  return (
    <section className="section-block" aria-labelledby="export-title">
      <div className="section-heading">
        <h2 id="export-title">Export</h2>
        <span>settings stay with this preview</span>
      </div>
      <div className="setting-grid">
        <label>
          <span>Format</span>
          <select aria-label="Format" disabled={busy} value={value.format} onChange={(event) => changeFormat(event.target.value as Settings["format"])}>
            <option value="original">Original</option>
            <option value="mp3">MP3</option>
            <option value="flac">FLAC</option>
            <option value="wav">WAV</option>
            <option value="m4a">AAC / M4A</option>
          </select>
        </label>
        <label>
          <span>Bit rate</span>
          <select aria-label="Bit rate" disabled={busy || lossless} value={lossless ? "original" : value.bitrate} onChange={(event) => {
            const next = event.target.value === "original" ? "original" : Number(event.target.value) as Settings["bitrate"];
            onChange({ ...value, bitrate: next });
          }}>
            <option value="original">{lossless ? "Original / lossless" : "Original"}</option>
            {!lossless && <>
              <option value={320}>320 kbps</option>
              <option value={256}>256 kbps</option>
              <option value={192}>192 kbps</option>
              <option value={128}>128 kbps</option>
            </>}
          </select>
        </label>
        <label>
          <span>Destination</span>
          <select aria-label="Destination" disabled={busy} value={value.destination} onChange={(event) => onChange({ ...value, destination: event.target.value as Settings["destination"] })}>
            <option value="browser">Browser Downloads · ZIP</option>
            <option value="project">Project root /output</option>
            <option value="custom">Other server path</option>
          </select>
        </label>
      </div>
      {value.destination === "custom" && (
        <label className="path-field">
          <span>Server output path</span>
          <input
            value={value.destinationPath}
            disabled={busy}
            placeholder="/absolute/path or a path relative to the server root"
            onChange={(event) => onChange({ ...value, destinationPath: event.target.value })}
          />
        </label>
      )}
      {value.destination === "project" && <p className="quiet-note">Tracks will be written to <code>output/&lt;album-name&gt;</code> under the server working directory.</p>}
      <p className="quiet-note">Every successful export includes <code>tracklist.txt</code>. Original keeps the source format and detected bit rate while preserving precise CUE cuts.</p>
    </section>
  );
}

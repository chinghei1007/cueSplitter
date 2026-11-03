import type { TrackDraft } from "../types";
import { formatDuration } from "../lib/files";
import { ProgressBar } from "./ProgressBar";

type Props = {
  tracks: TrackDraft[];
  busy: boolean;
  onRename: (index: number, name: string) => void;
};

export function TrackList({ tracks, busy, onRename }: Props) {
  return (
    <section className="section-block" aria-labelledby="tracks-title">
      <div className="section-heading">
        <h2 id="tracks-title">Expected tracks</h2>
        <span>{tracks.length} tracks · edit names before starting</span>
      </div>
      <div className="rows">
        {tracks.map((track) => {
          const progressLabel = track.status === "complete" ? "finished" : track.status === "processing" ? `${Math.round(track.progress * 100)}%` : track.status;
          return (
            <div className="track-row" key={`${track.number}-${track.startSeconds}`}>
              <span>{String(track.index + 1).padStart(2, "0")}</span>
              <span>{formatDuration(track.startSeconds)}–{formatDuration(track.endSeconds)}</span>
              <input
                aria-label={`Track ${track.index + 1} name`}
                value={track.name}
                disabled={busy}
                onChange={(event) => onRename(track.index, event.target.value)}
              />
              <ProgressBar value={track.progress} label={progressLabel} />
            </div>
          );
        })}
      </div>
    </section>
  );
}

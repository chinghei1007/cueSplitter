type CuePoint = {
  number: number;
  startSeconds: number;
  title: string;
  performer: string;
};

export type CueSheet = {
  title: string;
  performer: string;
  tracks: CuePoint[];
};

const trackPattern = /^TRACK\s+(\d+)\s+AUDIO$/i;
const indexPattern = /^INDEX\s+01\s+(\d+):(\d+):(\d+)$/i;
const titlePattern = /^TITLE\s+(?:"(.*)"|(.*))$/i;
const performerPattern = /^PERFORMER\s+(?:"(.*)"|(.*))$/i;

export function parseCue(text: string): CuePoint[] {
  return parseCueSheet(text).tracks;
}

export function parseCueSheet(text: string): CueSheet {
  const tracks: CuePoint[] = [];
  let current: CuePoint | undefined;
  let title = "";
  let performer = "";

  for (const rawLine of text.replace(/^\uFEFF/, "").split(/\r?\n/)) {
    const line = rawLine.trim();
    const trackMatch = line.match(trackPattern);
    if (trackMatch) {
      current = { number: Number(trackMatch[1]), startSeconds: Number.NaN, title: "", performer: "" };
      tracks.push(current);
      continue;
    }
    const titleMatch = line.match(titlePattern);
    if (titleMatch) {
      const value = (titleMatch[1] ?? titleMatch[2] ?? "").replace(/\\"/g, '"').trim();
      if (current) current.title = value;
      else title = value;
      continue;
    }
    const performerMatch = line.match(performerPattern);
    if (performerMatch) {
      const value = (performerMatch[1] ?? performerMatch[2] ?? "").replace(/\\"/g, '"').trim();
      if (current) current.performer = value;
      else performer = value;
      continue;
    }
    const indexMatch = line.match(indexPattern);
    if (!indexMatch) continue;
    if (!current) throw new Error("The CUE has INDEX 01 before its first AUDIO track.");
    const minutes = Number(indexMatch[1]);
    const seconds = Number(indexMatch[2]);
    const frames = Number(indexMatch[3]);
    if (seconds > 59 || frames > 74) throw new Error("The CUE contains an invalid timestamp.");
    current.startSeconds = minutes * 60 + seconds + frames / 75;
  }

  if (tracks.length === 0) throw new Error("The CUE contains no AUDIO tracks.");
  for (let index = 0; index < tracks.length; index += 1) {
    if (!Number.isFinite(tracks[index].startSeconds)) {
      throw new Error(`Track ${tracks[index].number} has no INDEX 01 timestamp.`);
    }
    if (index > 0 && tracks[index].startSeconds <= tracks[index - 1].startSeconds) {
      throw new Error("The CUE track timestamps are not in increasing order.");
    }
  }
  return { title, performer, tracks };
}

export type ClassifiedFiles = {
  all: File[];
  cue?: File;
  audio?: File;
  cover?: File;
  text?: File;
  ignored: File[];
};

export type NameSource = "txt" | "cue" | "generated";
export type TextEncoding = "auto" | "utf-8" | "big5" | "gb18030" | "shift_jis";

export type TrackDraft = {
  index: number;
  number: number;
  name: string;
  startSeconds: number;
  endSeconds: number;
  progress: number;
  status: "waiting" | "processing" | "complete" | "error";
};

export type InputPreview = {
  files: ClassifiedFiles;
  duration: number;
  tracks: TrackDraft[];
  albumTitle: string;
  artist: string;
  errors: string[];
  nameErrors: string[];
  nameSource: NameSource;
  textEncoding: TextEncoding;
};

export type ExportSettings = {
  format: "original" | "mp3" | "flac" | "wav" | "m4a";
  bitrate: "original" | 128 | 192 | 256 | 320;
  destination: "browser" | "project" | "custom";
  destinationPath: string;
};

export type TrackStatus = {
  index: number;
  name: string;
  startSeconds: number;
  endSeconds: number;
  progress: number;
  status: "waiting" | "processing" | "complete" | "error";
};

export type JobSnapshot = {
  id: string;
  state: "queued" | "processing" | "complete" | "error";
  message?: string;
  tracks: TrackStatus[];
  downloadUrl?: string;
  outputPath?: string;
  error?: string;
};

export type Health = {
  status: string;
  ffmpeg: boolean;
  ffprobe: boolean;
};

import { parseCueSheet } from "./cue";
import { decodeTextFile, parseNumberedTrackList } from "./tracklist";
import type { ClassifiedFiles, InputPreview, NameSource, TextEncoding, TrackDraft } from "../types";

const audioExtensions = new Set(["aac", "aiff", "ape", "flac", "m4a", "mp3", "ogg", "opus", "wav", "wma"]);
const imageExtensions = new Set(["jpeg", "jpg", "png", "webp"]);

export async function filesFromDrop(dataTransfer: DataTransfer): Promise<File[]> {
  const entries = Array.from(dataTransfer.items)
    .map((item) => item.webkitGetAsEntry())
    .filter((entry): entry is FileSystemEntry => Boolean(entry));

  if (entries.length === 0) return Array.from(dataTransfer.files);
  const nested = await Promise.all(entries.map(readEntry));
  return nested.flat();
}

async function readEntry(entry: FileSystemEntry): Promise<File[]> {
  if (entry.isFile) {
    const fileEntry = entry as FileSystemFileEntry;
    return new Promise((resolve, reject) => fileEntry.file((file) => resolve([file]), reject));
  }
  if (!entry.isDirectory) return [];
  const reader = (entry as FileSystemDirectoryEntry).createReader();
  const children: FileSystemEntry[] = [];
  while (true) {
    const batch = await new Promise<FileSystemEntry[]>((resolve, reject) => reader.readEntries(resolve, reject));
    if (batch.length === 0) break;
    children.push(...batch);
  }
  const nested = await Promise.all(children.map(readEntry));
  return nested.flat();
}

type PreviewOptions = {
  nameSource?: NameSource;
  textEncoding?: TextEncoding;
};

export async function createPreview(allFiles: File[], options: PreviewOptions = {}): Promise<InputPreview> {
  const files = classify(allFiles);
  const errors: string[] = [];
  const nameErrors: string[] = [];
  const textEncoding = options.textEncoding ?? "auto";
  if (!files.cue) errors.push("Add exactly one CUE file.");
  if (!files.audio) errors.push("Add exactly one supported audio file.");
  const cueCount = allFiles.filter((file) => extension(file.name) === "cue").length;
  const audioCount = allFiles.filter((file) => audioExtensions.has(extension(file.name))).length;
  if (cueCount > 1) errors.push(`Only one CUE file is supported; ${cueCount} were found.`);
  if (audioCount > 1) errors.push(`Only one album audio file is supported; ${audioCount} were found.`);

  let duration = 0;
  if (files.audio) {
    try {
      duration = await readAudioDuration(files.audio);
    } catch {
      errors.push("The browser could not preview the source duration; FFprobe will check it after upload.");
    }
  }

  const tracks: TrackDraft[] = [];
  let nameSource = options.nameSource ?? (files.text ? "txt" : "cue");
  let albumTitle = "";
  let artist = "";
  if (files.cue) {
    try {
      const structuralSheet = parseCueSheet(await files.cue.text());
      const cueTracks = structuralSheet.tracks;
      let textNames: string[] = [];
      if (nameSource === "txt" && files.text) {
        try {
          const parsed = parseNumberedTrackList(await decodeTextFile(files.text, textEncoding), cueTracks.length);
          textNames = parsed.names;
          nameErrors.push(...parsed.errors);
        } catch (error) {
          nameErrors.push(error instanceof Error ? error.message : "The song-list TXT could not be decoded.");
        }
      }

      let cueTitleNames = cueTracks.map((track) => track.title);
      if (nameSource === "cue" || (!files.text && options.nameSource === undefined)) {
        try {
          const decodedSheet = parseCueSheet(await decodeTextFile(files.cue, textEncoding));
          if (decodedSheet.tracks.length !== cueTracks.length) {
            nameErrors.push(`Decoded CUE has ${decodedSheet.tracks.length} tracks; expected ${cueTracks.length}.`);
          } else {
            cueTitleNames = decodedSheet.tracks.map((track) => track.title);
          }
          albumTitle = decodedSheet.title;
          artist = decodedSheet.performer;
        } catch (error) {
          nameErrors.push(error instanceof Error ? error.message : "The CUE titles could not be decoded.");
        }
      } else {
        try {
          const decodedSheet = parseCueSheet(await decodeTextFile(files.cue, "auto"));
          albumTitle = decodedSheet.title;
          artist = decodedSheet.performer;
        } catch {
          // Legacy CUE metadata can be filled after selecting CUE and its encoding.
        }
      }

      if (options.nameSource === undefined && !files.text && cueTitleNames.every((name) => !name.trim())) {
        nameSource = "generated";
      }
      if (nameSource === "txt" && !files.text) nameErrors.push("TXT song list is selected, but no TXT file was supplied.");
      if (nameSource === "cue") {
        cueTitleNames.forEach((name, index) => {
          if (!name.trim()) nameErrors.push(`CUE track ${String(index + 1).padStart(2, "0")} has no TITLE field.`);
        });
      }

      const selectedNames = nameSource === "txt" ? textNames : nameSource === "cue" ? cueTitleNames : [];
      cueTracks.forEach((track, index) => {
        tracks.push({
          index,
          number: track.number,
          name: selectedNames[index] || `track${String(index + 1).padStart(2, "0")}`,
          startSeconds: track.startSeconds,
          endSeconds: cueTracks[index + 1]?.startSeconds ?? duration,
          progress: 0,
          status: "waiting",
        });
      });
    } catch (error) {
      errors.push(error instanceof Error ? error.message : "The CUE could not be parsed.");
    }
  }
  errors.push(...nameErrors);
  return { files, duration, tracks, albumTitle, artist, errors, nameErrors, nameSource, textEncoding };
}

export function formatDuration(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "—";
  const whole = Math.max(0, Math.round(value));
  const hours = Math.floor(whole / 3600);
  const minutes = Math.floor((whole % 3600) / 60);
  const seconds = whole % 60;
  if (hours > 0) return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

function classify(allFiles: File[]): ClassifiedFiles {
  const cues = allFiles.filter((file) => extension(file.name) === "cue");
  const audio = allFiles.filter((file) => audioExtensions.has(extension(file.name)));
  const covers = allFiles.filter((file) => imageExtensions.has(extension(file.name)));
  const texts = allFiles.filter((file) => extension(file.name) === "txt");
  const recognized = new Set([...cues, ...audio, ...covers, ...texts]);
  return {
    all: allFiles,
    cue: cues.length === 1 ? cues[0] : undefined,
    audio: audio.length === 1 ? audio[0] : undefined,
    cover: preferred(covers, ["cover", "folder", "front"]),
    text: preferred(texts, ["music", "tracks", "titles"]),
    ignored: allFiles.filter((file) => !recognized.has(file)),
  };
}

function preferred(files: File[], words: string[]): File | undefined {
  for (const word of words) {
    const found = files.find((file) => file.name.toLowerCase().includes(word));
    if (found) return found;
  }
  return files[0];
}

function extension(name: string): string {
  return name.toLowerCase().split(".").pop() ?? "";
}

function readAudioDuration(file: File): Promise<number> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const audio = new Audio();
    const done = () => URL.revokeObjectURL(url);
    audio.preload = "metadata";
    audio.onloadedmetadata = () => {
      const value = audio.duration;
      done();
      if (Number.isFinite(value) && value > 0) resolve(value);
      else reject(new Error("invalid duration"));
    };
    audio.onerror = () => {
      done();
      reject(new Error("audio metadata unavailable"));
    };
    audio.src = url;
  });
}

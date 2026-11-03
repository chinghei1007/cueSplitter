import type { TextEncoding } from "../types";

export const numberedTrackPattern = /^\s*(\d{1,3})\s*[.)：:、—–-]\s+(.+?)\s*$/u;

const decoderLabels: Record<Exclude<TextEncoding, "auto">, string> = {
  "utf-8": "utf-8",
  big5: "big5",
  gb18030: "gb18030",
  shift_jis: "shift_jis",
};

export async function decodeTextFile(file: File, encoding: TextEncoding): Promise<string> {
  const bytes = new Uint8Array(await file.arrayBuffer());
  if (encoding === "auto") {
    try {
      return new TextDecoder("utf-8", { fatal: true }).decode(bytes).replace(/^\uFEFF/, "");
    } catch {
      throw new Error(`Could not decode ${file.name} as UTF-8. Choose Big5, GB18030, or Shift_JIS.`);
    }
  }
  try {
    return new TextDecoder(decoderLabels[encoding], { fatal: true }).decode(bytes).replace(/^\uFEFF/, "");
  } catch {
    throw new Error(`Could not decode ${file.name} as ${encodingLabel(encoding)}.`);
  }
}

export function parseNumberedTrackList(text: string, expectedCount: number): { names: string[]; errors: string[] } {
  const names = Array.from({ length: expectedCount }, () => "");
  const errors: string[] = [];
  const lines = text.replace(/^\uFEFF/, "").split(/\r?\n/);
  let expectedNumber = 1;
  let numberedLineCount = 0;
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index].trim();
    if (!line) continue;
    numberedLineCount += 1;
    const match = line.match(numberedTrackPattern);
    if (!match) {
      errors.push(`Song-list line ${index + 1} must look like ${String(expectedNumber).padStart(2, "0")}. Song title.`);
      expectedNumber += 1;
      continue;
    }
    const number = Number(match[1]);
    if (number !== expectedNumber) {
      errors.push(`Song-list line ${index + 1} must be track ${String(expectedNumber).padStart(2, "0")}, not ${match[1]}.`);
    } else if (expectedNumber <= expectedCount) {
      names[expectedNumber - 1] = match[2].trim();
    }
    expectedNumber += 1;
  }
  if (numberedLineCount !== expectedCount) {
    errors.push(`The song list has ${numberedLineCount} numbered lines, but the CUE has ${expectedCount} tracks.`);
  }
  return { names, errors };
}

export function titleOnlyError(name: string, index: number): string | undefined {
  if (!name.trim()) return `Track ${String(index + 1).padStart(2, "0")} needs a title.`;
  if (numberedTrackPattern.test(name)) {
    return `Track ${String(index + 1).padStart(2, "0")} already has its row number; remove the number from the title.`;
  }
  return undefined;
}

export function encodingLabel(value: TextEncoding): string {
  switch (value) {
    case "auto": return "Auto";
    case "utf-8": return "UTF-8";
    case "big5": return "Big5";
    case "gb18030": return "GB18030";
    case "shift_jis": return "Shift_JIS";
  }
}

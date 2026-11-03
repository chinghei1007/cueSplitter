import { useRef, useState } from "react";
import { filesFromDrop } from "../lib/files";

type Props = {
  busy: boolean;
  onFiles: (files: File[]) => void;
};

const directoryProps = { webkitdirectory: "", directory: "" } as Record<string, string>;

export function DropArea({ busy, onFiles }: Props) {
  const folderInput = useRef<HTMLInputElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);

  return (
    <div
      className={`drop-area${dragging ? " is-dragging" : ""}${busy ? " is-disabled" : ""}`}
      onDragEnter={(event) => {
        event.preventDefault();
        if (!busy) setDragging(true);
      }}
      onDragOver={(event) => event.preventDefault()}
      onDragLeave={(event) => {
        if (event.currentTarget === event.target) setDragging(false);
      }}
      onDrop={async (event) => {
        event.preventDefault();
        setDragging(false);
        if (!busy) onFiles(await filesFromDrop(event.dataTransfer));
      }}
    >
      <p>Drop one folder or the individual album files here</p>
      <div className="drop-actions">
        <button type="button" disabled={busy} onClick={() => folderInput.current?.click()}>Choose folder</button>
        <span>or</span>
        <button type="button" disabled={busy} onClick={() => fileInput.current?.click()}>Choose files</button>
      </div>
      <input
        ref={folderInput}
        className="visually-hidden"
        type="file"
        multiple
        disabled={busy}
        {...directoryProps}
        onChange={(event) => onFiles(Array.from(event.currentTarget.files ?? []))}
      />
      <input
        ref={fileInput}
        className="visually-hidden"
        type="file"
        multiple
        disabled={busy}
        onChange={(event) => onFiles(Array.from(event.currentTarget.files ?? []))}
      />
    </div>
  );
}

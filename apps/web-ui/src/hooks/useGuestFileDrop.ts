import { useCallback, useRef, useState } from 'react';
import { api, ApiError } from '../api/client';

export interface Transfer {
  key: string;
  name: string;
  state: 'uploading' | 'done' | 'error';
  message?: string;
}

interface GuestCreds {
  username: string;
  password: string;
}

// Guest credentials are cached in memory only, keyed by VM, for the lifetime of
// the page. They are never written to disk or storage, so a reload clears them —
// this is the "cache for the session" the user chose over prompting per file.
const credCache = new Map<string, GuestCreds>();

let transferSeq = 0;

// useGuestFileDrop wires a drop target to the hybrid host→guest file transfer.
// On drop it decides, per the VM's shared-folder presence, whether guest
// credentials are needed: a VM with a shared folder transfers with no prompt; a
// VM without one prompts once and reuses the cached credentials thereafter.
export function useGuestFileDrop(vmId: string) {
  const [dragging, setDragging] = useState(false);
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [credPrompt, setCredPrompt] = useState<File[] | null>(null);
  // Nested children fire dragenter/leave as the pointer crosses them; a depth
  // counter keeps the overlay from flickering until the drag truly leaves.
  const dragDepth = useRef(0);

  const setTransfer = useCallback((key: string, patch: Partial<Transfer>) => {
    setTransfers((list) => list.map((t) => (t.key === key ? { ...t, ...patch } : t)));
  }, []);

  const dismiss = useCallback((key: string) => {
    setTransfers((list) => list.filter((t) => t.key !== key));
  }, []);

  const upload = useCallback(
    async (files: File[], creds?: GuestCreds) => {
      for (const file of files) {
        const key = `xfer-${transferSeq++}`;
        setTransfers((list) => [...list, { key, name: file.name, state: 'uploading' }]);
        try {
          const res = await api.transferFileToGuest(vmId, file, creds);
          if (res.credentialsRequired) {
            // The VM lost its shared folder between the pre-check and now, and no
            // credentials were cached: ask for them and let the user retry.
            setTransfer(key, { state: 'error', message: 'Guest credentials required.' });
            setCredPrompt([file]);
          } else if (res.success) {
            setTransfer(key, { state: 'done', message: res.message });
            setTimeout(() => dismiss(key), 5000);
          } else {
            setTransfer(key, { state: 'error', message: res.message || 'Transfer failed.' });
          }
        } catch (err) {
          const message =
            err instanceof ApiError && err.body.trim() !== ''
              ? err.body.trim()
              : err instanceof Error
                ? err.message
                : 'Transfer failed.';
          setTransfer(key, { state: 'error', message });
        }
      }
    },
    [vmId, setTransfer, dismiss],
  );

  const onDrop = useCallback(
    async (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      dragDepth.current = 0;
      setDragging(false);

      const files = Array.from(e.dataTransfer?.files ?? []);
      if (files.length === 0) return;

      const cached = credCache.get(vmId);
      // Pre-check shared folders so we don't upload a large file only to discover
      // credentials are needed. A VM with a shared folder never needs them.
      let hasShare = false;
      try {
        const sf = await api.getSharedFolders(vmId);
        hasShare = sf.folders.length > 0;
      } catch {
        // Treat a failed lookup as "no share"; the backend still validates.
      }

      if (hasShare) {
        void upload(files);
      } else if (cached) {
        void upload(files, cached);
      } else {
        setCredPrompt(files);
      }
    },
    [vmId, upload],
  );

  const onDragEnter = useCallback((e: React.DragEvent) => {
    if (!e.dataTransfer?.types?.includes('Files')) return;
    e.preventDefault();
    dragDepth.current += 1;
    setDragging(true);
  }, []);

  const onDragOver = useCallback((e: React.DragEvent) => {
    if (!e.dataTransfer?.types?.includes('Files')) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
  }, []);

  const onDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    dragDepth.current = Math.max(0, dragDepth.current - 1);
    if (dragDepth.current === 0) setDragging(false);
  }, []);

  const submitCreds = useCallback(
    (username: string, password: string) => {
      const creds = { username, password };
      credCache.set(vmId, creds);
      const files = credPrompt ?? [];
      setCredPrompt(null);
      void upload(files, creds);
    },
    [vmId, credPrompt, upload],
  );

  const cancelCreds = useCallback(() => setCredPrompt(null), []);

  return {
    dragging,
    transfers,
    credPrompt,
    dropHandlers: { onDrop, onDragEnter, onDragOver, onDragLeave },
    submitCreds,
    cancelCreds,
    dismiss,
  };
}

// Guest credentials are cached in memory only, keyed by VM, for the lifetime of
// the page. They are never written to disk or storage, so a reload clears them —
// this is the "cache for the session" the user chose over prompting per action.
// The cache is shared across every guest-control feature (drag-drop file
// transfer, run-in-guest, copy-from-guest) so credentials entered once in any of
// them are reused by the others for that VM.

export interface GuestCreds {
  username: string;
  password: string;
}

const credCache = new Map<string, GuestCreds>();

export function getGuestCreds(vmId: string): GuestCreds | undefined {
  return credCache.get(vmId);
}

export function setGuestCreds(vmId: string, creds: GuestCreds): void {
  credCache.set(vmId, creds);
}

export function clearGuestCreds(vmId: string): void {
  credCache.delete(vmId);
}

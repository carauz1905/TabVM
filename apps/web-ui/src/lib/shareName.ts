// deriveShareName turns a host folder path into a valid, unique VirtualBox
// shared-folder name so the user never has to type one. VBox share names (and
// the guest mount point derived from them) must be conservative, matching the
// agent's server-side pattern ^[A-Za-z0-9._-]{1,64}$.
//
// The basename is sanitized (invalid characters collapse to '-'), clamped to 64
// characters, and de-duplicated against names already shared on the VM by
// appending -2, -3, … so two folders called "share" don't collide.
export function deriveShareName(hostPath: string, existingNames: string[] = []): string {
  const segments = hostPath.split(/[\\/]+/).filter((s) => s.trim() !== '');
  const base = segments.length > 0 ? segments[segments.length - 1] : '';

  let name = base
    .replace(/[^A-Za-z0-9._-]+/g, '-')
    .replace(/^[-._]+|[-._]+$/g, '')
    .slice(0, 64);
  if (name === '') name = 'share';

  const taken = new Set(existingNames.map((n) => n.toLowerCase()));
  if (!taken.has(name.toLowerCase())) return name;

  // Reserve room for the "-N" suffix within the 64-char limit.
  for (let i = 2; i < 1000; i++) {
    const suffix = `-${i}`;
    const candidate = name.slice(0, 64 - suffix.length) + suffix;
    if (!taken.has(candidate.toLowerCase())) return candidate;
  }
  return name; // pathological fallback; the backend still validates.
}

// guestMountPath is the Linux Guest Additions mount point for a share name.
// VirtualBox auto-mounts shares under /media/sf_<name>.
export function guestMountPath(shareName: string): string {
  return `/media/sf_${shareName}`;
}

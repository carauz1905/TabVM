import { describe, it, expect } from 'vitest';
import { deriveShareName, guestMountPath } from './shareName';

describe('deriveShareName', () => {
  it('uses the folder basename from a Windows path', () => {
    expect(deriveShareName('C:\\Users\\Asistente\\labs\\share')).toBe('share');
  });

  it('uses the folder basename from a POSIX path', () => {
    expect(deriveShareName('/home/user/pentest/ctf-drop')).toBe('ctf-drop');
  });

  it('sanitizes spaces and invalid characters to dashes', () => {
    expect(deriveShareName('D:\\CTF Drops (2026)')).toBe('CTF-Drops-2026');
  });

  it('trims leading and trailing separators left by sanitizing', () => {
    expect(deriveShareName('C:\\@weird@')).toBe('weird');
  });

  it('falls back to "share" when the basename has no valid characters', () => {
    expect(deriveShareName('C:\\@@@')).toBe('share');
  });

  it('handles a trailing slash on the path', () => {
    expect(deriveShareName('C:\\labs\\share\\')).toBe('share');
  });

  it('de-duplicates against existing names case-insensitively', () => {
    expect(deriveShareName('C:\\a\\share', ['share'])).toBe('share-2');
    expect(deriveShareName('C:\\a\\Share', ['share', 'share-2'])).toBe('Share-3');
  });

  it('clamps to 64 characters', () => {
    const long = 'x'.repeat(100);
    expect(deriveShareName(`C:\\${long}`).length).toBe(64);
  });
});

describe('guestMountPath', () => {
  it('builds the VirtualBox auto-mount path', () => {
    expect(guestMountPath('labshare')).toBe('/media/sf_labshare');
  });
});

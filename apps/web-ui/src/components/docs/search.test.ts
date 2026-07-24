import { describe, it, expect } from 'vitest';
import { matchesQuery, normalizeForSearch } from './search';

describe('normalizeForSearch', () => {
  it('lowercases the text', () => {
    expect(normalizeForSearch('USB Devices')).toBe('usb devices');
  });

  it('strips diacritics', () => {
    expect(normalizeForSearch('Búsqueda RÁPIDA')).toBe('busqueda rapida');
    expect(normalizeForSearch('Instantáneas')).toBe('instantaneas');
  });
});

describe('matchesQuery', () => {
  it('is case-insensitive', () => {
    expect(matchesQuery('USB devices', 'usb')).toBe(true);
    expect(matchesQuery('usb devices', 'USB')).toBe(true);
  });

  it('is diacritic-insensitive in both directions', () => {
    // Accented haystack, plain query…
    expect(matchesQuery('búsqueda', 'busqueda')).toBe(true);
    // …and plain haystack, accented query.
    expect(matchesQuery('busqueda', 'búsqueda')).toBe(true);
  });

  it('matches everything on an empty or whitespace-only query', () => {
    expect(matchesQuery('anything', '')).toBe(true);
    expect(matchesQuery('anything', '   ')).toBe(true);
    expect(matchesQuery('', '')).toBe(true);
  });

  it('rejects a query that does not appear in the haystack', () => {
    expect(matchesQuery('USB devices', 'snapshot')).toBe(false);
  });
});

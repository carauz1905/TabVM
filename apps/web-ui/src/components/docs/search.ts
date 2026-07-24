import type { DocsStrings } from './content';

// Manual search helpers. Pure functions so the matching rules are unit-testable
// apart from the DocsView component.

export type DocsSectionId = keyof DocsStrings['sections'];

// normalizeForSearch folds case and strips combining diacritics so that
// "busqueda" finds "búsqueda" and vice versa.
export function normalizeForSearch(text: string): string {
  return text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase();
}

// matchesQuery reports whether the haystack contains the query, ignoring case
// and diacritics. An empty (or whitespace-only) query matches everything.
export function matchesQuery(haystack: string, query: string): boolean {
  const q = normalizeForSearch(query.trim());
  if (!q) return true;
  return normalizeForSearch(haystack).includes(q);
}

// collectStrings gathers every string leaf of a docs content subtree, so the
// haystack covers leads, titles, bodies, cards, steps, FAQ entries and tips
// without knowing each section's shape.
function collectStrings(value: unknown, out: string[]): void {
  if (typeof value === 'string') {
    out.push(value);
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) collectStrings(item, out);
    return;
  }
  if (value && typeof value === 'object') {
    for (const item of Object.values(value)) collectStrings(item, out);
  }
}

// sectionHaystack builds the searchable text of one manual section in the
// active language: its TOC title plus every string in its content subtree.
export function sectionHaystack(d: DocsStrings, id: DocsSectionId): string {
  const out: string[] = [d.sections[id]];
  collectStrings(d[id], out);
  return out.join('\n');
}

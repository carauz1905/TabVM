import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, cleanup, fireEvent } from '@testing-library/react';
import { DocsView } from './DocsView';
import { LanguageProvider } from '../../i18n/i18n';

// The manual has 13 sections; the suite relies on that count to prove the
// filter restores everything.
const SECTION_COUNT = 13;

// Every element handed to IntersectionObserver.observe across all instances,
// so tests can prove the scroll-spy re-attaches to remounted sections.
let observed: Element[] = [];

describe('DocsView', () => {
  beforeEach(() => {
    observed = [];
    // jsdom does not implement IntersectionObserver, which the docs scroll-spy
    // and the credits signature use.
    class MockIntersectionObserver {
      observe = (el: Element) => {
        observed.push(el);
      };
      unobserve = vi.fn();
      disconnect = vi.fn();
    }
    vi.stubGlobal('IntersectionObserver', MockIntersectionObserver);
    // jsdom does not implement matchMedia either, which the personalize demo
    // and the credits signature consult.
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    );
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it('renders all sections and TOC entries when the search is empty', () => {
    const { container, getByLabelText } = render(<DocsView />);

    expect(container.querySelectorAll('.docs-sec')).toHaveLength(SECTION_COUNT);
    expect(container.querySelectorAll('.docs-nav-link')).toHaveLength(SECTION_COUNT);
    const input = getByLabelText('Search the manual') as HTMLInputElement;
    expect(input.value).toBe('');
  });

  it('filters the rendered sections and the TOC to query matches', () => {
    const { container, getByLabelText, queryByText } = render(<DocsView />);

    fireEvent.change(getByLabelText('Search the manual'), { target: { value: 'USB' } });

    expect(container.querySelector('#usb')).toBeTruthy();
    expect(container.querySelector('#snapshots')).toBeNull();
    expect(queryByText('Quick start')).toBeNull();
    const tocLabels = Array.from(container.querySelectorAll('.docs-nav-link')).map((b) => b.textContent);
    expect(tocLabels).toEqual(['USB devices']);
  });

  it('matches diacritic-insensitively against the Spanish content', () => {
    localStorage.setItem('tabvm.lang', 'es');
    const { container, getByLabelText } = render(
      <LanguageProvider>
        <DocsView />
      </LanguageProvider>,
    );

    // Plain "instantanea" must find the section titled "Instantáneas".
    fireEvent.change(getByLabelText('Buscar en el manual'), { target: { value: 'instantanea' } });

    expect(container.querySelector('#snapshots')).toBeTruthy();
    expect(container.querySelector('#usb')).toBeNull();
    const tocLabels = Array.from(container.querySelectorAll('.docs-nav-link')).map((b) => b.textContent);
    expect(tocLabels).toContain('Instantáneas');
  });

  it('shows a no-results block and the clear button restores every section', () => {
    const { container, getByLabelText, getByText, queryByText } = render(<DocsView />);

    fireEvent.change(getByLabelText('Search the manual'), { target: { value: 'zzzqqq' } });

    expect(getByText('No results for "zzzqqq"')).toBeTruthy();
    expect(container.querySelectorAll('.docs-sec')).toHaveLength(0);

    fireEvent.click(getByLabelText('Clear search'));

    expect(queryByText(/No results/)).toBeNull();
    expect(container.querySelectorAll('.docs-sec')).toHaveLength(SECTION_COUNT);
  });

  it('re-attaches the scroll-spy to remounted sections after clearing a filter', () => {
    const { container, getByLabelText } = render(<DocsView />);

    fireEvent.change(getByLabelText('Search the manual'), { target: { value: 'USB' } });
    fireEvent.click(getByLabelText('Clear search'));

    // The most recently observed #snapshots element must be the live node the
    // clear remounted — not the stale one from the first mount.
    const snapshots = observed.filter((el) => el.id === 'snapshots').pop();
    expect(snapshots).toBeTruthy();
    expect(snapshots!.isConnected).toBe(true);
    expect(snapshots).toBe(container.querySelector('#snapshots'));
  });

  it('renders the Spanish placeholder, clear label and no-results text', () => {
    localStorage.setItem('tabvm.lang', 'es');
    const { getByLabelText, getByPlaceholderText, getByText } = render(
      <LanguageProvider>
        <DocsView />
      </LanguageProvider>,
    );

    const input = getByPlaceholderText('Buscar en el manual');
    fireEvent.change(input, { target: { value: 'zzzqqq' } });

    expect(getByText('Sin resultados para "zzzqqq"')).toBeTruthy();
    expect(getByLabelText('Limpiar búsqueda')).toBeTruthy();
  });
});

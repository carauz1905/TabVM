import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/react';
import { AppShell, type ShellView } from './AppShell';

function renderShell(overrides: { active?: ShellView; onNavigate?: (v: ShellView) => void } = {}) {
  const onNavigate = overrides.onNavigate ?? vi.fn();
  const utils = render(
    <AppShell active={overrides.active ?? 'machines'} onNavigate={onNavigate} crumb="machines" agentOnline>
      <p>content</p>
    </AppShell>,
  );
  return { ...utils, onNavigate };
}

describe('AppShell', () => {
  beforeEach(() => {
    // jsdom does not implement matchMedia, which the theme toggle consults.
    window.matchMedia = vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    document.documentElement.removeAttribute('data-theme');
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    document.documentElement.removeAttribute('data-theme');
  });

  it('renders the brand wordmark and navigation', () => {
    const { getByText, getByLabelText } = renderShell();

    expect(getByText('content')).toBeTruthy();
    expect(getByLabelText('Virtual machines')).toBeTruthy();
    expect(getByLabelText('Activity')).toBeTruthy();
    expect(getByLabelText('Agent')).toBeTruthy();
  });

  it('calls onNavigate when a nav item is clicked', () => {
    const { getByLabelText, onNavigate } = renderShell();

    fireEvent.click(getByLabelText('Activity'));
    expect(onNavigate).toHaveBeenCalledWith('activity');
  });

  it('marks the active view with aria-current', () => {
    const { getByLabelText } = renderShell({ active: 'agent' });
    expect(getByLabelText('Agent').getAttribute('aria-current')).toBe('page');
    expect(getByLabelText('Virtual machines').getAttribute('aria-current')).toBeNull();
  });

  it('collapses and expands the sidebar via the collapse control', () => {
    const { getByLabelText, container } = renderShell();
    const app = container.querySelector('.tv-app')!;

    expect(app.className).not.toContain('collapsed');
    fireEvent.click(getByLabelText('Collapse sidebar'));
    expect(app.className).toContain('collapsed');
  });

  it('expands a collapsed sidebar when the logo mark is clicked', () => {
    const { getByLabelText, container } = renderShell();
    const app = container.querySelector('.tv-app')!;

    fireEvent.click(getByLabelText('Collapse sidebar'));
    expect(app.className).toContain('collapsed');
    // The logo mark toggles the rail too — click it to expand again.
    fireEvent.click(getByLabelText('Toggle sidebar'));
    expect(app.className).not.toContain('collapsed');
  });

  it('toggles the theme attribute on the document root', () => {
    const { getByLabelText } = renderShell();

    fireEvent.click(getByLabelText('Toggle theme'));
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    fireEvent.click(getByLabelText('Toggle theme'));
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('persists the selected theme so a reload keeps it', () => {
    const { getByLabelText } = renderShell();

    fireEvent.click(getByLabelText('Toggle theme'));
    expect(localStorage.getItem('tabvm.theme')).toBe('dark');
    fireEvent.click(getByLabelText('Toggle theme'));
    expect(localStorage.getItem('tabvm.theme')).toBe('light');
  });

  it('shows the agent offline state when not online', () => {
    const { getAllByText } = render(
      <AppShell active="machines" onNavigate={vi.fn()} crumb="machines" agentOnline={false}>
        <p>content</p>
      </AppShell>,
    );

    expect(getAllByText('agent offline').length).toBeGreaterThan(0);
  });
});

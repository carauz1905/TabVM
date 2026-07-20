import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/react';
import { UpdateBanner } from './UpdateBanner';
import type { UpdateStatus } from '../types/api';

const available: UpdateStatus = {
  current: '0.1.2',
  latest: '0.1.3',
  updateAvailable: true,
  releaseUrl: 'https://github.com/carauz1905/TabVM/releases/tag/v0.1.3',
};

describe('UpdateBanner', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it('renders when an update is available and not dismissed', () => {
    const { getByText } = render(<UpdateBanner status={available} />);
    expect(getByText(/TabVM v0\.1\.3 is available/)).toBeTruthy();
  });

  it('renders nothing when no update is available', () => {
    const { container } = render(
      <UpdateBanner status={{ current: '0.1.2', updateAvailable: false }} />,
    );
    expect(container.querySelector('.update-banner')).toBeNull();
  });

  it('links Download to the release URL and opens it safely', () => {
    const { getByText } = render(<UpdateBanner status={available} />);
    const link = getByText('Download') as HTMLAnchorElement;
    expect(link.getAttribute('href')).toBe(available.releaseUrl);
    expect(link.getAttribute('target')).toBe('_blank');
    expect(link.getAttribute('rel')).toBe('noopener noreferrer');
  });

  it('shows a Scoop upgrade hint', () => {
    const { getByText } = render(<UpdateBanner status={available} />);
    expect(getByText(/scoop update tabvm/)).toBeTruthy();
  });

  it('dismisses the banner and persists the dismissed version', () => {
    const { getByLabelText, container } = render(<UpdateBanner status={available} />);
    expect(container.querySelector('.update-banner')).toBeTruthy();

    fireEvent.click(getByLabelText('Dismiss'));

    expect(container.querySelector('.update-banner')).toBeNull();
    expect(localStorage.getItem('tabvm.updateDismissed')).toBe('0.1.3');
  });

  it('stays hidden when the current latest was already dismissed', () => {
    localStorage.setItem('tabvm.updateDismissed', '0.1.3');
    const { container } = render(<UpdateBanner status={available} />);
    expect(container.querySelector('.update-banner')).toBeNull();
  });

  it('reappears when a newer version ships than the one dismissed', () => {
    localStorage.setItem('tabvm.updateDismissed', '0.1.3');
    const { container } = render(
      <UpdateBanner
        status={{ ...available, latest: '0.1.4', releaseUrl: 'https://example.com/0.1.4' }}
      />,
    );
    expect(container.querySelector('.update-banner')).toBeTruthy();
  });
});

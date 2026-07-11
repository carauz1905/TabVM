import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/react';
import { ConsolePreview } from './ConsolePreview';

// jsdom's canvas has no 2D context, so the stream short-circuits and the
// component renders its status label without opening a WebSocket.
describe('ConsolePreview', () => {
  afterEach(() => cleanup());

  it('renders a connecting state and a click-to-attach hint', () => {
    const { getByText, getByLabelText } = render(<ConsolePreview vmId="x" onOpen={() => {}} />);

    expect(getByLabelText('Open live console')).toBeTruthy();
    expect(getByText('connecting…')).toBeTruthy();
    expect(getByText(/click to attach/i)).toBeTruthy();
  });

  it('calls onOpen when clicked', () => {
    const onOpen = vi.fn();
    const { getByLabelText } = render(<ConsolePreview vmId="x" onOpen={onOpen} />);

    fireEvent.click(getByLabelText('Open live console'));
    expect(onOpen).toHaveBeenCalledTimes(1);
  });
});

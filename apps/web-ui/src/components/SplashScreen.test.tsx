import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, cleanup, act } from '@testing-library/react';
import { SplashScreen } from './SplashScreen';

function mockMotion(reduce: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('reduced-motion') ? reduce : false,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })) as unknown as typeof window.matchMedia;
}

describe('SplashScreen', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockMotion(false);
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    cleanup();
  });

  it('types the slogan progressively, then fades out and calls onDone', () => {
    const onDone = vi.fn();
    const { getByText } = render(
      <SplashScreen onDone={onDone} slogan="ab" startDelayMs={10} typeSpeedMs={10} holdMs={10} fadeMs={10} />,
    );

    act(() => void vi.advanceTimersByTime(10)); // start delay elapses, typing begins
    act(() => void vi.advanceTimersByTime(10)); // first char
    act(() => void vi.advanceTimersByTime(10)); // slogan complete

    expect(getByText('ab')).toBeTruthy();
    expect(onDone).not.toHaveBeenCalled();

    act(() => void vi.advanceTimersByTime(10)); // hold elapses, fade begins
    act(() => void vi.advanceTimersByTime(10)); // fade elapses, done

    expect(onDone).toHaveBeenCalledTimes(1);
  });

  it('renders the full slogan immediately when reduced motion is preferred', () => {
    mockMotion(true);
    const onDone = vi.fn();
    const { getByText } = render(
      <SplashScreen onDone={onDone} slogan="hello" startDelayMs={999} typeSpeedMs={999} holdMs={5} fadeMs={5} />,
    );

    expect(getByText('hello')).toBeTruthy();

    act(() => void vi.advanceTimersByTime(5)); // hold
    act(() => void vi.advanceTimersByTime(5)); // fade
    expect(onDone).toHaveBeenCalledTimes(1);
  });

  it('calls onDone exactly once', () => {
    const onDone = vi.fn();
    render(<SplashScreen onDone={onDone} slogan="x" startDelayMs={1} typeSpeedMs={1} holdMs={1} fadeMs={1} />);

    act(() => void vi.advanceTimersByTime(200));
    expect(onDone).toHaveBeenCalledTimes(1);
  });
});

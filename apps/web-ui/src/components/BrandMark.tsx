interface BrandMarkProps {
  // When true, the tab drops in and the panel draws itself once on mount.
  animated?: boolean;
  // Render the tab in a muted tone for stopped machines.
  muted?: boolean;
  className?: string;
}

// BrandMark is the TabVM logo mark: an outlined panel with a solid tab on top
// and a prompt caret inside — a VM session living in a browser tab.
export function BrandMark({ animated = false, muted = false, className }: BrandMarkProps) {
  const tabFill = muted ? 'var(--muted)' : 'var(--accent)';
  return (
    <svg viewBox="0 0 64 64" className={className} aria-hidden="true">
      <rect
        className={animated ? 'mk-panel' : undefined}
        x="8"
        y="20"
        width="48"
        height="36"
        rx="8"
        fill="none"
        stroke="var(--mark)"
        strokeWidth="4"
      />
      <rect
        className={animated ? 'mk-tab' : undefined}
        x="14"
        y="8"
        width="23"
        height="18"
        rx="5"
        fill={tabFill}
      />
      <path
        className={animated ? 'mk-prompt' : undefined}
        d="M19 38 l6.5 4.4 -6.5 4.4"
        fill="none"
        stroke="var(--mark)"
        strokeWidth="3.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {!muted && (
        <rect
          className={animated ? 'mk-caret' : undefined}
          x="31"
          y="44.6"
          width="14"
          height="3.6"
          rx="1.8"
          fill="var(--accent)"
        />
      )}
    </svg>
  );
}

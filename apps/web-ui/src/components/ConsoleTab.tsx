import { useEffect } from 'react';
import { ScreenConsole } from './ScreenConsole';

interface ConsoleTabProps {
  vmId: string;
  vmName: string;
}

// ConsoleTab is the whole page when the app is opened at ?console=<id>: just the
// live console, full-screen, with no dashboard chrome. Closing it closes the tab.
export function ConsoleTab({ vmId, vmName }: ConsoleTabProps) {
  useEffect(() => {
    const previous = document.title;
    document.title = `${vmName} — TabVM console`;
    return () => {
      document.title = previous;
    };
  }, [vmName]);

  return <ScreenConsole vmId={vmId} vmName={vmName} fullscreen onClose={() => window.close()} />;
}

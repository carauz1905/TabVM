import { useCallback, useState } from 'react';
import { AppShell, type ShellView } from './components/AppShell';
import { MachinesView } from './components/MachinesView';
import { ActivityView } from './components/ActivityView';
import { AgentView } from './components/AgentView';
import { DocsView } from './components/docs/DocsView';
import { ConsoleTab } from './components/ConsoleTab';
import { TerminalTab } from './components/TerminalTab';
import { SplashScreen } from './components/SplashScreen';
import { useHealth } from './hooks/useHealth';

const crumbs: Record<ShellView, string> = {
  machines: 'virtual machines',
  activity: 'activity',
  agent: 'agent',
  docs: 'docs',
};

const SPLASH_SEEN_KEY = 'tabvm.splash.seen';

// shouldShowSplash decides whether to play the launch intro. The desktop
// launcher opens the tab with ?splash=1 to force it; otherwise it plays once
// per browser session so a normal refresh does not replay it.
function shouldShowSplash(): boolean {
  const forced = new URLSearchParams(window.location.search).get('splash');
  if (forced === '1') return true;
  if (forced === '0') return false;
  try {
    return sessionStorage.getItem(SPLASH_SEEN_KEY) !== '1';
  } catch {
    return true;
  }
}

// Workspace is the normal dashboard (sidebar shell + views).
function Workspace() {
  const health = useHealth();
  const agentOnline = health.state === 'success' && health.data?.status === 'healthy';
  const [view, setView] = useState<ShellView>('machines');
  const [showSplash, setShowSplash] = useState(shouldShowSplash);

  const dismissSplash = useCallback(() => {
    try {
      sessionStorage.setItem(SPLASH_SEEN_KEY, '1');
    } catch {
      // sessionStorage may be unavailable; the splash still dismisses.
    }
    setShowSplash(false);
  }, []);

  return (
    <>
      {showSplash && <SplashScreen onDone={dismissSplash} />}
      <AppShell active={view} onNavigate={setView} crumb={crumbs[view]} agentOnline={agentOnline} version={health.data?.version}>
        {view === 'machines' && <MachinesView />}
        {view === 'activity' && <ActivityView />}
        {view === 'agent' && <AgentView />}
        {view === 'docs' && <DocsView />}
      </AppShell>
    </>
  );
}

function App() {
  // ?console=<id>&name=<name> renders only the full-screen console (a dedicated
  // browser tab opened from the machine list). Everything else is the dashboard.
  const params = new URLSearchParams(window.location.search);
  const consoleId = params.get('console');
  if (consoleId) {
    return <ConsoleTab vmId={consoleId} vmName={params.get('name') ?? consoleId} />;
  }

  // ?terminal=<id>&name=<name> renders only the full-screen serial terminal.
  const terminalId = params.get('terminal');
  if (terminalId) {
    return <TerminalTab vmId={terminalId} vmName={params.get('name') ?? terminalId} />;
  }

  return <Workspace />;
}

export default App;

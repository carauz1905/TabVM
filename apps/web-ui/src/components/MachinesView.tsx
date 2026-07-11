import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api, ApiError } from '../api/client';
import { useHealth } from '../hooks/useHealth';
import { useVmStatus } from '../hooks/useVmStatus';
import { BrandMark } from './BrandMark';
import { ConsolePreview } from './ConsolePreview';
import { FilesPanel } from './FilesPanel';
import { HardwarePanel } from './HardwarePanel';
import { StoragePanel } from './StoragePanel';
import { NetworkPanel } from './NetworkPanel';
import { ScreenConsole } from './ScreenConsole';
import { SnapshotsPanel } from './SnapshotsPanel';
import { CreateVmWizard } from './CreateVmWizard';
import { useT } from '../i18n/i18n';
import type {
  GuestAdditionsStatusResponse,
  GuestAdditionsUpdateResponse,
  LocalStateStatusResponse,
  VmInfo,
  VmTelemetryResponse,
} from '../types/api';

type VmAction = 'start' | 'stop' | 'reset' | 'delete';
const lifecycleActions: VmAction[] = ['start', 'stop', 'reset', 'delete'];

// formatUptime renders a seconds count as HH:MM:SS for the agent meta line.
function formatUptime(totalSeconds: number): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  const h = Math.floor(totalSeconds / 3600);
  const m = Math.floor((totalSeconds % 3600) / 60);
  const s = Math.floor(totalSeconds % 60);
  return `${pad(h)}:${pad(m)}:${pad(s)}`;
}

function formatRam(mb: number): string {
  if (mb <= 0) return '—';
  if (mb % 1024 === 0) return `${mb / 1024} GB`;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 GB';
  const gb = bytes / 1024 ** 3;
  if (gb >= 1) return `${gb.toFixed(gb >= 10 ? 0 : 1)} GB`;
  return `${Math.round(bytes / 1024 ** 2)} MB`;
}

// stateClass maps a real VM state to the three visual buckets the list styles.
function stateClass(state: string): 'running' | 'booting' | 'stopped' {
  const s = state.toLowerCase();
  if (s === 'running') return 'running';
  if (s === 'booting') return 'booting';
  return 'stopped';
}

export function MachinesView() {
  const { t, tf, ts } = useT();
  const health = useHealth();
  const { state: vmState, discovery, vms, error: vmError, refresh } = useVmStatus();
  const [loadingActions, setLoadingActions] = useState<
    Record<string, Partial<Record<VmAction, boolean>>>
  >({});
  const [actionError, setActionError] = useState<string | null>(null);
  const [localStateStatus, setLocalStateStatus] = useState<LocalStateStatusResponse | undefined>();
  const [consoleVm, setConsoleVm] = useState<{ id: string; name: string } | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [wizardOpen, setWizardOpen] = useState(false);
  const [telemetry, setTelemetry] = useState<VmTelemetryResponse | null>(null);
  const [gaStatus, setGaStatus] = useState<Record<string, GuestAdditionsStatusResponse>>({});
  const [gaBusy, setGaBusy] = useState<Record<string, boolean>>({});
  const [gaInserted, setGaInserted] = useState<Record<string, boolean>>({});
  // Guest Additions update-via-guest-control modal: which VM, the one-time
  // credentials (never persisted), and the in-flight/result state.
  const [gaUpdateVm, setGaUpdateVm] = useState<{ id: string; name: string } | null>(null);
  const [gaUser, setGaUser] = useState('');
  const [gaPass, setGaPass] = useState('');
  const [gaUpdateBusy, setGaUpdateBusy] = useState(false);
  const [gaUpdateResult, setGaUpdateResult] = useState<GuestAdditionsUpdateResponse | null>(null);
  // Which VMs are Linux guests (so the serial-terminal button is offered). The
  // ostype is VBox metadata, readable regardless of power state, fetched once.
  const [termCapable, setTermCapable] = useState<Record<string, boolean>>({});
  const termFetchedRef = useRef<Set<string>>(new Set());

  const agentOnline = health.state === 'success' && health.data?.status === 'healthy';

  // The focus panel manages one machine at a time: the one whose console is open,
  // otherwise the VM the user clicked in the list, otherwise the first running VM.
  // A selected VM is focused regardless of state — Files (persistent shares),
  // Network (modifyvm), and Snapshots all operate on a powered-off VM, so the
  // panel must open for stopped machines too. Only the live console + telemetry
  // block is gated on the VM actually running (see focusRunning below).
  const focusVm = useMemo<VmInfo | undefined>(() => {
    if (consoleVm) {
      const match = vms.find((vm) => vm.id === consoleVm.id);
      if (match) return match;
    }
    if (selectedId) {
      const match = vms.find((vm) => vm.id === selectedId);
      if (match) return match;
    }
    return vms.find((vm) => vm.state.toLowerCase() === 'running');
  }, [vms, consoleVm, selectedId]);

  const focusRunning = focusVm ? focusVm.state.toLowerCase() === 'running' : false;
  const focusLoading = focusVm ? (loadingActions[focusVm.id] ?? {}) : {};
  const focusBusy = Object.values(focusLoading).some(Boolean);

  function openConsole(id: string) {
    const vm = vms.find((candidate) => candidate.id === id);
    setSelectedId(id);
    setConsoleVm({ id, name: vm?.name ?? id });
  }

  // openConsoleTab opens the console full-screen in a fresh browser tab. The new
  // tab loads the same app at ?console=<id>, which renders only the console.
  function openConsoleTab(id: string, name: string) {
    const url = `${window.location.pathname}?console=${encodeURIComponent(id)}&name=${encodeURIComponent(name)}`;
    window.open(url, '_blank', 'noopener');
  }

  // openTerminalTab opens the serial terminal full-screen in a fresh browser tab
  // at ?terminal=<id>, mirroring openConsoleTab.
  function openTerminalTab(id: string, name: string) {
    const url = `${window.location.pathname}?terminal=${encodeURIComponent(id)}&name=${encodeURIComponent(name)}`;
    // No 'noopener' here: the terminal tab's close button uses window.close(),
    // which the browser only honors for a script-opened, script-reachable tab.
    window.open(url, '_blank');
  }

  useEffect(() => {
    let cancelled = false;
    async function loadLocalState() {
      try {
        const response = await api.getLocalStateStatus();
        if (!cancelled) setLocalStateStatus(response);
      } catch {
        if (!cancelled) setLocalStateStatus(undefined);
      }
    }
    void loadLocalState();
    return () => {
      cancelled = true;
    };
  }, []);

  // Probe each VM's guest OS once so the row can offer the serial-terminal button
  // only for Linux guests. ostype is metadata, so this works for stopped VMs too.
  useEffect(() => {
    let cancelled = false;
    for (const vm of vms) {
      if (termFetchedRef.current.has(vm.id)) continue;
      termFetchedRef.current.add(vm.id);
      api
        .getVmGuestOS(vm.id)
        .then((res) => {
          if (!cancelled) setTermCapable((current) => ({ ...current, [vm.id]: res.terminalCapable }));
        })
        .catch(() => termFetchedRef.current.delete(vm.id));
    }
    return () => {
      cancelled = true;
    };
  }, [vms]);

  // Telemetry for the focused running machine. Best-effort; failures leave the
  // panel empty rather than surfacing an error. Only a running VM reports live
  // session telemetry, so a stopped focus skips the fetch instead of erroring.
  useEffect(() => {
    if (!focusVm || focusVm.state.toLowerCase() !== 'running') {
      setTelemetry(null);
      return;
    }
    let cancelled = false;
    api
      .getVmTelemetry(focusVm.id)
      .then((t) => {
        if (!cancelled) setTelemetry(t);
      })
      .catch(() => {
        if (!cancelled) setTelemetry(null);
      });
    return () => {
      cancelled = true;
    };
  }, [focusVm]);

  // Guest Additions is only detectable while a VM is running (the version guest
  // property is populated by the running additions service). Probe each running
  // VM so the row can offer the disc-insert shortcut when it is missing.
  const runningKey = vms
    .filter((vm) => vm.state.toLowerCase() === 'running')
    .map((vm) => vm.id)
    .join(',');
  useEffect(() => {
    if (runningKey === '') return;
    let cancelled = false;
    for (const id of runningKey.split(',')) {
      api
        .getGuestAdditionsStatus(id)
        .then((status) => {
          if (!cancelled) setGaStatus((current) => ({ ...current, [id]: status }));
        })
        .catch(() => {
          // best-effort; a probe failure just hides the shortcut for this VM
        });
    }
    return () => {
      cancelled = true;
    };
  }, [runningKey]);

  const installGuestAdditions = useCallback(async (id: string) => {
    setGaBusy((current) => ({ ...current, [id]: true }));
    setActionError(null);
    try {
      await api.installGuestAdditions(id);
      setGaInserted((current) => ({ ...current, [id]: true }));
    } catch (error: unknown) {
      let message = error instanceof Error ? error.message : String(error);
      if (error instanceof ApiError && error.body.trim() !== '') message = error.body.trim();
      setActionError(message);
    } finally {
      setGaBusy((current) => ({ ...current, [id]: false }));
    }
  }, []);

  const openGaUpdate = useCallback((id: string, name: string) => {
    setGaUpdateVm({ id, name });
    setGaUser('');
    setGaPass('');
    setGaUpdateResult(null);
  }, []);

  const closeGaUpdate = useCallback(() => {
    setGaUpdateVm(null);
    setGaUser('');
    setGaPass(''); // drop the password from memory
    setGaUpdateResult(null);
  }, []);

  const submitGaUpdate = useCallback(async () => {
    if (!gaUpdateVm) return;
    setGaUpdateBusy(true);
    setGaUpdateResult(null);
    try {
      const res = await api.updateGuestAdditions(gaUpdateVm.id, gaUser, gaPass);
      setGaUpdateResult(res);
    } catch (error: unknown) {
      let message = error instanceof Error ? error.message : String(error);
      if (error instanceof ApiError && error.body.trim() !== '') message = error.body.trim();
      setGaUpdateResult({ success: false, vmId: gaUpdateVm.id, message });
    } finally {
      setGaUpdateBusy(false);
      setGaPass(''); // never keep the password around after the call
    }
  }, [gaUpdateVm, gaUser, gaPass]);

  const runAction = useCallback(
    async (id: string, action: VmAction) => {
      setLoadingActions((current) => ({ ...current, [id]: { ...current[id], [action]: true } }));
      setActionError(null);
      try {
        if (action === 'start') await api.startVm(id);
        else if (action === 'stop') await api.stopVm(id);
        else if (action === 'reset') await api.resetVm(id);
        else if (action === 'delete') await api.deleteVm(id);

        if (lifecycleActions.includes(action)) await refresh();
      } catch (error: unknown) {
        let message = error instanceof Error ? error.message : String(error);
        if (error instanceof ApiError && error.body.trim() !== '') message = error.body.trim();
        setActionError(message);
      } finally {
        setLoadingActions((current) => ({ ...current, [id]: { ...current[id], [action]: false } }));
      }
    },
    [refresh],
  );

  function handleReset(id: string, name: string) {
    if (
      window.confirm(
        tf(
          'Reset will forcibly restart "{name}" and may cause data loss. This is not a graceful shutdown. Are you sure you want to continue?',
          { name },
        ),
      )
    ) {
      void runAction(id, 'reset');
    }
  }

  function handleDelete(id: string, name: string) {
    if (
      window.confirm(
        tf(
          'Delete "{name}" permanently? Its disks and configuration files will be removed from this computer. This cannot be undone.',
          { name },
        ),
      )
    ) {
      void runAction(id, 'delete');
    }
  }

  const vboxVersion = vmState === 'success' && discovery?.found ? discovery.version : undefined;

  return (
    <>
      <div className="tv-page-head tv-rise d1">
        <h1>{t('Virtual machines')}</h1>
        <p>{t('Local VirtualBox machines, controlled from the browser like tabs.')}</p>
        <div className="tv-meta">
          <span className="kv">
            TabVM <b>v{__TABVM_VERSION__}</b>
          </span>
          {!agentOnline && (
            <>
              <span className="sep" />
              <span className="kv warn">
                <span className="tv-dot off" />
                {t('agent offline')}
              </span>
            </>
          )}
          {vboxVersion && (
            <>
              <span className="sep" />
              <span className="kv">
                vboxmanage <b>{vboxVersion}</b>
              </span>
            </>
          )}
          {localStateStatus?.available && (
            <>
              <span className="sep" />
              <span className="kv">
                {t('local state')} <b>{t('schema')} {localStateStatus.schema}</b>
              </span>
            </>
          )}
          {typeof health.data?.uptimeSeconds === 'number' && (
            <>
              <span className="sep" />
              <span className="kv">
                {t('uptime')} <b>{formatUptime(health.data.uptimeSeconds)}</b>
              </span>
            </>
          )}
        </div>
      </div>

      <section className="tv-sec tv-rise d2">
        <div className="tv-sec-top">
          <h2>{t('Virtual machines')}</h2>
          <span className="count">{vms.length}</span>
          <div className="grow">
            <button type="button" className="tv-btn" onClick={() => setWizardOpen(true)}>
              <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 5v14M5 12h14" />
              </svg>
              {t('New VM')}
            </button>
            <button type="button" className="tv-btn quiet" onClick={() => void refresh()}>
              <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M3 12a9 9 0 1 0 3-6.7L3 8" />
                <path d="M3 3v5h5" />
              </svg>
              {t('Refresh')}
            </button>
          </div>
        </div>

        {vmState === 'loading' && <div className="tv-empty">{t('Discovering VirtualBox…')}</div>}
        {vmState === 'error' && <div className="tv-error">{ts(vmError ?? '')}</div>}
        {actionError && <div className="tv-error">{ts(actionError)}</div>}

        {vmState === 'success' && (
          <div className="tv-list">
            {vms.length === 0 && <div className="tv-empty">{t('No virtual machines found.')}</div>}
            {vms.map((vm) => {
              const cls = stateClass(vm.state);
              const loading = loadingActions[vm.id] ?? {};
              const busy = Object.values(loading).some(Boolean);
              return (
                <div
                  className={`tv-vm ${cls} ${focusVm?.id === vm.id ? 'is-focused' : ''}`}
                  key={vm.id}
                  onClick={() => setSelectedId(vm.id)}
                >
                  <div className="tv-vm-ico">
                    <BrandMark muted={cls === 'stopped'} />
                  </div>
                  <div className="tv-vm-meta">
                    <div className="tv-vm-name">
                      <h3>{vm.name}</h3>
                      <span className="tv-state">
                        <span className="d" />
                        {t(vm.state)}
                      </span>
                    </div>
                    <div className="tv-vm-sub">
                      <span className="id">{vm.id}</span>
                    </div>
                  </div>
                  <div className="tv-vm-actions">
                    {cls === 'running' && (
                      <>
                        {gaStatus[vm.id]?.status !== 'installed' && !gaInserted[vm.id] && (
                          <button
                            type="button"
                            className="tv-abtn ga"
                            aria-label={tf('Install Guest Additions on {vm}', { vm: vm.name })}
                            title={t('Install Guest Additions')}
                            disabled={gaBusy[vm.id]}
                            onClick={() => void installGuestAdditions(vm.id)}
                          >
                            <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M12 3v12" />
                              <path d="M7 10l5 5 5-5" />
                              <path d="M5 21h14" />
                            </svg>
                            {gaBusy[vm.id] ? t('inserting…') : t('Install Guest Additions')}
                          </button>
                        )}
                        {gaInserted[vm.id] && (
                          <span className="tv-ga-done" title={t('disc inserted · run installer in VM')}>
                            {t('disc inserted · run installer in VM')}
                          </span>
                        )}
                        {gaStatus[vm.id]?.updateAvailable && (
                          <button
                            type="button"
                            className="tv-abtn ga"
                            aria-label={tf('Update Guest Additions on {vm}', { vm: vm.name })}
                            title={`Guest Additions ${gaStatus[vm.id]?.version ?? ''} → ${gaStatus[vm.id]?.hostVersion ?? ''}`}
                            onClick={() => openGaUpdate(vm.id, vm.name)}
                          >
                            <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M21 12a9 9 0 1 1-3-6.7L21 8" />
                              <path d="M21 3v5h-5" />
                            </svg>
                            {t('Update Guest Additions')}
                          </button>
                        )}
                        <div className="tv-quiet">
                          <button
                            type="button"
                            className="tv-abtn"
                            aria-label={tf('Stop {vm}', { vm: vm.name })}
                            disabled={busy}
                            onClick={() => void runAction(vm.id, 'stop')}
                          >
                            {loading.stop ? t('stopping…') : t('stop')}
                          </button>
                          <button
                            type="button"
                            className="tv-abtn danger"
                            aria-label={tf('Reset {vm}', { vm: vm.name })}
                            disabled={busy}
                            onClick={() => handleReset(vm.id, vm.name)}
                          >
                            {t('reset')}
                          </button>
                        </div>
                        <button
                          type="button"
                          className="tv-abtn"
                          aria-label={tf('Open {vm} console in a new tab', { vm: vm.name })}
                          title={t('new tab')}
                          onClick={() => openConsoleTab(vm.id, vm.name)}
                        >
                          <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M15 3h6v6" />
                            <path d="M10 14L21 3" />
                            <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                          </svg>
                          {t('new tab')}
                        </button>
                        {termCapable[vm.id] && (
                          <button
                            type="button"
                            className="tv-abtn"
                            aria-label={tf('Open {vm} terminal in a new tab', { vm: vm.name })}
                            title={t('terminal')}
                            onClick={() => openTerminalTab(vm.id, vm.name)}
                          >
                            <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M4 17l6-6-6-6" />
                              <path d="M12 19h8" />
                            </svg>
                            {t('terminal')}
                          </button>
                        )}
                        <button
                          type="button"
                          className="tv-abtn go"
                          aria-label={tf('Open console for {vm}', { vm: vm.name })}
                          onClick={() => openConsole(vm.id)}
                        >
                          <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <rect x="3" y="4" width="18" height="16" rx="2" />
                            <path d="M7 9l3 3-3 3" />
                            <path d="M13 15h4" />
                          </svg>
                          {t('open console')}
                        </button>
                      </>
                    )}
                    {cls === 'booting' && (
                      <button type="button" className="tv-abtn" disabled aria-label={tf('{vm} is starting', { vm: vm.name })}>
                        {t('starting…')}
                      </button>
                    )}
                    {cls === 'stopped' && (
                      <>
                        <button
                          type="button"
                          className="tv-abtn danger"
                          aria-label={tf('Delete {vm}', { vm: vm.name })}
                          title={t('Delete VM')}
                          disabled={busy}
                          onClick={() => handleDelete(vm.id, vm.name)}
                        >
                          <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M3 6h18" />
                            <path d="M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2" />
                            <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
                            <path d="M10 11v6M14 11v6" />
                          </svg>
                          {loading.delete ? t('deleting…') : t('delete')}
                        </button>
                        {termCapable[vm.id] && (
                          <button
                            type="button"
                            className="tv-abtn"
                            aria-label={tf('Open {vm} terminal in a new tab', { vm: vm.name })}
                            title={t('terminal')}
                            onClick={() => openTerminalTab(vm.id, vm.name)}
                          >
                            <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M4 17l6-6-6-6" />
                              <path d="M12 19h8" />
                            </svg>
                            {t('terminal')}
                          </button>
                        )}
                        <button
                          type="button"
                          className="tv-abtn go"
                          aria-label={tf('Start {vm}', { vm: vm.name })}
                          disabled={busy}
                          onClick={() => void runAction(vm.id, 'start')}
                        >
                          {loading.start ? t('starting…') : t('start')}
                        </button>
                      </>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      {focusVm && (
        <section className="tv-sec tv-rise d3">
          <div className="tv-sec-top">
            <h2>{focusRunning ? t('Live console') : t('Machine')}</h2>
          </div>
          <div className="tv-grid">
            <div className="tv-window">
              <div className="tv-winbar">
                <span className="dots">
                  <i />
                  <i />
                  <i />
                </span>
                <span className="title">
                  <BrandMark className="tv-winmark" />
                  {focusVm.name}
                </span>
              </div>
              <div className="tv-screen tv-screen--live">
                {!focusRunning ? (
                  <div className="tv-screen-off">
                    <span className="tv-off-note">{t('This machine is powered off.')}</span>
                    <button
                      type="button"
                      className="tv-abtn go"
                      disabled={focusBusy}
                      onClick={() => void runAction(focusVm.id, 'start')}
                    >
                      {focusLoading.start ? t('starting…') : t('start')}
                    </button>
                  </div>
                ) : consoleVm ? (
                  <div className="tv-attach">
                    <span className="tv-attach-sub">{t('console attached')}</span>
                  </div>
                ) : (
                  <ConsolePreview vmId={focusVm.id} onOpen={() => openConsole(focusVm.id)} />
                )}
              </div>
            </div>

            <div className="tv-tele">
              <div>
                <div className="lab">{t('Configured')}</div>
                <div className="big">
                  {telemetry?.cpuCount ?? '—'}
                  <small>vCPU</small>
                </div>
              </div>
              <div>
                <div className="lab">{t('Session')}</div>
                <div className="tv-kv">
                  <span className="k">{t('Memory')}</span>
                  <span className="v">{telemetry ? formatRam(telemetry.ramMb) : '—'}</span>
                </div>
                <div className="tv-kv">
                  <span className="k">{t('Disk')}</span>
                  <span className="v">
                    {telemetry && telemetry.disks.length > 0
                      ? `${formatBytes(telemetry.disks[0].allocatedBytes)} / ${formatBytes(telemetry.disks[0].capacityBytes)}`
                      : '—'}
                  </span>
                </div>
                <div className="tv-kv">
                  <span className="k">{t('Network')}</span>
                  <span className="v">{networkSummary(telemetry)}</span>
                </div>
                <div className="tv-kv">
                  <span className="k">Guest Additions</span>
                  <span className="v">{telemetry ? (telemetry.guestAdditions ? t('active') : t('not detected')) : '—'}</span>
                </div>
              </div>
            </div>
          </div>

          <HardwarePanel vmId={focusVm.id} onChanged={() => void refresh()} />
          <StoragePanel vmId={focusVm.id} onChanged={() => void refresh()} />
          <FilesPanel vmId={focusVm.id} vmName={focusVm.name} />
          <NetworkPanel vmId={focusVm.id} onChanged={() => void refresh()} />
          <SnapshotsPanel vmId={focusVm.id} vmName={focusVm.name} onChanged={() => void refresh()} />
        </section>
      )}

      {consoleVm && (
        <ScreenConsole vmId={consoleVm.id} vmName={consoleVm.name} onClose={() => setConsoleVm(null)} />
      )}

      {wizardOpen && (
        <CreateVmWizard
          onClose={() => setWizardOpen(false)}
          onCreated={() => void refresh()}
        />
      )}

      {gaUpdateVm && (
        <div className="ga-overlay" role="dialog" aria-label={`${t('Update Guest Additions')} · ${gaUpdateVm.name}`}>
          <div className="ga-modal">
            <div className="ga-modal-head">
              <h3>{t('Update Guest Additions')}</h3>
              <button type="button" className="ga-x" aria-label={t('close')} onClick={closeGaUpdate}>
                ×
              </button>
            </div>
            <p className="ga-modal-note">
              {t(
                'Runs the installer inside the guest over VirtualBox guest control. Requires a running Linux guest with Guest Additions already active. Use root, or a user with sudo — credentials are used once and never stored.',
              )}
            </p>
            <label className="ga-field">
              <span>{t('Guest username')}</span>
              <input
                type="text"
                autoComplete="off"
                value={gaUser}
                placeholder="root"
                disabled={gaUpdateBusy}
                onChange={(e) => setGaUser(e.target.value)}
              />
            </label>
            <label className="ga-field">
              <span>{t('Guest password')}</span>
              <input
                type="password"
                autoComplete="off"
                value={gaPass}
                disabled={gaUpdateBusy}
                onChange={(e) => setGaPass(e.target.value)}
              />
            </label>
            {gaUpdateResult && (
              <div className={`ga-result ${gaUpdateResult.success ? 'ok' : 'err'}`}>
                <div className="ga-result-msg">{ts(gaUpdateResult.message)}</div>
                {gaUpdateResult.output && <pre className="ga-result-log">{gaUpdateResult.output}</pre>}
              </div>
            )}
            <div className="ga-modal-actions">
              <button type="button" className="tv-abtn" onClick={closeGaUpdate} disabled={gaUpdateBusy}>
                {gaUpdateResult?.success ? t('close') : t('cancel')}
              </button>
              <button
                type="button"
                className="tv-abtn go"
                onClick={() => void submitGaUpdate()}
                disabled={gaUpdateBusy || gaUser.trim() === '' || gaPass === ''}
              >
                {gaUpdateBusy ? t('updating…') : t('Update')}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

// networkSummary renders the first interface that has a guest IP, or its mode
// when no IP is reported yet.
function networkSummary(telemetry: VmTelemetryResponse | null): string {
  if (!telemetry || telemetry.networks.length === 0) return '—';
  const withIp = telemetry.networks.find((nic) => nic.ipv4.length > 0);
  if (withIp) return `${withIp.mode} · ${withIp.ipv4[0]}`;
  return telemetry.networks[0].mode;
}

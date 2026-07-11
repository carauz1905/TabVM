import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
import type { VirtualBoxDiscovery, VmInfo } from '../types/api';

type LoadingState = 'loading' | 'success' | 'error';

interface UseVmStatusState {
  state: LoadingState;
  discovery?: VirtualBoxDiscovery;
  vms: VmInfo[];
  error?: string;
}

export interface UseVmStatusResult extends UseVmStatusState {
  refresh: () => Promise<void>;
}

export function useVmStatus(): UseVmStatusResult {
  const [state, setState] = useState<UseVmStatusState>({
    state: 'loading',
    vms: [],
  });
  const [refreshKey, setRefreshKey] = useState(0);
  const loadPromiseRef = useRef<Promise<void> | null>(null);
  const resolveLoadRef = useRef<(() => void) | null>(null);

  const refresh = useCallback((): Promise<void> => {
    setRefreshKey((current) => current + 1);

    if (!loadPromiseRef.current) {
      loadPromiseRef.current = new Promise<void>((resolve) => {
        resolveLoadRef.current = resolve;
      });
    }

    return loadPromiseRef.current;
  }, []);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      setState((current) => ({ ...current, state: 'loading' }));

      try {
        const discovery = await api.getVirtualBoxDiscovery();

        if (!discovery.found) {
          if (!cancelled) {
            setState({ state: 'success', discovery, vms: [] });
          }
          return;
        }

        const vmList = await api.getVms();

        if (!cancelled) {
          setState({ state: 'success', discovery, vms: vmList.vms });
        }
      } catch (error: unknown) {
        if (!cancelled) {
          const message = error instanceof Error ? error.message : String(error);
          setState({ state: 'error', vms: [], error: message });
        }
      } finally {
        if (resolveLoadRef.current) {
          resolveLoadRef.current();
          resolveLoadRef.current = null;
          loadPromiseRef.current = null;
        }
      }
    }

    load();

    return () => {
      cancelled = true;
    };
  }, [refreshKey]);

  return { ...state, refresh };
}

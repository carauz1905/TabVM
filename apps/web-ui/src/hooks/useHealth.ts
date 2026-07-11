import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { HealthStatus } from '../types/api';

type LoadingState = 'loading' | 'success' | 'error';

interface UseHealthResult {
  state: LoadingState;
  data?: HealthStatus;
  error?: string;
}

export function useHealth(): UseHealthResult {
  const [result, setResult] = useState<UseHealthResult>({ state: 'loading' });

  useEffect(() => {
    let cancelled = false;

    api
      .getHealth()
      .then((data) => {
        if (!cancelled) {
          setResult({ state: 'success', data });
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          const message = error instanceof Error ? error.message : String(error);
          setResult({ state: 'error', error: message });
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return result;
}

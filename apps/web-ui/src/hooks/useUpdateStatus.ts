import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { UpdateStatus } from '../types/api';

// useUpdateStatus fetches the best-effort "update available" status from the
// agent, mirroring useHealth's loading/success shape but flattened to the
// resolved status (a failed or pending check simply reads as "no update", which
// keeps the banner hidden).
//
// Privacy / identity: TabVM is local-first. The underlying request triggers the
// agent's ONLY outbound network call — an unauthenticated GET to GitHub's public
// releases API, cached (6h) and silent-fail offline. No telemetry. A user who
// wants zero outbound traffic can opt out by setting
// localStorage['tabvm.updateCheck']='off', in which case we never fetch.
const NO_UPDATE: UpdateStatus = { current: '', updateAvailable: false };

function checkDisabled(): boolean {
  try {
    return localStorage.getItem('tabvm.updateCheck') === 'off';
  } catch {
    // localStorage may be unavailable; default to allowing the check.
    return false;
  }
}

export function useUpdateStatus(): UpdateStatus {
  const [status, setStatus] = useState<UpdateStatus>(NO_UPDATE);

  useEffect(() => {
    // Honor the opt-out: skip the only outbound call entirely.
    if (checkDisabled()) return;

    let cancelled = false;
    api
      .getUpdateStatus()
      .then((data) => {
        if (!cancelled) setStatus(data);
      })
      .catch(() => {
        // Best-effort and silent: any failure leaves the banner hidden.
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return status;
}

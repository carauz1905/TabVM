import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { es } from './es';
import { LanguageProvider } from './i18n';
import { useDocs } from '../components/docs/content';

// Spot checks for the app-wide Spanish register: neutral-professional "usted",
// never "tú". Full coverage is enforced by review sweeps; these assertions pin
// the known offenders so the register cannot silently regress.
describe('Spanish register (usted)', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('uses usted imperatives in the USB controller notice', () => {
    const value =
      es[
        'This VM has no USB controller enabled. Power the VM off and enable USB in its settings — the controller cannot be turned on while the VM is running.'
      ];
    expect(value.startsWith('Esta VM no tiene un controlador USB habilitado. Apague la VM y habilite USB')).toBe(
      true,
    );
  });

  it('uses usted imperatives in the Extension Pack notice', () => {
    const value =
      es['USB 2.0 and 3.0 passthrough needs the Oracle VirtualBox Extension Pack. Install it to attach most devices.'];
    expect(value).toContain('Instálelo');
  });

  it('uses usted in the Spanish manual bodies', () => {
    localStorage.setItem('tabvm.lang', 'es');

    const { result } = renderHook(() => useDocs(), {
      wrapper: ({ children }: { children: ReactNode }) => <LanguageProvider>{children}</LanguageProvider>,
    });

    expect(result.current.quickStart.steps[0].title).toBe('Instale y abra');
    expect(result.current.quickStart.steps[0].body).toContain('Ejecute el instalador de TabVM');
    expect(result.current.welcome.lead).toContain('sus máquinas locales');
    // The brand slogan is part of the brand and is never translated.
    expect(result.current.tagline).toBe('every VM. one tab.');
  });
});

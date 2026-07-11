import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { es, esServerExact, esServerPatterns } from './es';

export type Lang = 'en' | 'es';

const STORAGE_KEY = 'tabvm.lang';

// initialLang reads the persisted language. English is the default so any string
// not present in the Spanish dictionary falls back to its English source.
export function initialLang(): Lang {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === 'en' || stored === 'es') return stored;
  } catch {
    // localStorage may be unavailable.
  }
  return 'en';
}

interface LangContextValue {
  lang: Lang;
  setLang: (lang: Lang) => void;
  toggle: () => void;
}

const LangContext = createContext<LangContextValue>({
  lang: 'en',
  setLang: () => {},
  toggle: () => {},
});

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(initialLang);

  // Keep <html lang> in sync for accessibility and correct hyphenation.
  useEffect(() => {
    document.documentElement.setAttribute('lang', lang);
  }, [lang]);

  const setLang = useCallback((next: Lang) => {
    setLangState(next);
    document.documentElement.setAttribute('lang', next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // In-session change still applies even if it cannot be persisted.
    }
  }, []);

  const toggle = useCallback(() => setLangState((prev) => {
    const next: Lang = prev === 'en' ? 'es' : 'en';
    document.documentElement.setAttribute('lang', next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // ignore
    }
    return next;
  }), []);

  const value = useMemo(() => ({ lang, setLang, toggle }), [lang, setLang, toggle]);
  return <LangContext.Provider value={value}>{children}</LangContext.Provider>;
}

export function useLang(): LangContextValue {
  return useContext(LangContext);
}

// localizeServer translates a backend-produced message (error or notification).
// Backend strings stay English at the source; this maps the known set to Spanish
// by exact match first, then by pattern for messages carrying names/paths. An
// unmapped message falls back to its English text rather than breaking.
export function localizeServer(message: string, lang: Lang): string {
  if (lang !== 'es' || !message) return message;
  const trimmed = message.trim();
  if (esServerExact[trimmed]) return esServerExact[trimmed];
  for (const [pattern, template] of esServerPatterns) {
    const match = trimmed.match(pattern);
    if (match) {
      return template.replace(/\$(\d+)/g, (_, n) => match[Number(n)] ?? '');
    }
  }
  // Fall back to the UI dictionary so UI-set status strings localize too.
  if (es[trimmed]) return es[trimmed];
  return message;
}

// useT exposes translation helpers bound to the current language:
//   t(en)          -> exact UI string lookup, English fallback
//   tf(en, vars)   -> same, replacing {name} placeholders
//   ts(serverMsg)  -> localize a backend error/notification
export function useT() {
  const { lang } = useLang();

  const t = useCallback((en: string): string => (lang === 'es' ? es[en] ?? en : en), [lang]);

  const tf = useCallback(
    (en: string, vars: Record<string, string | number>): string => {
      let out = lang === 'es' ? es[en] ?? en : en;
      for (const key of Object.keys(vars)) {
        out = out.replace(new RegExp(`\\{${key}\\}`, 'g'), String(vars[key]));
      }
      return out;
    },
    [lang],
  );

  const ts = useCallback((message: string): string => localizeServer(message, lang), [lang]);

  return { t, tf, ts, lang };
}

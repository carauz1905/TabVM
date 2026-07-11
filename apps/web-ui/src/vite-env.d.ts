/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_TABVM_SESSION_TOKEN?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

// Injected by Vite (define) from package.json version.
declare const __TABVM_VERSION__: string;

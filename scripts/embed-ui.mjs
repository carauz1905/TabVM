// Copy the freshly built web UI into the desktop agent's go:embed directory.
//
// The embedded dist (apps/desktop-agent/internal/webui/dist) is a GENERATED
// artifact: it is git-ignored and required at compile time by `//go:embed`.
// Run `npm run build` (which invokes this after building the web UI) to
// regenerate it on a fresh clone before `go build`/`go test`.
import { rmSync, cpSync, existsSync } from 'node:fs';

const src = 'apps/web-ui/dist';
const dest = 'apps/desktop-agent/internal/webui/dist';

if (!existsSync(src)) {
  console.error(`Missing ${src}. Run "npm run webui:build" first.`);
  process.exit(1);
}

rmSync(dest, { recursive: true, force: true });
cpSync(src, dest, { recursive: true });
console.log(`Embedded UI: ${src} -> ${dest}`);

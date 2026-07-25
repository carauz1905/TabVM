// TabVM's version lives in three files that must agree. Releases are cut by
// hand (see RELEASING.md), so nothing but this check stops them drifting -- and
// they did drift: the root manifest sat at 0.1.1 while the agent and the web UI
// both shipped 0.4.0.
//
// Run from the repository root: node scripts/check-versions.mjs
import { readFileSync } from 'node:fs';

const AGENT_VERSION_FILE = 'apps/desktop-agent/internal/version/version.go';

const sources = [
  {
    label: 'package.json',
    read: () => JSON.parse(readFileSync('package.json', 'utf8')).version,
  },
  {
    label: 'apps/web-ui/package.json',
    read: () => JSON.parse(readFileSync('apps/web-ui/package.json', 'utf8')).version,
  },
  {
    label: AGENT_VERSION_FILE,
    read: () => {
      const source = readFileSync(AGENT_VERSION_FILE, 'utf8');
      const match = source.match(/const Version = "([^"]+)"/);
      if (!match) {
        throw new Error(`could not find 'const Version' in ${AGENT_VERSION_FILE}`);
      }
      return match[1];
    },
  },
];

let found;
try {
  found = sources.map(({ label, read }) => ({ label, version: read() }));
} catch (error) {
  console.error(`Could not read a version: ${error.message}`);
  process.exit(1);
}

const distinct = new Set(found.map((entry) => entry.version));

if (distinct.size > 1) {
  console.error('Version manifests disagree:\n');
  for (const { label, version } of found) {
    console.error(`  ${version.padEnd(12)} ${label}`);
  }
  console.error('\nAll three must match before a release. See RELEASING.md step 1.');
  process.exit(1);
}

console.log(`Version manifests agree: ${[...distinct][0]}`);

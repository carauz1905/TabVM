# Releasing TabVM

This is the runbook for cutting a TabVM release. It documents the exact,
repeatable steps; there is **no tag-triggered CI**, so publication is manual.

TabVM follows [Semantic Versioning](https://semver.org): in `0.x`, a **patch**
(`0.1.1 → 0.1.2`) is bug fixes only, a **minor** (`0.1.x → 0.2.0`) adds features.
Ship bug fixes as prompt patch releases; batch features into the next minor.

## Prerequisites

- Go and Node installed; `npm ci` works from the repo root.
- [Inno Setup 6](https://jrsoftware.org/isinfo.php) (optional) for the installer.
  Without it, `build-release.ps1` still produces the portable ZIP.
- `gh` authenticated with push + release access to `carauz1905/TabVM`.
- On `main`, clean tree, all release PRs merged, CI green.

## 1. Bump the version (two files, kept in sync)

The version lives in two places that MUST match:

- `apps/desktop-agent/internal/version/version.go` — `const Version = "X.Y.Z"`
- `apps/web-ui/package.json` — `"version": "X.Y.Z"`

## 2. Update the changelog

In `CHANGELOG.md`:

- Move the `## [Unreleased]` entries under a new `## [X.Y.Z] - YYYY-MM-DD` heading.
- Update the link references at the bottom:
  - `[Unreleased]: .../compare/vX.Y.Z...HEAD`
  - add `[X.Y.Z]: .../compare/v<prev>...vX.Y.Z`

## 3. Commit and tag

```bash
git add apps/desktop-agent/internal/version/version.go apps/web-ui/package.json CHANGELOG.md
git commit -m "chore(release): vX.Y.Z"
git tag -a vX.Y.Z -m "TabVM vX.Y.Z — <one-line summary>"
git push origin main
git push origin vX.Y.Z
```

## 4. Build the artifacts

```bash
pwsh scripts/build-release.ps1          # or: powershell.exe -File scripts/build-release.ps1
# --SkipInstaller to build the ZIP only
```

Produces:

- `dist/TabVM-portable.zip`
- `dist/TabVM-Setup.exe` (only if Inno Setup is present)

The script builds the web UI, embeds it (`go:embed`), and builds the windowed
`tabvm-agent.exe` + `TabVM.exe` with `CGO_ENABLED=0`.

## 5. Checksums

Use `sha256sum` (git-bash). **Do not rely on `Get-FileHash`** — it is missing in
some PowerShell contexts on this setup.

```bash
cd dist
sha256sum TabVM-portable.zip | cut -d' ' -f1 > TabVM-portable.zip.sha256
sha256sum TabVM-Setup.exe    | cut -d' ' -f1 > TabVM-Setup.exe.sha256
```

The scoop manifest's `autoupdate` block reads `$url.sha256`, so the
`.zip.sha256` sidecar MUST be published with the release.

## 6. Publish the GitHub Release

```bash
gh release create vX.Y.Z \
  --title "TabVM vX.Y.Z" \
  --notes-file <notes.md> \
  --verify-tag \
  dist/TabVM-portable.zip dist/TabVM-portable.zip.sha256 \
  dist/TabVM-Setup.exe    dist/TabVM-Setup.exe.sha256
```

Notes are the `### Fixed` / `### Added` bullets from the changelog.

## 7. Update the scoop manifest

Edit `bucket/tabvm.json`:

- `version` → `X.Y.Z`
- `url` → `.../releases/download/vX.Y.Z/TabVM-portable.zip`
- `hash` → the ZIP's SHA-256 (from step 5)

Verify the manifest hash matches the published sidecar before committing:

```bash
PUB=$(curl -sL ".../releases/download/vX.Y.Z/TabVM-portable.zip.sha256" | tr -d '\r\n ')
MAN=$(grep -oE '"hash": "[a-f0-9]{64}"' bucket/tabvm.json | grep -oE '[a-f0-9]{64}')
[ "$PUB" = "$MAN" ] && echo "MATCH" || echo "MISMATCH"
```

Then:

```bash
git add bucket/tabvm.json
git commit -m "chore(scoop): point manifest at vX.Y.Z release"
git push origin main
```

## Notes

- **Scoop users** update with `scoop update tabvm`. The `checkver` / `autoupdate`
  blocks in the manifest are for regenerating the manifest, not for notifying the
  app's users.
- **No tag CI**: pushing `vX.Y.Z` triggers nothing automated; steps 4–7 are manual.
  If this becomes frequent, add a `release.yml` workflow triggered on `push: tags`
  that runs the build and uploads the assets.

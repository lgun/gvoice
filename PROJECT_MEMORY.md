# guvoice Project Memory

Last updated: 2026-05-28

## Product Direction

guvoice / 구보이스 is a Go + Wails desktop app for making playful Korean sample-based voices. The target feeling is similar to Animal Crossing style speech, but the app should avoid positioning itself as a full Korean TTS engine.

The core loop is:

1. Create or select a voice source.
2. Fill the source with required samples through direct recording or upload.
3. Type text in the Speak screen.
4. Generate a preview only when the selected source is sufficiently filled.
5. Export the result, ultimately as MP3.

Direct recording is the primary workflow. Upload is a secondary convenience workflow.

Empty sources, or sources with missing required samples, must not generate speech. This rule is central to the product.

## MVP Decisions

- Use a compact Korean minimal sample set first, not all 11,172 Hangul syllables.
- Treat the app as a cute syllable/jamo sampler, not accurate natural Korean TTS.
- Store data locally under the user's app config directory in a `guvoice` folder.
- Keep MVP metadata in JSON rather than SQLite.
- Let the frontend run in a normal browser with localStorage fallback when Wails bindings are unavailable.
- Use Wails bindings when running as the desktop app.

## UI Direction

The app should open as a usable tool, not a landing page.

Current layout:

- Left: voice source list and create button.
- Center: tabs for Speak, Record, Source Manager.
- Right: engine status, selected source coverage, button readiness, next sample prompts.

Important UX rule:

- If the user cannot preview/export because samples are missing, the UI should make the missing state obvious and steer them to recording.
- Recording must happen inside guvoice. The Record tab uses browser/WebView2 `getUserMedia` + Web Audio API PCM capture; if there is no selected source, pressing Record creates one automatically before requesting microphone permission. Saved recordings are mono 16-bit WAV data URLs so Go can decode them directly.
- Avoid broad page-level inner scrolling on desktop. Keep the main tool surface fitted to the Wails window; only long source/sample/prompt lists should scroll.
- Do not present grouped phrases such as `아 에 이 오 우...` as one required sample unless automatic segmentation exists. Current MVP records one prompt as one sample, so required prompts should be individual syllables/tokens like `아`, `어`, `가`.

## Current Implementation

Frontend:

- `frontend/src/App.tsx`: main React UI.
- `frontend/src/lib/adapter.ts`: Wails-first API adapter with browser localStorage fallback.
- `frontend/src/types.ts`: UI data contracts.
- `frontend/src/styles.css`: operational desktop-tool styling.

Backend:

- `main.go`: Wails v2 app entrypoint, embeds `frontend/dist`.
- `app.go`: backend domain methods and sample-based WAV synthesis orchestration.
- `frontend_api.go`: Wails methods matching the frontend adapter names.
- `prompts.go`: guvoice minimal prompt catalog, text-to-prompt mapping, and usable WAV sample checks.
- `internal/storage/store.go`: JSON state, source/sample/upload/export persistence.
- `internal/hangul/hangul.go`: Hangul decomposition/compose helpers.
- `internal/catalog/catalog.go`: Korean minimal sample set.
- `internal/synth/wav.go`: WAV reader/writer and concatenative sequence renderer.

Prosody note: spaces, commas, periods, `!`, `?`, and `~` are synthesis controls, not sample requirements. They add clamped pauses or adjust the previous prompt step's gain, speed, gap, and stretch while preserving the rule that empty or incomplete voice sources cannot generate speech.

Docs:

- `docs/planning.md`: planning notes and MVP reasoning.
- `README.md`: current usage, data layout, and limitations.

## Current Limitations

- Sample concatenation synthesis is implemented for WAV samples recorded by the app.
- Export currently saves the sample-based output as WAV because no MP3 encoder/ffmpeg is wired in.
- Wails generated `frontend/wailsjs/` and `frontend/package.json.md5` are ignored because the current frontend adapter does not import generated bindings directly.
- Older samples captured as WebM/Opus by previous builds are not usable for synthesis; re-record them in the current WAV-based recorder.
- WAV decoding currently accepts RIFF/WAVE 16-bit PCM. WAVE_FORMAT_EXTENSIBLE uploads are not handled yet; app recordings already use supported PCM16 WAV.

## Verification Already Done

```powershell
cd frontend
npm run build
```

Result: passed.

Go and Wails were installed locally for this workspace session:

- Go: `C:\Users\zjavb\.codex\tools\go1.26.3\go`
- Wails CLI: `C:\Users\zjavb\.codex\tools\gobin\wails.exe`

Use this PATH setup in PowerShell if system Go/Wails are still unavailable:

```powershell
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.3\go'
$gobin = Join-Path $env:USERPROFILE '.codex\tools\gobin'
$env:GOROOT = $goRoot
$env:GOBIN = $gobin
$env:PATH = (Join-Path $goRoot 'bin') + ';' + $gobin + ';' + $env:PATH
```

These commands were verified after installing local tools:

```powershell
go test ./...
wails doctor
wails build
```

Result: passed. `wails build` produced `build\bin\guvoice.exe`.

`ffmpeg -version` was still unavailable, so real MP3 encoding remains unwired.

Additional verification after sample-based synthesis implementation:

```powershell
go test ./...
npm run build
wails build
```

Result: passed. The root integration test records all required promptIds into a temp store, generates a sample-based WAV, and reads it back through the WAV decoder.

## Recommended Next Steps

1. Run `wails dev` with the local Go/Wails PATH above or install Go/Wails globally.
2. Add real MP3 export through ffmpeg detection/bundling or a Go encoder.
3. Improve recording flow with sample-by-sample queue, quality checks, trim, and retry.
4. Add better upload validation/conversion for non-WAV files.

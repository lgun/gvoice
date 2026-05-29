# guvoice Project Memory

Last updated: 2026-05-29

## Product Direction

guvoice / gvoice is a Go + Wails + React desktop app for making playful Korean sample-based voices. The target feeling is similar to Animal Crossing style speech, but the app should avoid positioning itself as a full Korean TTS engine.

The core loop is:

1. Create or select a voice source.
2. Fill the source with required samples through direct recording or upload.
3. Type text in the Speak screen.
4. Generate a preview only when the selected source is sufficiently filled with usable samples.
5. Export the result as MP3 or save it to the in-app speech library.

Direct recording is the primary workflow. Upload is a secondary convenience workflow.

Empty sources, sources with missing required samples, and sources whose required samples are silent or unreadable must not generate speech. This rule is central to the product.

## MVP Decisions

- Use a compact Korean minimal sample set first, not all 11,172 Hangul syllables.
- Voice sources/presets now have a recording type. The four types normalize `targetSamples` to 25, 100, 200, or 300 samples: inaccurate 25, fairly accurate 100, quite accurate 200, and very accurate 300.
- Legacy/imported/arbitrary `targetSamples` values snap to the nearest supported recording type.
- Treat the app as a cute syllable/jamo sampler, not accurate natural Korean TTS.
- Store data locally under the user's app config directory in a `guvoice` folder.
- Keep MVP metadata in JSON rather than SQLite.
- Let the frontend run in a normal browser with localStorage fallback when Wails bindings are unavailable.
- Use Wails bindings when running as the desktop app.
- Keep export destination configurable, while preserving the app's default `exports` folder as the normalized default.
- Keep the in-app speech library separate from MP3 export output. The two configured directories must not resolve to the same physical path.

## UI Direction

The app should open as a usable tool, not a landing page.

Current layout:

- Left: voice source list and create button.
- Center: tabs for Speak, Record, Library, Source Manager, and settings-like export/library controls.
- Right: engine status, selected source coverage, button readiness, next sample prompts.

Important UX rules:

- If the user cannot preview/export because samples are missing or unusable, the UI should make the missing state obvious and steer them to recording.
- Recording must happen inside guvoice. The Record tab uses browser/WebView2 `getUserMedia` + Web Audio API PCM capture; if there is no selected source, pressing Record creates one automatically before requesting microphone permission.
- After saving a recording, the Record tab advances automatically to the next missing prompt.
- The Record tab includes next missing, skip, and re-record controls to reduce click fatigue.
- The Record tab prompt grid, status, and next prompt only show prompts up to the selected preset's target.
- The Record tab now also has a sentence recording flow. Users can read a built-in Korean sentence pack or entered sentence through in-app `getUserMedia` recording, then review backend-extracted sample candidates before saving them.
- The left `+` button and empty-state CTA now open preset/source management creation instead of jumping straight to the Record tab. The post-create flow still moves into recording.
- The Speak tab includes a `보관함 저장` action that creates an MP3 from the current text, source, and options, then stores it in the app's speech library folder.
- The `보관함` tab lists saved speech items with title, source, date, duration, and file path. Items can be deleted, and item audio is loaded lazily through `재생 준비` before rendering `<audio controls>`.
- Avoid broad page-level inner scrolling on desktop. Keep the main tool surface fitted to the Wails window; only long source/sample/prompt lists should scroll.
- Do not present grouped phrases as one required sample unless automatic segmentation exists. Current MVP records one prompt as one sample, so required prompts should be individual syllables/tokens.

## Current Implementation

Frontend:

- `frontend/src/App.tsx`: main React UI, including recording workflow, speech library, source management, and MP3 export/library folder controls.
- `frontend/src/lib/adapter.ts`: Wails-first API adapter with browser localStorage fallback.
- `frontend/src/lib/audio.ts`: shared Web Audio helpers for recording/upload decoding, leading/trailing silence trim, and mono 16-bit WAV encoding.
- `frontend/src/types.ts`: UI data contracts.
- `frontend/src/styles.css`: operational desktop-tool styling.
- Record sentence capture lives in the existing React UI and uses the same browser/WebView2 microphone path before sending the known script and WAV data to the backend for candidate extraction.

Backend:

- `main.go`: Wails v2 app entrypoint, embeds `frontend/dist`.
- `app.go`: backend domain methods and sample-based WAV synthesis orchestration.
- `frontend_api.go`: Wails methods matching the frontend adapter names, including directory selection via Wails `OpenDirectoryDialog`, `ListSentencePrompts`, and `ExtractSentenceSamples`.
- `sentence_extraction.go`: sentence recording prompt/candidate extraction. This is VAD/energy plus script-proportional segmentation heuristic, not a complete ASR or forced-alignment engine.
- `sample_validation.go`: backend WAV normalization/trim/readiness validation helpers.
- `prompts.go`: guvoice minimal prompt catalog, text-to-prompt mapping, and usable WAV sample checks.
- `internal/storage/store.go`: JSON state, source/sample/upload/export persistence, including export and speech library folder preferences.
- `internal/hangul/hangul.go`: Hangul decomposition/compose helpers.
- `internal/catalog/catalog.go`: Korean minimal sample set.
- `internal/synth/wav.go`: WAV reader/writer, MP3 writer, simple DSP options, sample trim/normalization, and concatenative sequence renderer.

Prosody note: spaces, commas, periods, `!`, `?`, and `~` are synthesis controls, not sample requirements. They add clamped pauses or adjust the previous prompt step's gain, speed, gap, and stretch while preserving the rule that empty or incomplete voice sources cannot generate speech.

Prompt catalog note: the first 25 minimal prompts are preserved. Prompts 26-300 extend coverage with balanced exact syllable prompts, avoid duplicate minimal text, and include multiple initial consonants and practical medial vowels.

## Current Behavior

- Recording and upload both pass through shared Web Audio processing, trim leading/trailing silence, and store mono 16-bit WAV.
- The backend also normalizes and trims saved/uploaded WAV samples.
- Silent samples are rejected at save/upload time.
- Legacy samples that are silent or cannot be read are excluded from analysis and synthesis readiness, so they do not make a source look usable.
- Preview renders WAV data URLs.
- Export writes real `.mp3` files using the pure Go `github.com/braheezy/shine-mp3/pkg/mp3` encoder.
- MP3 export folder is user-configurable. The persisted setting uses `path=""` for default; the UI shows `defaultPath` for the actual default `exports` directory. If the user selects or enters the actual default exports path, it is normalized back to the default setting.
- Speech library saving is separate from one-off MP3 export. `SaveSpeechItem` creates an MP3 from the current text/source/options and stores it under the speech library directory.
- Speech library folder settings mirror the export-folder default behavior: the default is app data `speech-library`, persisted state keeps the default as `path=""`, and the UI receives both `defaultPath` and `resolvedPath`.
- Export and speech library settings reject paths that resolve to the same physical directory, preventing exported files and saved playlist/library items from being mixed together.
- `ListSpeechItems` returns metadata only. Audio data is loaded on demand through `GetSpeechItemAudio`, which returns `data:audio/mpeg;base64,...` for the selected saved item.
- Speech library backend API methods are `GetSpeechLibrarySettings`, `SetSpeechLibraryDirectory`, `ChooseSpeechLibraryDirectory`, `ListSpeechItems`, `SaveSpeechItem`, `DeleteSpeechItem`, and `GetSpeechItemAudio`.
- `SaveSpeechItem` applies nested options including speed, pitch, clarity, and noise reduction.
- Sentence recording backend API methods are `ListSentencePrompts` and `ExtractSentenceSamples`.
- Sentence extraction requests pass the selected preset's `targetSamples`.
- Sentence extraction returns candidates with `id`, `promptId`, `label`, `text`, `timing`, `confidence`, `status`/`warning`, `audioName`, `audioUrl`, and `dataBase64`.
- Sentence extraction is intentionally conservative: silent, near-silent, too-short, insufficient-speech, or one/two-sound recordings return `candidates=[]` so a source is not incorrectly filled.
- Candidate playback/review is part of the workflow before saving. The frontend supports saving individual candidates and "save all usable candidates"; bulk save only includes candidates with ready/usable/good/ok/accepted status, `confidence >= 0.75`, and no warning. Review/warning candidates require individual listening and saving.
- After a candidate save succeeds, a later refresh failure is still treated as save success to reduce duplicate-save retry risk.
- For 100/200/300-sample recording types, analysis, synthesis, and sentence candidate extraction prefer exact syllable prompts inside the source target range when present. If no target-range exact prompt exists, they fall back to the existing representative consonant/vowel path.

## Current Limitations

- Sample concatenation synthesis is implemented for WAV samples recorded or uploaded through the current app flows.
- Sentence recording candidate extraction is heuristic. It assumes the known script was read reasonably and uses VAD/energy and proportional timing rather than ASR/forced alignment, so candidate boundaries need user review before saving.
- Wails generated `frontend/wailsjs/` and `frontend/package.json.md5` are ignored because the current frontend adapter does not import generated bindings directly.
- Older samples captured as WebM/Opus by previous builds are not usable directly from disk; re-upload them through the current upload flow or re-record them so they are stored as WAV samples.
- Upload uses WebView2/browser `decodeAudioData` and then stores mono 16-bit WAV. Formats/codecs the WebView cannot decode still fail with a clear error.
- Pitch, clarity, and noise reduction are implemented as simple DSP approximations over PCM, not as studio-grade voice processing.
- Clarity DSP has been improved, but it is still intentionally simple: lower clarity applies low-pass/smoothing for a muffled sound; higher clarity applies high/transient emphasis and normalization for clearer articulation boundaries.

## Verification Already Done

This implementation has been checked during the current/parent work with:

```powershell
go test ./...
cd frontend
npm run build
cd ..
git diff --check
wails build
```

Implementation verification passed: `go test ./...`, `npm run build`, `git diff --check`, and `wails build`.

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

Useful verification commands:

```powershell
go test ./...
wails doctor
wails build
```

`wails build` should produce `build\bin\guvoice.exe`.

MP3 encoding no longer depends on `ffmpeg`; it is handled by a bundled pure Go dependency.

## Recommended Next Steps

1. Run `wails dev` and manually check the recording queue, sentence recording with a real microphone, candidate playback/save, upload trim, export folder picker, speech library save/list/delete, and lazy playback in WebView2.
2. Manually check 100/200/300 large-recording flows in a real WebView2 window, especially prompt-grid scrolling and recording fatigue.
3. Improve 300-prompt ordering and the sentence pack/candidate boundary quality, especially for Korean prompt coverage and natural read pacing.
4. Investigate a more accurate free forced-alignment or ASR-assisted candidate path if it can stay local/practical.
5. Add focused regression tests around export-folder default normalization if not already covered enough by storage/API tests.
6. Continue improving DSP quality only within the sample-based product direction; do not weaken the rule that unusable sources cannot synthesize speech.

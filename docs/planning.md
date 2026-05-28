# guvoice Planning

guvoice is a Wails desktop tool for making short, playful Korean sample-based voices. The product is intentionally closer to a syllable sampler than a full Korean TTS engine.

## MVP Decision

- Users create voice sources and fill required samples by recording or upload.
- Empty sources, incomplete sources, silent samples, and unreadable legacy samples cannot generate speech.
- The primary capture flow is direct recording.
- The first usable voice mode is a small Korean minimal set, not all 11,172 Hangul syllables.
- Generated audio can be previewed and exported.
- MP3 export location is configurable, but the app keeps a normalized default state for the built-in exports folder.

## Minimal Sample Strategy

The first mode uses a compact set so users can hear results quickly:

- Vowel-like syllables.
- Representative consonant syllables.
- Fallback prompt.

For each Hangul syllable, the app decomposes the character into choseong, jungseong, and jongseong, then chooses the best available sample:

1. Exact syllable sample.
2. Representative syllable for the initial consonant.
3. Vowel sample.
4. Fallback sample.

The MVP keeps final consonants as a timing/pitch artifact rather than requiring full final-consonant recording.

## Core Screens

- Speak: select a source, type text, check missing/unusable samples, preview, export.
- Record: record required samples with progress, automatic advance to the next missing prompt, next missing, skip, and re-record controls.
- Source Manager: upload samples, inspect coverage, duplicate/delete/export/import sources where supported.
- Export/settings controls: choose or reset the MP3 export folder.

## Implementation Notes

- Wails/Go owns persistence and export paths.
- React owns recording UI through `getUserMedia` + Web Audio API PCM capture and passes mono 16-bit WAV data URLs to Go when Wails bindings exist.
- Recording and upload share frontend Web Audio helpers for decode/capture, leading/trailing silence trim, and mono 16-bit WAV encoding.
- The frontend also has a localStorage fallback so the UI can be demonstrated in a normal browser during development.
- Internal sample storage keeps current captures and metadata under the user's application data directory.
- The Go backend decodes saved WAV samples by promptId, normalizes/trims samples, rejects silence, renders preview WAV files, and renders MP3 exports by concatenating the mapped sample sequence.
- Upload accepts browser/WebView2-decodable `audio/*`, converts it to mono 16-bit WAV in the frontend, and sends that WAV sample to Go.
- The backend WAV decoder accepts RIFF/WAVE 16-bit PCM and WAVE_FORMAT_EXTENSIBLE PCM subtype.
- Legacy samples that are silent or fail WAV reading are excluded from analysis and synthesis readiness.
- Speed, pitch, clarity, and noise reduction are applied as simple DSP approximations over PCM.
- Clarity now has two more distinct directions: low values apply low-pass/smoothing for a muffled result, while high values apply high/transient emphasis and normalization to preserve clearer articulation boundaries.
- Prosody rule: supported punctuation is preserved in `sequenceForText` as synthesis timing/expression. Spaces become short clamped pauses, comma/period-like marks have distinct pause lengths, `!` emphasizes, `?` slows/stretches the ending, and `~` stretches the previous sample without adding missing sample requirements.

## Export Folder Behavior

- The default export destination is the app data `exports` directory.
- State persists the MP3 folder setting.
- `path=""` means "use the default export folder".
- The UI receives/displays `defaultPath` so users can see where the default points.
- Selecting or typing the resolved default exports path is normalized back to `path=""`.
- Wails desktop builds use `OpenDirectoryDialog` for folder picking.

## Recently Completed

- Recording save now advances to the next missing prompt.
- Added next missing, skip, and re-record controls in the Record tab.
- Unified recording/upload WAV preparation through shared Web Audio helpers.
- Added frontend silence trimming for both recording and upload.
- Added backend sample normalize/trim and silent sample rejection.
- Excluded legacy silent/unreadable samples from readiness.
- Improved clarity DSP behavior.
- Added MP3 export folder settings UI/API and persistence.
- Parent verification passed: `go test ./...`, `npm run build`, and `wails build`.

## Next Work

1. Manually test `wails dev` for recording queue flow, upload trim, and export folder selection in WebView2.
2. Add/expand focused tests for export-folder default normalization if coverage is thin.
3. Consider import/migration help for samples from older WebM/Opus builds.
4. Continue incremental DSP polish while preserving the "no usable samples, no speech" product rule.

# guvoice Planning

guvoice is a Wails desktop tool for making short, playful Korean sample-based voices. The product is intentionally closer to a syllable sampler than a full Korean TTS engine.

## MVP Decision

- Users create voice sources and fill required samples by recording or upload.
- Empty sources, incomplete sources, silent samples, and unreadable legacy samples cannot generate speech.
- The primary capture flow is direct recording.
- Sentence recording is an assisted capture flow for proposing samples from a known Korean script, but it does not weaken the sample readiness rule.
- The first usable voice mode is a small Korean minimal set, not all 11,172 Hangul syllables.
- Generated audio can be previewed and exported.
- MP3 export location is configurable, but the app keeps a normalized default state for the built-in exports folder.
- Generated MP3 speech can also be saved to an in-app speech library/playback list that remains separate from one-off MP3 exports.

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

- Speak: select a source, type text, check missing/unusable samples, preview, export, and save the current generated MP3 to the speech library with `보관함 저장`.
- Record: record required samples with progress, automatic advance to the next missing prompt, next missing, skip, and re-record controls. It also supports sentence recording from a built-in Korean sentence pack or user-entered sentence, followed by candidate playback/review/save.
- Library (`보관함`): list saved speech items, show title/source/date/duration/file path, delete items, and lazily prepare item audio for `<audio controls>` playback.
- Source Manager: upload samples, inspect coverage, duplicate/delete/export/import sources where supported.
- Export/settings controls: choose or reset the MP3 export folder and speech library folder.

## Implementation Notes

- Wails/Go owns persistence and export paths.
- React owns recording UI through `getUserMedia` + Web Audio API PCM capture and passes mono 16-bit WAV data URLs to Go when Wails bindings exist.
- Recording and upload share frontend Web Audio helpers for decode/capture, leading/trailing silence trim, and mono 16-bit WAV encoding.
- Sentence recording uses the same in-app microphone path. The backend receives the known script and WAV data, then extracts candidate samples with VAD/energy and script-proportional segmentation. This is a heuristic candidate extractor, not complete ASR or forced alignment.
- Sentence recording Wails APIs are `ListSentencePrompts` and `ExtractSentenceSamples`.
- Sentence candidates carry `id`, `promptId`, `label`, `text`, `timing`, `confidence`, `status`/`warning`, `audioName`, `audioUrl`, and `dataBase64`.
- The extractor returns `candidates=[]` for silent, near-silent, too-short, insufficient-speech, or one/two-sound recordings so bad input cannot fill a source. Users are expected to play and inspect candidates before saving.
- Candidate saving supports individual save and "save all usable candidates". Bulk save only includes ready/usable/good/ok/accepted candidates with `confidence >= 0.75` and no warning; review/warning candidates require individual save after listening.
- Once a candidate save succeeds, a later refresh failure is still treated as save success to reduce duplicate retry risk.
- The frontend also has a localStorage fallback so the UI can be demonstrated in a normal browser during development.
- Internal sample storage keeps current captures and metadata under the user's application data directory.
- The Go backend decodes saved WAV samples by promptId, normalizes/trims samples, rejects silence, renders preview WAV files, and renders MP3 exports by concatenating the mapped sample sequence.
- Upload accepts browser/WebView2-decodable `audio/*`, converts it to mono 16-bit WAV in the frontend, and sends that WAV sample to Go.
- The backend WAV decoder accepts RIFF/WAVE 16-bit PCM and WAVE_FORMAT_EXTENSIBLE PCM subtype.
- Legacy samples that are silent or fail WAV reading are excluded from analysis and synthesis readiness.
- Speed, pitch, clarity, and noise reduction are applied as simple DSP approximations over PCM.
- Clarity now has two more distinct directions: low values apply low-pass/smoothing for a muffled result, while high values apply high/transient emphasis and normalization to preserve clearer articulation boundaries.
- Prosody rule: supported punctuation is preserved in `sequenceForText` as synthesis timing/expression. Spaces become short clamped pauses, comma/period-like marks have distinct pause lengths, `!` emphasizes, `?` slows/stretches the ending, and `~` stretches the previous sample without adding missing sample requirements.
- Speech library APIs are `GetSpeechLibrarySettings`, `SetSpeechLibraryDirectory`, `ChooseSpeechLibraryDirectory`, `ListSpeechItems`, `SaveSpeechItem`, `DeleteSpeechItem`, and `GetSpeechItemAudio`.
- `SaveSpeechItem` renders MP3 using the current text, source, and nested options including speed, pitch, clarity, and noise reduction.
- `ListSpeechItems` returns metadata only. `GetSpeechItemAudio` performs lazy audio loading and returns `data:audio/mpeg;base64,...`.

## Export Folder Behavior

- The default export destination is the app data `exports` directory.
- State persists the MP3 folder setting.
- `path=""` means "use the default export folder".
- The UI receives/displays `defaultPath` so users can see where the default points.
- Selecting or typing the resolved default exports path is normalized back to `path=""`.
- Wails desktop builds use `OpenDirectoryDialog` for folder picking.

## Speech Library Folder Behavior

- The default speech library destination is the app data `speech-library` directory.
- State persists the speech library folder setting.
- `path=""` means "use the default speech library folder".
- The UI receives/displays both `defaultPath` and `resolvedPath`.
- MP3 export and speech library directories must not resolve to the same physical path; both settings reject that overlap so exported files and saved library items stay separate.

## Recently Completed

- Recording save now advances to the next missing prompt.
- Added next missing, skip, and re-record controls in the Record tab.
- Unified recording/upload WAV preparation through shared Web Audio helpers.
- Added frontend silence trimming for both recording and upload.
- Added backend sample normalize/trim and silent sample rejection.
- Excluded legacy silent/unreadable samples from readiness.
- Improved clarity DSP behavior.
- Added MP3 export folder settings UI/API and persistence.
- Added speech library save/list/delete/lazy-playback UI and backend API.
- Added speech library folder settings UI/API and same-physical-path rejection against the MP3 export directory.
- Added sentence recording based candidate extraction with conservative empty/noisy/under-filled input rejection.
- Implementation verification used and passed: `go test ./...`, `npm run build`, `git diff --check`, and `wails build`. Final parent verification may continue if more work lands afterward.

## Next Work

1. Manually test `wails dev` with a real microphone for normal recording, sentence recording, candidate playback/save, upload trim, export/library folder selection, speech library save/delete, and lazy playback in WebView2.
2. Improve the Korean sentence pack and candidate boundary quality.
3. Research a more accurate free forced-alignment or ASR-assisted candidate path if it can stay practical for the app.
4. Add/expand focused tests for export-folder default normalization if coverage is thin.
5. Continue incremental DSP polish while preserving the "no usable samples, no speech" product rule.

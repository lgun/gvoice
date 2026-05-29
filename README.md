# guvoice / gvoice

guvoice is a Wails + Go + React desktop app for making playful Korean sample-based voices. It is closer to a cute syllable sampler than a full Korean TTS engine: users record or upload short prompt samples, then the app maps typed Korean text to those samples and stitches them into speech-like audio.

The core product rule is strict: an empty source, an incomplete source, or a source whose required samples are silent/unreadable cannot preview or export speech.

## Current Features

- Create, edit, and delete voice sources.
- Track required sample coverage per source.
- Record samples directly in the app with WebView2/browser microphone capture.
- After saving a recording, automatically advance to the next missing prompt.
- Use next missing, skip, and re-record controls to move through the prompt queue with less clicking.
- Record a Korean sentence pack or entered sentence and review backend-extracted sample candidates before saving.
- Upload browser/WebView2-decodable `audio/*` files as samples.
- Trim leading/trailing silence for both recordings and uploads.
- Store current samples as mono 16-bit WAV.
- Reject silent samples.
- Ignore legacy silent/unreadable samples when deciding whether a source is ready.
- Preview generated audio as WAV.
- Export generated audio as real MP3.
- Configure the MP3 export folder with a Wails directory picker. Leaving the setting at default uses the app's `exports` folder.
- Save generated MP3 speech items to an in-app speech library from the Speak tab with `보관함 저장`.
- Use the `보관함` tab to list saved speech items, inspect title/source/date/duration/file path, delete items, and prepare an item for lazy `<audio controls>` playback.
- Configure the speech library folder separately from the MP3 export folder. The app rejects settings where both folders resolve to the same physical path.
- Run the frontend in a normal browser with a localStorage fallback when Wails bindings are unavailable.

## Audio Behavior

Recording and upload share Web Audio processing in the frontend. The app decodes/captures audio, trims leading and trailing silence, converts to mono 16-bit WAV, and sends that WAV sample to the Go backend.

The backend performs additional WAV normalization and trim before storing or using samples. Samples that are silent after processing are rejected. Older samples that cannot be decoded, or are effectively silent, are excluded from analysis and synthesis readiness.

Synthesis maps input text to prompt IDs, loads the matching usable WAV samples, applies simple PCM DSP, and concatenates the result. Spaces and punctuation are expression controls, not sample requirements:

- Spaces add short clamped pauses.
- Commas, periods, and sentence marks add distinct pause lengths.
- `!` emphasizes the previous sample.
- `?` gives the previous ending a slower/stretched question feel.
- `~` stretches the previous sample with a repeat cap.

Saving to the speech library uses the same source readiness rule and synthesis options as MP3 export, including nested speed, pitch, clarity, and noise reduction settings. Library listings return metadata first, and each item's MP3 data is loaded only when playback is requested.

Sentence recording is still sample-based. The Record tab captures the user reading a known Korean sentence through `getUserMedia`, then the backend uses the known script to propose sample candidates. This is not full ASR or forced alignment; it is VAD/energy plus script-proportional segmentation, so candidates are meant to be played back and checked before saving.

The extractor is conservative. Silent, near-silent, too-short, under-filled, or one/two-sound recordings return no candidates instead of filling a source with bad data. Candidate records include `id`, `promptId`, `label`, `text`, `timing`, `confidence`, `status`/`warning`, `audioName`, `audioUrl`, and `dataBase64`. Bulk save only includes candidates that are ready/usable/good/ok/accepted, have `confidence >= 0.75`, and have no warning; review/warning candidates are saved individually after listening.

## Data Location

User data is stored under the user's app config directory in a `guvoice` folder.

- `state.json`: voice sources, samples, uploads, export folder preference, speech library folder preference, saved speech item metadata, and history metadata.
- `samples/{voiceSourceId}/prompts`: recorded prompt samples.
- `samples/{voiceSourceId}/uploads`: uploaded samples.
- `exports`: default preview/export output folder.
- `speech-library`: default saved speech library folder.

The MP3 folder setting stores an empty path as the default. The UI can still display the resolved default exports path. If the user selects or enters the actual default exports path, the app normalizes it back to the default setting.

The speech library folder follows the same default-state pattern: an empty stored path means the app data `speech-library` folder, while the UI receives the default and resolved physical paths. Export and library directories are intentionally kept distinct.

## Development

Frontend development:

```powershell
cd frontend
npm install
npm run dev
```

Go/Wails development:

```powershell
go mod tidy
go test ./...
wails dev
```

If Go and Wails are not installed globally, this workspace has also used local tools:

```powershell
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.3\go'
$gobin = Join-Path $env:USERPROFILE '.codex\tools\gobin'
$env:GOROOT = $goRoot
$env:GOBIN = $gobin
$env:PATH = (Join-Path $goRoot 'bin') + ';' + $gobin + ';' + $env:PATH

go test ./...
wails doctor
wails build
```

The current sentence-extraction implementation was checked with:

```powershell
go test ./...
npm run build
git diff --check
wails build
```

`wails build` should produce `build\bin\guvoice.exe`.

## Limitations

- The app is not natural Korean TTS. It is a compact Korean sample mapper and concatenative renderer.
- Sentence recording is a heuristic candidate extractor, not an ASR/forced-alignment system; candidate timing quality depends on the recording and script match.
- The first voice mode uses a minimal prompt set, not all Hangul syllables.
- Current audio quality depends heavily on the recorded/uploaded samples.
- Pitch, clarity, and noise reduction are simple PCM DSP approximations.
- Clarity is intentionally stylized: low clarity uses low-pass/smoothing for a muffled sound, while high clarity uses high/transient emphasis and normalization for clearer articulation boundaries.
- Upload depends on WebView2/browser `decodeAudioData`; unsupported codecs fail with a clear error.
- Older WebM/Opus samples from previous builds should be re-uploaded or re-recorded so they become current WAV samples.
- MP3 encoding is handled by the pure Go `github.com/braheezy/shine-mp3/pkg/mp3` dependency; `ffmpeg` is not required.

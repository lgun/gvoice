# guvoice Planning

guvoice is a Wails desktop tool for making short, playful Korean sample-based voices. The product is intentionally closer to a syllable sampler than a full Korean TTS engine.

## MVP Decision

- Users create voice sources and fill required samples by recording or upload.
- Empty sources and sources missing required samples cannot generate speech.
- The primary capture flow is direct recording.
- The first usable voice mode is a small Korean minimal set, not all 11,172 Hangul syllables.
- Generated audio can be previewed and exported.

## Minimal Sample Strategy

The first mode uses a compact set so users can hear results quickly:

- Vowel-like syllables: 아, 야, 어, 여, 오, 요, 우, 유, 으, 이, 애, 에
- Representative consonant syllables: 가, 나, 다, 라, 마, 바, 사, 자, 차, 카, 타, 파, 하
- Fallback: 아

For each Hangul syllable, the app decomposes the character into choseong, jungseong, and jongseong, then chooses the best available sample:

1. Exact syllable sample
2. Representative syllable for the initial consonant
3. Vowel sample
4. Fallback sample

The MVP keeps final consonants as a timing/pitch artifact rather than requiring full final-consonant recording.

## Core Screens

- Speak: select a source, type text, check missing samples, preview, export.
- Record: record required samples with progress, retry, keyboard-friendly flow, level/quality hints.
- Source Manager: upload samples, inspect coverage, duplicate/delete/export/import sources.

## Implementation Notes

- Wails/Go owns persistence and export paths.
- React owns recording UI through `getUserMedia` + Web Audio API PCM capture and passes mono 16-bit WAV data URLs to Go when Wails bindings exist.
- The frontend also has a localStorage fallback so the UI can be demonstrated in a normal browser during development.
- Internal sample storage should keep original captures and metadata under the user's application data directory.
- The Go backend decodes saved WAV samples by promptId, renders preview WAV files, and renders MP3 exports by concatenating the mapped sample sequence.
- Upload accepts browser/WebView2-decodable `audio/*`, converts it to mono 16-bit WAV in the frontend, and sends that WAV sample to Go.
- The backend WAV decoder accepts RIFF/WAVE 16-bit PCM and WAVE_FORMAT_EXTENSIBLE PCM subtype.
- Speed, pitch, clarity, and noise reduction are applied as simple DSP approximations over PCM.
- Prosody rule: supported punctuation is preserved in `sequenceForText` as synthesis timing/expression. Spaces become short clamped pauses, comma/period-like marks have distinct pause lengths, `!` emphasizes, `?` slows/stretches the ending, and `~` stretches the previous sample without adding missing sample requirements.

# guvoice / 구보이스

구보이스는 Wails + Go + React로 만든 한글 샘플 기반 목소리 제작 앱입니다. 목표는 완전한 한국어 TTS가 아니라, 사용자가 직접 녹음한 짧은 샘플을 조합해 동물의 숲풍으로 말하는 듯한 소리를 빠르게 만드는 것입니다.

## 현재 구현

- 목소리 소스 생성, 수정, 삭제
- 소스별 목표 샘플 수와 진행률 저장
- Web Audio API 기반 직접 녹음 UI
- 오디오 파일 업로드 UI
- 소스가 목표 샘플 수만큼 채워지기 전까지 미리듣기/저장 차단
- Wails 바인딩이 없을 때도 동작하는 브라우저 localStorage fallback
- Go 백엔드의 앱 데이터 폴더 저장
- 녹음된 WAV 샘플을 promptId 순서로 이어 붙이는 미리듣기/저장

녹음은 외부 앱을 쓰지 않고 앱 안에서 진행합니다. `녹음` 탭에서 `녹음 시작`을 누르면 목소리 소스가 없을 때 자동으로 만들고, WebView2/브라우저 마이크 권한을 요청한 뒤 정지 시 mono 16-bit WAV 샘플로 저장합니다. MVP에서는 한 번 녹음한 파일을 자동으로 여러 음절로 분할하지 않습니다. `아`, `어`, `가`처럼 화면에 표시된 한 항목을 하나씩 녹음합니다.

## 데이터 위치

앱 데이터는 사용자 설정 폴더 아래 `guvoice` 디렉터리에 저장됩니다.

- `state.json`: 목소리 소스, 샘플, 업로드, 생성 이력
- `samples/{voiceSourceId}/prompts`: 녹음 샘플
- `samples/{voiceSourceId}/uploads`: 업로드 샘플
- `exports`: 미리듣기/저장 결과

## 개발 실행

```powershell
cd frontend
npm install
npm run dev
```

Go와 Wails가 설치된 환경에서는 다음 흐름을 사용합니다.

```powershell
go mod tidy
go test ./...
wails dev
```

현재 작업 환경에서는 시스템 전역 설치 대신 로컬 도구 폴더를 사용해 검증했습니다.

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

검증 결과 `build\bin\guvoice.exe` 생성까지 성공했습니다.

## 제한

현재 음성 합성은 필수 promptId에 해당하는 실제 WAV 샘플이 모두 있을 때만 동작합니다. 한글 입력은 최소 매핑으로 promptId 시퀀스로 변환되고, 백엔드가 해당 WAV들을 trim/resample/fade 처리 후 이어 붙여 결과 WAV를 만듭니다. MVP 저장 버튼은 WAV 파일 저장으로 동작합니다. 다음 단계에서 ffmpeg 번들 또는 Go MP3 인코더를 연결하면 실제 MP3 export로 바꿀 수 있습니다.

WAV 디코더는 현재 RIFF/WAVE 16-bit PCM을 지원합니다. 앱 자체 녹음은 이 형식으로 저장되며, WAVE_FORMAT_EXTENSIBLE 업로드 지원은 후속 작업입니다.

Prosody punctuation: spaces create short clamped pauses; commas, periods, and sentence marks add distinct pause lengths; `!` emphasizes the previous sample; `?` gives the previous ending a slower/stretched question feel; `~` stretches the previous sample with a cap for repeats. These marks never count as missing samples.

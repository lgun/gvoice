# guvoice / 구보이스

구보이스는 Wails + Go + React로 만든 한글 샘플 기반 목소리 제작 앱입니다. 목표는 완전한 한국어 TTS가 아니라, 사용자가 직접 녹음한 짧은 샘플을 조합해 동물의 숲풍으로 말하는 듯한 소리를 빠르게 만드는 것입니다.

## 현재 구현

- 목소리 소스 생성, 수정, 삭제
- 소스별 목표 샘플 수와 진행률 저장
- MediaRecorder 기반 직접 녹음 UI
- 오디오 파일 업로드 UI
- 소스가 목표 샘플 수만큼 채워지기 전까지 미리듣기/저장 차단
- Wails 바인딩이 없을 때도 동작하는 브라우저 localStorage fallback
- Go 백엔드의 앱 데이터 폴더 저장
- MVP placeholder WAV 미리듣기/저장

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

현재 음성 합성은 실제 샘플 연결 엔진이 아니라 통합 검증용 WAV placeholder입니다. `MP3 저장` 버튼은 백엔드가 MP3 인코더를 찾지 못하는 현재 상태에서는 WAV 파일 저장으로 동작합니다. 다음 단계에서 ffmpeg 번들 또는 Go MP3 인코더를 연결하면 실제 mp3 export로 바꿀 수 있습니다.

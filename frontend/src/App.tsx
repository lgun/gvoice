import { ChangeEvent, FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { voiceApi } from "./lib/adapter";
import {
  AnalysisResult,
  EngineStatus,
  MAX_SAMPLE_TARGET,
  MIN_SAMPLE_TARGET,
  PreviewResult,
  SAMPLE_PROMPTS,
  SamplePrompt,
  SynthesisOptions,
  TabId,
  VoiceSample,
  VoiceSource
} from "./types";

const initialOptions: SynthesisOptions = {
  speed: 1,
  pitch: 0,
  clarity: 72,
  noiseReduction: 48
};

const defaultSpeakText = "안녕하세요. 오늘은 구보이스 목소리 소스를 테스트합니다.";

const clampTargetSamples = (value?: number) =>
  Math.min(MAX_SAMPLE_TARGET, Math.max(MIN_SAMPLE_TARGET, value || MIN_SAMPLE_TARGET));

const formatDate = (value: string) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat("ko-KR", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
};

const requiredPromptsFor = (source?: VoiceSource) =>
  SAMPLE_PROMPTS.slice(0, clampTargetSamples(source?.targetSamples));

const filledPromptIdsOf = (source?: VoiceSource) =>
  new Set(source?.samples.map((sample) => sample.promptId ?? sample.label) ?? []);

const filledCountOf = (source?: VoiceSource) => {
  const required = requiredPromptsFor(source);
  const filledIds = filledPromptIdsOf(source);
  return required.filter((prompt) => filledIds.has(prompt.id)).length;
};

const progressOf = (source: VoiceSource) =>
  Math.min(100, Math.round((filledCountOf(source) / clampTargetSamples(source.targetSamples)) * 100));

const blobToDataUrl = (blob: Blob) =>
  new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error ?? new Error("오디오를 읽지 못했습니다."));
    reader.readAsDataURL(blob);
  });

type WindowWithWebkitAudio = Window & {
  webkitAudioContext?: typeof AudioContext;
};

const getAudioContextConstructor = () =>
  window.AudioContext ?? (window as WindowWithWebkitAudio).webkitAudioContext;

const mergePcmChunks = (chunks: Float32Array[]) => {
  const totalLength = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
  const merged = new Float32Array(totalLength);
  let offset = 0;
  for (const chunk of chunks) {
    merged.set(chunk, offset);
    offset += chunk.length;
  }
  return merged;
};

const arrayBufferToBase64 = (buffer: ArrayBuffer) => {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    const chunk = bytes.subarray(offset, offset + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
};

const encodeMono16Wav = (samples: Float32Array, sampleRate: number) => {
  const dataSize = samples.length * 2;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);

  const writeString = (offset: number, value: string) => {
    for (let i = 0; i < value.length; i += 1) {
      view.setUint8(offset + i, value.charCodeAt(i));
    }
  };

  writeString(0, "RIFF");
  view.setUint32(4, 36 + dataSize, true);
  writeString(8, "WAVE");
  writeString(12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeString(36, "data");
  view.setUint32(40, dataSize, true);

  let offset = 44;
  for (const sample of samples) {
    const clipped = Math.max(-1, Math.min(1, sample));
    view.setInt16(offset, clipped < 0 ? clipped * 0x8000 : clipped * 0x7fff, true);
    offset += 2;
  }

  return buffer;
};

const wavDataUrlFromPcm = (chunks: Float32Array[], sampleRate: number) => {
  const samples = mergePcmChunks(chunks);
  const buffer = encodeMono16Wav(samples, sampleRate);
  return {
    dataUrl: `data:audio/wav;base64,${arrayBufferToBase64(buffer)}`,
    previewUrl: URL.createObjectURL(new Blob([buffer], { type: "audio/wav" }))
  };
};

function ProgressBar({ value }: { value: number }) {
  return (
    <div className="progress" aria-label={`진행률 ${value}%`}>
      <span style={{ width: `${value}%` }} />
    </div>
  );
}

function FieldLabel({ children }: { children: React.ReactNode }) {
  return <span className="field-label">{children}</span>;
}

function SourceList({
  sources,
  selectedId,
  onSelect,
  onCreate
}: {
  sources: VoiceSource[];
  selectedId: string;
  onSelect: (id: string) => void;
  onCreate: () => void;
}) {
  return (
    <aside className="source-panel">
      <div className="panel-head">
        <div>
          <p className="eyebrow">guvoice</p>
          <h1>목소리 소스</h1>
        </div>
        <button className="icon-button" type="button" onClick={onCreate} title="새 소스">
          +
        </button>
      </div>

      {sources.length === 0 ? (
        <button className="source-empty" type="button" onClick={onCreate}>
          <strong>새 소스 만들기</strong>
          <span>녹음 탭에서 바로 시작할 수 있습니다.</span>
        </button>
      ) : (
        <div className="source-list">
          {sources.map((source) => {
            const progress = progressOf(source);
            return (
              <button
                className={`source-item ${source.id === selectedId ? "is-active" : ""}`}
                key={source.id}
                type="button"
                onClick={() => onSelect(source.id)}
              >
                <span className="source-title">{source.name}</span>
                <span className="source-meta">
                  {source.speaker} · 필수 {filledCountOf(source)}/{clampTargetSamples(source.targetSamples)}
                </span>
                <ProgressBar value={progress} />
              </button>
            );
          })}
        </div>
      )}
    </aside>
  );
}

function Tabs({
  activeTab,
  onChange
}: {
  activeTab: TabId;
  onChange: (tab: TabId) => void;
}) {
  const tabs: Array<{ id: TabId; label: string }> = [
    { id: "speak", label: "말하기" },
    { id: "record", label: "녹음" },
    { id: "manage", label: "소스 관리" }
  ];

  return (
    <div className="tabs" role="tablist" aria-label="작업 탭">
      {tabs.map((tab) => (
        <button
          className={activeTab === tab.id ? "is-active" : ""}
          key={tab.id}
          type="button"
          role="tab"
          aria-selected={activeTab === tab.id}
          onClick={() => onChange(tab.id)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

function SpeakTab({
  selectedSource,
  text,
  setText,
  analysis,
  isAnalyzing,
  options,
  setOptions,
  preview,
  isSynthesizing,
  isExporting,
  onPreview,
  onExport,
  onGoRecord
}: {
  selectedSource?: VoiceSource;
  text: string;
  setText: (text: string) => void;
  analysis: AnalysisResult | null;
  isAnalyzing: boolean;
  options: SynthesisOptions;
  setOptions: (options: SynthesisOptions) => void;
  preview: PreviewResult | null;
  isSynthesizing: boolean;
  isExporting: boolean;
  onPreview: () => void;
  onExport: () => void;
  onGoRecord: () => void;
}) {
  const sampleCount = filledCountOf(selectedSource);
  const targetCount = clampTargetSamples(selectedSource?.targetSamples);
  const hasBlockingMissing = Boolean(analysis?.missing.some((item) => item.severity === "missing"));
  const sourceComplete = Boolean(selectedSource && sampleCount >= targetCount);
  const canPreview = Boolean(selectedSource && text.trim() && sourceComplete && !hasBlockingMissing);
  const canExport = Boolean(preview?.audioUrl || preview?.status === "ready");

  const previewHint = !selectedSource
    ? "목소리 소스를 만들거나 선택하세요."
    : !text.trim()
      ? "말할 텍스트를 입력하세요."
      : sampleCount < targetCount
        ? `필수 샘플 ${targetCount - sampleCount}개를 더 녹음해야 합니다.`
        : hasBlockingMissing
          ? "누락 샘플을 확인하세요."
          : "미리듣기 가능";

  return (
    <section className="work-surface speak-surface">
      <div className="surface-row">
        <label className="text-input-wrap">
          <FieldLabel>음성 텍스트</FieldLabel>
          <textarea
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder="구보이스로 말할 문장을 입력하세요."
            rows={8}
          />
        </label>

        <div className="analysis-panel">
          <div className="analysis-head">
            <div>
              <FieldLabel>생성 가능 상태</FieldLabel>
              <strong>{isAnalyzing ? "검사 중" : `${analysis?.coverage ?? 0}%`}</strong>
            </div>
            <span className="pill">{analysis?.matched ?? 0}/{analysis?.required ?? targetCount}</span>
          </div>
          <div className="missing-list">
            {analysis?.missing.length ? (
              analysis.missing.map((item) => (
                <span className={`missing-chip ${item.severity}`} key={`${item.token}-${item.reason}`}>
                  {item.token}
                  <small>{item.reason}</small>
                </span>
              ))
            ) : (
              <span className="empty-line">현재 입력 기준으로 막는 항목이 없습니다.</span>
            )}
          </div>
          {!sourceComplete ? (
            <button className="inline-action" type="button" onClick={onGoRecord}>
              부족한 샘플 녹음
            </button>
          ) : null}
        </div>
      </div>

      <div className="option-grid">
        <SliderField
          label="속도"
          value={options.speed}
          min={0.65}
          max={1.35}
          step={0.05}
          suffix="x"
          onChange={(speed) => setOptions({ ...options, speed })}
        />
        <SliderField
          label="피치"
          value={options.pitch}
          min={-6}
          max={6}
          step={1}
          suffix=""
          onChange={(pitch) => setOptions({ ...options, pitch })}
        />
        <SliderField
          label="명료도"
          value={options.clarity}
          min={0}
          max={100}
          step={1}
          suffix="%"
          onChange={(clarity) => setOptions({ ...options, clarity })}
        />
        <SliderField
          label="잡음 억제"
          value={options.noiseReduction}
          min={0}
          max={100}
          step={1}
          suffix="%"
          onChange={(noiseReduction) => setOptions({ ...options, noiseReduction })}
        />
      </div>

      <div className="action-strip">
        <button className="primary" type="button" onClick={onPreview} disabled={!canPreview || isSynthesizing}>
          {isSynthesizing ? "생성 중" : "미리듣기"}
        </button>
        <button type="button" onClick={onExport} disabled={!canExport || isExporting}>
          {isExporting ? "저장 중" : "WAV 저장"}
        </button>
        <span className="button-state">{preview?.message ?? previewHint}</span>
      </div>

      {preview?.audioUrl ? (
        <audio className="preview-player" controls src={preview.audioUrl}>
          <track kind="captions" />
        </audio>
      ) : null}
    </section>
  );
}

function SliderField({
  label,
  value,
  min,
  max,
  step,
  suffix,
  onChange
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  suffix: string;
  onChange: (value: number) => void;
}) {
  return (
    <label className="slider-field">
      <span>
        {label}
        <strong>
          {value}
          {suffix}
        </strong>
      </span>
      <input
        type="range"
        value={value}
        min={min}
        max={max}
        step={step}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </label>
  );
}

function RecordTab({
  selectedSource,
  onEnsureSource,
  onAddSample,
  onDeleteSample
}: {
  selectedSource?: VoiceSource;
  onEnsureSource: () => Promise<VoiceSource | undefined>;
  onAddSample: (sourceId: string, sample: Omit<VoiceSample, "id" | "createdAt">) => Promise<void>;
  onDeleteSample: (sampleId: string) => void;
}) {
  const [selectedPromptId, setSelectedPromptId] = useState(SAMPLE_PROMPTS[0].id);
  const [label, setLabel] = useState(SAMPLE_PROMPTS[0].label);
  const [transcript, setTranscript] = useState(SAMPLE_PROMPTS[0].text);
  const [isRecording, setIsRecording] = useState(false);
  const [isPreparing, setIsPreparing] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  const [recorderError, setRecorderError] = useState("");
  const [recorderNotice, setRecorderNotice] = useState("앱 안에서 마이크 권한을 요청하고 바로 녹음합니다.");
  const [lastPreviewUrl, setLastPreviewUrl] = useState("");
  const audioContextRef = useRef<AudioContext | null>(null);
  const sourceNodeRef = useRef<MediaStreamAudioSourceNode | null>(null);
  const processorRef = useRef<ScriptProcessorNode | null>(null);
  const monitorGainRef = useRef<GainNode | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const pcmChunksRef = useRef<Float32Array[]>([]);
  const recordingSourceRef = useRef<VoiceSource | null>(null);
  const recordingPromptRef = useRef<SamplePrompt | null>(null);
  const recordingLabelRef = useRef("");
  const recordingTranscriptRef = useRef("");
  const startedAtRef = useRef(0);

  const selectedPrompt = SAMPLE_PROMPTS.find((prompt) => prompt.id === selectedPromptId) ?? SAMPLE_PROMPTS[0];
  const recordingSupported = Boolean(navigator.mediaDevices?.getUserMedia) && Boolean(getAudioContextConstructor());

  useEffect(() => {
    if (!isRecording) {
      return;
    }

    const timer = window.setInterval(() => {
      setElapsed((Date.now() - startedAtRef.current) / 1000);
    }, 200);

    return () => window.clearInterval(timer);
  }, [isRecording]);

  const applyPrompt = (prompt: SamplePrompt) => {
    setSelectedPromptId(prompt.id);
    setLabel(prompt.label);
    setTranscript(prompt.text);
  };

  const resolveSource = async () => {
    if (selectedSource) {
      return selectedSource;
    }
    setRecorderNotice("목소리 소스를 만들고 있습니다.");
    return onEnsureSource();
  };

  const cleanupRecording = () => {
    processorRef.current?.disconnect();
    if (processorRef.current) {
      processorRef.current.onaudioprocess = null;
    }
    sourceNodeRef.current?.disconnect();
    monitorGainRef.current?.disconnect();
    streamRef.current?.getTracks().forEach((track) => track.stop());
    void audioContextRef.current?.close();
    processorRef.current = null;
    sourceNodeRef.current = null;
    monitorGainRef.current = null;
    streamRef.current = null;
    audioContextRef.current = null;
  };

  const startRecording = async () => {
    setRecorderError("");
    setRecorderNotice("");
    setIsPreparing(true);

    try {
      const source = await resolveSource();
      if (!source) {
        setRecorderError("목소리 소스를 만들지 못했습니다.");
        return;
      }
      if (!recordingSupported) {
        setRecorderError("현재 WebView에서 마이크 녹음을 사용할 수 없습니다. WebView2와 마이크 권한을 확인하세요.");
        return;
      }

      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: false,
          noiseSuppression: false,
          autoGainControl: false
        }
      });
      const AudioContextConstructor = getAudioContextConstructor();
      if (!AudioContextConstructor) {
        throw new Error("Web Audio API를 사용할 수 없습니다.");
      }
      const audioContext = new AudioContextConstructor();
      const sourceNode = audioContext.createMediaStreamSource(stream);
      const processor = audioContext.createScriptProcessor(4096, Math.max(1, sourceNode.channelCount || 1), 1);
      const monitorGain = audioContext.createGain();
      monitorGain.gain.value = 0;

      processor.onaudioprocess = (event) => {
        const input = event.inputBuffer;
        const frameCount = input.length;
        const channelCount = input.numberOfChannels;
        const mono = new Float32Array(frameCount);
        for (let channel = 0; channel < channelCount; channel += 1) {
          const data = input.getChannelData(channel);
          for (let index = 0; index < frameCount; index += 1) {
            mono[index] += data[index] / channelCount;
          }
        }
        pcmChunksRef.current.push(mono);
      };

      sourceNode.connect(processor);
      processor.connect(monitorGain);
      monitorGain.connect(audioContext.destination);

      streamRef.current = stream;
      audioContextRef.current = audioContext;
      sourceNodeRef.current = sourceNode;
      processorRef.current = processor;
      monitorGainRef.current = monitorGain;
      pcmChunksRef.current = [];
      recordingSourceRef.current = source;
      recordingPromptRef.current = selectedPrompt;
      recordingLabelRef.current = label.trim() || selectedPrompt.label;
      recordingTranscriptRef.current = transcript.trim() || selectedPrompt.text;
      startedAtRef.current = Date.now();
      setElapsed(0);
      setLastPreviewUrl("");

      setIsRecording(true);
      setRecorderNotice("녹음 중입니다. 제시된 샘플을 읽은 뒤 정지를 누르세요.");
    } catch (error) {
      setRecorderError(error instanceof Error ? error.message : "마이크 권한을 확인하세요.");
      cleanupRecording();
      setIsRecording(false);
    } finally {
      setIsPreparing(false);
    }
  };

  const stopRecording = async () => {
    if (!isRecording) {
      return;
    }

    const source = recordingSourceRef.current;
    const prompt = recordingPromptRef.current ?? selectedPrompt;
    const chunks = pcmChunksRef.current;
    const sampleRate = audioContextRef.current?.sampleRate ?? 44100;
    const duration = Math.max(0.2, (Date.now() - startedAtRef.current) / 1000);
    cleanupRecording();
    pcmChunksRef.current = [];
    recordingSourceRef.current = null;
    recordingPromptRef.current = null;
    setIsRecording(false);

    if (!source) {
      setRecorderError("목소리 소스를 찾지 못했습니다.");
      return;
    }
    if (!chunks.length) {
      setRecorderError("녹음 데이터가 비어 있습니다. 마이크 입력을 확인하세요.");
      return;
    }

    try {
      const { dataUrl, previewUrl } = wavDataUrlFromPcm(chunks, sampleRate);
      setLastPreviewUrl(previewUrl);

      await onAddSample(source.id, {
        promptId: prompt.id,
        label: recordingLabelRef.current || prompt.label,
        text: recordingTranscriptRef.current || prompt.text,
        duration,
        origin: "recording",
        audioName: `recording-${prompt.id}.wav`,
        audioUrl: previewUrl,
        dataBase64: dataUrl
      });
      setRecorderNotice("WAV 녹음 샘플을 저장했습니다. 방금 녹음한 소리는 아래에서 확인할 수 있습니다.");
    } catch (error) {
      setRecorderError(error instanceof Error ? error.message : "녹음 WAV를 저장하지 못했습니다.");
    }
  };

  const handleUpload = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    if (!files.length) {
      return;
    }
    const source = await resolveSource();
    if (!source) {
      setRecorderError("목소리 소스를 만들지 못했습니다.");
      return;
    }
    setRecorderError("");
    try {
      for (const file of files) {
        const isWav = file.type === "audio/wav" || file.type === "audio/wave" || file.name.toLowerCase().endsWith(".wav");
        if (!isWav) {
          throw new Error("업로드 합성은 WAV/PCM 파일만 지원합니다. 녹음 파일을 WAV로 변환해서 다시 올려 주세요.");
        }
        await onAddSample(source.id, {
          promptId: selectedPrompt.id,
          label: label.trim() || selectedPrompt.label || file.name,
          text: transcript.trim() || selectedPrompt.text || file.name,
          duration: 0,
          origin: "upload",
          audioName: file.name,
          audioUrl: URL.createObjectURL(file),
          dataBase64: await blobToDataUrl(file)
        });
      }
    } catch (error) {
      setRecorderError(error instanceof Error ? error.message : "WAV 파일을 업로드하지 못했습니다.");
      event.target.value = "";
      return;
    }
    setRecorderNotice(`${files.length}개 파일을 현재 샘플 항목에 등록했습니다.`);
    event.target.value = "";
  };

  const progress = selectedSource ? progressOf(selectedSource) : 0;
  const statusText = isRecording ? `${elapsed.toFixed(1)}초 녹음 중` : isPreparing ? "마이크 준비 중" : "대기";

  return (
    <section className="record-grid">
      <div className="recorder-panel">
        <div className="section-head compact">
          <div>
            <FieldLabel>앱 내 직접 녹음</FieldLabel>
            <h2>{selectedSource?.name ?? "새 소스가 자동으로 만들어집니다"}</h2>
          </div>
          <span className="pill">
            필수 {filledCountOf(selectedSource)}/{clampTargetSamples(selectedSource?.targetSamples)}
          </span>
        </div>

        <ProgressBar value={progress} />

        <div className="record-main">
          <div className="current-prompt">
            <FieldLabel>읽을 샘플</FieldLabel>
            <strong>{selectedPrompt.label}</strong>
            <p>{selectedPrompt.text}</p>
          </div>

          <div className="sample-form compact-form">
            <label>
              <FieldLabel>샘플 이름</FieldLabel>
              <input value={label} onChange={(event) => setLabel(event.target.value)} />
            </label>
            <label>
              <FieldLabel>읽을 문장</FieldLabel>
              <textarea value={transcript} onChange={(event) => setTranscript(event.target.value)} rows={3} />
            </label>
          </div>

          <div className="recorder-controls">
            <button className="primary" type="button" onClick={startRecording} disabled={isRecording || isPreparing}>
              녹음 시작
            </button>
            <button type="button" onClick={stopRecording} disabled={!isRecording}>
              정지
            </button>
            <span className={`meter ${isRecording ? "is-live" : ""}`}>{statusText}</span>
          </div>

          {lastPreviewUrl ? (
            <audio className="preview-player" controls src={lastPreviewUrl}>
              <track kind="captions" />
            </audio>
          ) : null}
          {recorderError ? <p className="error-line">{recorderError}</p> : <p className="info-line">{recorderNotice}</p>}
        </div>
      </div>

      <div className="record-side">
        <div className="upload-panel">
          <div className="section-head compact">
            <div>
              <FieldLabel>프롬프트</FieldLabel>
              <h2>녹음할 항목</h2>
            </div>
            <label className="file-button">
              파일 업로드
              <input type="file" accept=".wav,audio/wav,audio/wave,audio/x-wav" multiple onChange={handleUpload} />
            </label>
          </div>
          <div className="prompt-grid">
            {SAMPLE_PROMPTS.map((prompt) => (
              <button
                className={prompt.id === selectedPromptId ? "is-active" : ""}
                key={prompt.id}
                type="button"
                onClick={() => applyPrompt(prompt)}
              >
                <span>{prompt.label}</span>
                <small>{prompt.text}</small>
              </button>
            ))}
          </div>
        </div>

        <div className="upload-panel samples-panel">
          <div className="section-head compact">
            <div>
              <FieldLabel>저장됨</FieldLabel>
              <h2>샘플 목록</h2>
            </div>
          </div>
          <div className="sample-list">
            {selectedSource?.samples.length ? (
              selectedSource.samples.map((sample) => (
                <div className="sample-row" key={sample.id}>
                  <div>
                    <strong>{sample.label}</strong>
                    <span>
                      {sample.origin === "recording" ? "녹음" : "업로드"} · {sample.audioName ?? "오디오"} ·{" "}
                      {formatDate(sample.createdAt)}
                    </span>
                  </div>
                  <button type="button" onClick={() => onDeleteSample(sample.id)} title="샘플 삭제">
                    ×
                  </button>
                </div>
              ))
            ) : (
              <span className="empty-line">아직 저장된 샘플이 없습니다.</span>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}

function ManageTab({
  selectedSource,
  onCreate,
  onUpdate,
  onDelete
}: {
  selectedSource?: VoiceSource;
  onCreate: (input: { name: string; speaker: string; note: string; targetSamples: number }) => Promise<VoiceSource>;
  onUpdate: (patch: Partial<VoiceSource>) => void;
  onDelete: () => void;
}) {
  const [newName, setNewName] = useState("");
  const [newSpeaker, setNewSpeaker] = useState("");
  const [newNote, setNewNote] = useState("");
  const [newTarget, setNewTarget] = useState(MIN_SAMPLE_TARGET);
  const [editName, setEditName] = useState("");
  const [editSpeaker, setEditSpeaker] = useState("");
  const [editNote, setEditNote] = useState("");
  const [editTarget, setEditTarget] = useState(MIN_SAMPLE_TARGET);

  useEffect(() => {
    setEditName(selectedSource?.name ?? "");
    setEditSpeaker(selectedSource?.speaker ?? "");
    setEditNote(selectedSource?.note ?? "");
    setEditTarget(clampTargetSamples(selectedSource?.targetSamples));
  }, [selectedSource]);

  const submitCreate = async (event: FormEvent) => {
    event.preventDefault();
    await onCreate({
      name: newName,
      speaker: newSpeaker,
      note: newNote,
      targetSamples: clampTargetSamples(newTarget)
    });
    setNewName("");
    setNewSpeaker("");
    setNewNote("");
    setNewTarget(MIN_SAMPLE_TARGET);
  };

  const submitUpdate = (event: FormEvent) => {
    event.preventDefault();
    onUpdate({
      name: editName,
      speaker: editSpeaker,
      note: editNote,
      targetSamples: clampTargetSamples(editTarget)
    });
  };

  return (
    <section className="manage-grid">
      <form className="manage-form" onSubmit={submitCreate}>
        <div className="section-head compact">
          <div>
            <FieldLabel>생성</FieldLabel>
            <h2>새 목소리 소스</h2>
          </div>
          <button className="primary" type="submit">
            생성
          </button>
        </div>
        <label>
          <FieldLabel>소스 이름</FieldLabel>
          <input value={newName} onChange={(event) => setNewName(event.target.value)} placeholder="예: 내 안내 목소리" />
        </label>
        <label>
          <FieldLabel>화자</FieldLabel>
          <input value={newSpeaker} onChange={(event) => setNewSpeaker(event.target.value)} placeholder="예: 나" />
        </label>
        <label>
          <FieldLabel>메모</FieldLabel>
          <textarea value={newNote} onChange={(event) => setNewNote(event.target.value)} rows={3} />
        </label>
        <label>
          <FieldLabel>목표 샘플 수</FieldLabel>
          <input
            type="number"
            min={MIN_SAMPLE_TARGET}
            max={MAX_SAMPLE_TARGET}
            value={newTarget}
            onChange={(event) => setNewTarget(clampTargetSamples(Number(event.target.value)))}
          />
        </label>
      </form>

      <form className="manage-form" onSubmit={submitUpdate}>
        <div className="section-head compact">
          <div>
            <FieldLabel>편집</FieldLabel>
            <h2>{selectedSource?.name ?? "선택된 소스 없음"}</h2>
          </div>
          <button type="submit" disabled={!selectedSource}>
            저장
          </button>
        </div>
        <label>
          <FieldLabel>소스 이름</FieldLabel>
          <input value={editName} onChange={(event) => setEditName(event.target.value)} disabled={!selectedSource} />
        </label>
        <label>
          <FieldLabel>화자</FieldLabel>
          <input value={editSpeaker} onChange={(event) => setEditSpeaker(event.target.value)} disabled={!selectedSource} />
        </label>
        <label>
          <FieldLabel>메모</FieldLabel>
          <textarea value={editNote} onChange={(event) => setEditNote(event.target.value)} rows={3} disabled={!selectedSource} />
        </label>
        <label>
          <FieldLabel>목표 샘플 수</FieldLabel>
          <input
            type="number"
            min={MIN_SAMPLE_TARGET}
            max={MAX_SAMPLE_TARGET}
            value={editTarget}
            onChange={(event) => setEditTarget(clampTargetSamples(Number(event.target.value)))}
            disabled={!selectedSource}
          />
        </label>
        <button className="danger" type="button" onClick={onDelete} disabled={!selectedSource}>
          소스 삭제
        </button>
      </form>
    </section>
  );
}

function StatusPanel({
  selectedSource,
  analysis,
  engineStatus,
  preview
}: {
  selectedSource?: VoiceSource;
  analysis: AnalysisResult | null;
  engineStatus: EngineStatus | null;
  preview: PreviewResult | null;
}) {
  const progress = selectedSource ? progressOf(selectedSource) : 0;
  const sourceComplete = Boolean(selectedSource && filledCountOf(selectedSource) >= clampTargetSamples(selectedSource.targetSamples));
  const coveredPromptLabels = new Set(selectedSource?.samples.map((sample) => sample.label));
  const coveredPromptIds = new Set(selectedSource?.samples.map((sample) => sample.promptId ?? sample.label));
  const nextPrompts = SAMPLE_PROMPTS.filter(
    (prompt) => !coveredPromptIds.has(prompt.id) && !coveredPromptLabels.has(prompt.label)
  ).slice(0, 4);

  return (
    <aside className="status-panel">
      <div className="status-block">
        <FieldLabel>엔진</FieldLabel>
        <div className="status-line">
          <span className={`status-dot ${engineStatus?.ready ? "ready" : ""}`} />
          <strong>{engineStatus?.label ?? "확인 중"}</strong>
        </div>
        <p>{engineStatus?.message ?? "상태를 불러오는 중입니다."}</p>
      </div>

      <div className="status-block">
        <FieldLabel>선택 소스</FieldLabel>
        <h2>{selectedSource?.name ?? "없음"}</h2>
        <dl className="metric-list">
          <div>
            <dt>화자</dt>
            <dd>{selectedSource?.speaker ?? "-"}</dd>
          </div>
          <div>
            <dt>샘플</dt>
            <dd>
              {filledCountOf(selectedSource)}/{clampTargetSamples(selectedSource?.targetSamples)}
            </dd>
          </div>
          <div>
            <dt>커버</dt>
            <dd>{analysis?.coverage ?? progress}%</dd>
          </div>
        </dl>
        <ProgressBar value={progress} />
      </div>

      <div className="status-block compact-status">
        <FieldLabel>생성 상태</FieldLabel>
        <dl className="metric-list">
          <div>
            <dt>미리듣기</dt>
            <dd>{sourceComplete ? "가능" : "대기"}</dd>
          </div>
          <div>
            <dt>WAV 저장</dt>
            <dd>{preview?.status === "ready" ? "가능" : "대기"}</dd>
          </div>
        </dl>
      </div>

        <div className="status-block next-block">
        <FieldLabel>다음 필수 샘플</FieldLabel>
        <div className="next-list">
          {nextPrompts.length ? (
            nextPrompts.map((prompt) => (
              <div key={prompt.id}>
                <strong>{prompt.label}</strong>
                <span>{prompt.text}</span>
              </div>
            ))
          ) : (
            <span className="empty-line">권장 샘플 완료</span>
          )}
        </div>
      </div>
    </aside>
  );
}

export default function App() {
  const [sources, setSources] = useState<VoiceSource[]>([]);
  const [selectedSourceId, setSelectedSourceId] = useState("");
  const [activeTab, setActiveTab] = useState<TabId>("speak");
  const [text, setText] = useState(defaultSpeakText);
  const [analysis, setAnalysis] = useState<AnalysisResult | null>(null);
  const [isAnalyzing, setIsAnalyzing] = useState(false);
  const [options, setOptions] = useState<SynthesisOptions>(initialOptions);
  const [preview, setPreview] = useState<PreviewResult | null>(null);
  const [isSynthesizing, setIsSynthesizing] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [engineStatus, setEngineStatus] = useState<EngineStatus | null>(null);
  const [notice, setNotice] = useState("");

  const selectedSource = useMemo(
    () => sources.find((source) => source.id === selectedSourceId),
    [sources, selectedSourceId]
  );

  const refreshSources = async (preferredId?: string) => {
    const nextSources = await voiceApi.listSources();
    setSources(nextSources);
    const nextSelectedId = preferredId || selectedSourceId || nextSources[0]?.id || "";
    setSelectedSourceId(nextSources.some((source) => source.id === nextSelectedId) ? nextSelectedId : nextSources[0]?.id || "");
  };

  useEffect(() => {
    refreshSources();
    voiceApi.getEngineStatus().then(setEngineStatus);
  }, []);

  useEffect(() => {
    setPreview(null);
  }, [selectedSourceId, text, options]);

  useEffect(() => {
    if (!selectedSource || !text.trim()) {
      setAnalysis(null);
      return;
    }

    let alive = true;
    setIsAnalyzing(true);
    const timer = window.setTimeout(() => {
      voiceApi
        .analyzeText(selectedSource.id, text)
        .then((result) => {
          if (alive) {
            setAnalysis(result);
          }
        })
        .finally(() => {
          if (alive) {
            setIsAnalyzing(false);
          }
        });
    }, 200);

    return () => {
      alive = false;
      window.clearTimeout(timer);
    };
  }, [selectedSource?.id, selectedSource?.updatedAt, text]);

  const selectSource = (id: string) => {
    setSelectedSourceId(id);
    setNotice("");
  };

  const handleCreateSource = async (input: {
    name: string;
    speaker: string;
    note: string;
    targetSamples: number;
  }) => {
    const created = await voiceApi.createSource({
      name: input.name.trim() || "새 목소리",
      speaker: input.speaker.trim() || "나",
      note: input.note.trim(),
      targetSamples: clampTargetSamples(input.targetSamples)
    });
    await refreshSources(created.id);
    setActiveTab("record");
    setNotice("새 목소리 소스를 만들었습니다. 녹음 탭에서 바로 샘플을 채울 수 있습니다.");
    return created;
  };

  const ensureRecordSource = async () => {
    if (selectedSource) {
      return selectedSource;
    }
    return handleCreateSource({
      name: "내 목소리",
      speaker: "나",
      note: "앱에서 직접 녹음한 목소리 소스",
      targetSamples: MIN_SAMPLE_TARGET
    });
  };

  const handleUpdateSource = async (patch: Partial<VoiceSource>) => {
    if (!selectedSource) {
      return;
    }
    const updated = await voiceApi.updateSource(selectedSource.id, patch);
    await refreshSources(updated.id);
    setNotice("소스 정보를 저장했습니다.");
  };

  const handleDeleteSource = async () => {
    if (!selectedSource) {
      return;
    }
    await voiceApi.deleteSource(selectedSource.id);
    await refreshSources();
    setNotice("소스를 삭제했습니다.");
  };

  const handleAddSample = async (sourceId: string, sample: Omit<VoiceSample, "id" | "createdAt">) => {
    const updated = await voiceApi.addSample(sourceId, sample);
    await refreshSources(updated.id);
    setNotice("샘플을 저장했습니다.");
  };

  const handleDeleteSample = async (sampleId: string) => {
    if (!selectedSource) {
      return;
    }
    const nextSamples = selectedSource.samples.filter((sample) => sample.id !== sampleId);
    const updated = await voiceApi.updateSource(selectedSource.id, { samples: nextSamples });
    await refreshSources(updated.id);
    setNotice("샘플을 삭제했습니다.");
  };

  const handlePreview = async () => {
    if (!selectedSource || !text.trim()) {
      return;
    }
    setIsSynthesizing(true);
    try {
      const result = await voiceApi.synthesize({
        sourceId: selectedSource.id,
        text,
        options
      });
      setPreview(result);
      setNotice(result.message);
    } finally {
      setIsSynthesizing(false);
    }
  };

  const handleExport = async () => {
    if (!selectedSource || !text.trim()) {
      return;
    }
    setIsExporting(true);
    try {
      const result = await voiceApi.exportMP3({
        sourceId: selectedSource.id,
        text,
        options
      });
      if (result.downloadUrl) {
        const anchor = document.createElement("a");
        anchor.href = result.downloadUrl;
        anchor.download = `${selectedSource.name.replace(/\s+/g, "-")}-guvoice.wav`;
        anchor.click();
      }
      setNotice(result.path ? `${result.message}: ${result.path}` : result.message);
    } finally {
      setIsExporting(false);
    }
  };

  return (
    <div className="app-shell">
      <SourceList
        sources={sources}
        selectedId={selectedSourceId}
        onSelect={selectSource}
        onCreate={() => setActiveTab("record")}
      />

      <main className="main-panel">
        <div className="main-topbar">
          <Tabs activeTab={activeTab} onChange={setActiveTab} />
          <div className="topbar-status">
            <span className={`status-dot ${engineStatus?.ready ? "ready" : ""}`} />
            {engineStatus?.mode === "wails" ? "Wails" : "Browser"}
          </div>
        </div>

        {notice ? <div className="notice">{notice}</div> : null}

        <div className="tab-body">
          {activeTab === "speak" ? (
            <SpeakTab
              selectedSource={selectedSource}
              text={text}
              setText={setText}
              analysis={analysis}
              isAnalyzing={isAnalyzing}
              options={options}
              setOptions={setOptions}
              preview={preview}
              isSynthesizing={isSynthesizing}
              isExporting={isExporting}
              onPreview={handlePreview}
              onExport={handleExport}
              onGoRecord={() => setActiveTab("record")}
            />
          ) : null}

          {activeTab === "record" ? (
            <RecordTab
              selectedSource={selectedSource}
              onEnsureSource={ensureRecordSource}
              onAddSample={handleAddSample}
              onDeleteSample={handleDeleteSample}
            />
          ) : null}

          {activeTab === "manage" ? (
            <ManageTab
              selectedSource={selectedSource}
              onCreate={handleCreateSource}
              onUpdate={handleUpdateSource}
              onDelete={handleDeleteSource}
            />
          ) : null}
        </div>
      </main>

      <StatusPanel selectedSource={selectedSource} analysis={analysis} engineStatus={engineStatus} preview={preview} />
    </div>
  );
}

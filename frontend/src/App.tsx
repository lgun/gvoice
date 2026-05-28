import { ChangeEvent, FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { voiceApi } from "./lib/adapter";
import {
  AnalysisResult,
  EngineStatus,
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

const formatDate = (value: string) =>
  new Intl.DateTimeFormat("ko-KR", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));

const progressOf = (source: VoiceSource) =>
  Math.min(100, Math.round((source.samples.length / source.targetSamples) * 100));

const blobToDataUrl = (blob: Blob) =>
  new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error ?? new Error("오디오를 읽지 못했습니다."));
    reader.readAsDataURL(blob);
  });

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
                {source.speaker} · 샘플 {source.samples.length}/{source.targetSamples}
              </span>
              <ProgressBar value={progress} />
            </button>
          );
        })}
      </div>
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
  onExport
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
}) {
  const sampleCount = selectedSource?.samples.length ?? 0;
  const targetCount = selectedSource?.targetSamples ?? MIN_SAMPLE_TARGET;
  const hasBlockingMissing = Boolean(analysis?.missing.some((item) => item.severity === "missing"));
  const sourceComplete = Boolean(selectedSource && sampleCount >= targetCount);
  const canPreview = Boolean(selectedSource && text.trim() && sourceComplete && !hasBlockingMissing);
  const canExport = Boolean(preview?.audioUrl || preview?.status === "ready");

  const previewHint = !selectedSource
    ? "소스 선택 필요"
    : !text.trim()
      ? "텍스트 필요"
      : sampleCount < targetCount
        ? `샘플 ${targetCount - sampleCount}개 더 필요`
        : hasBlockingMissing
          ? "누락 샘플 확인 필요"
        : "미리듣기 가능";

  return (
    <section className="work-surface">
      <div className="surface-row">
        <label className="text-input-wrap">
          <FieldLabel>합성 텍스트</FieldLabel>
          <textarea
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder="합성할 한국어 문장을 입력하세요."
            rows={9}
          />
        </label>

        <div className="analysis-panel">
          <div className="analysis-head">
            <div>
              <FieldLabel>누락 샘플 검사</FieldLabel>
              <strong>{isAnalyzing ? "검사 중" : `${analysis?.coverage ?? 0}% 커버`}</strong>
            </div>
            <span className="pill">{analysis?.matched ?? 0}/{analysis?.required ?? 0}</span>
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
              <span className="empty-line">입력 텍스트 기준 누락 없음</span>
            )}
          </div>
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
          label="노이즈 억제"
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
          <span className="btn-icon">▶</span>
          {isSynthesizing ? "생성 중" : "미리듣기"}
        </button>
        <button type="button" onClick={onExport} disabled={!canExport || isExporting}>
          <span className="btn-icon">↓</span>
          {isExporting ? "저장 중" : "MP3 저장"}
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
  onAddSample,
  onDeleteSample
}: {
  selectedSource?: VoiceSource;
  onAddSample: (sample: Omit<VoiceSample, "id" | "createdAt">) => Promise<void>;
  onDeleteSample: (sampleId: string) => void;
}) {
  const [selectedPromptId, setSelectedPromptId] = useState(SAMPLE_PROMPTS[0].id);
  const [label, setLabel] = useState(SAMPLE_PROMPTS[0].label);
  const [transcript, setTranscript] = useState(SAMPLE_PROMPTS[0].text);
  const [isRecording, setIsRecording] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  const [recorderError, setRecorderError] = useState("");
  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const startedAtRef = useRef(0);

  const selectedPrompt = SAMPLE_PROMPTS.find((prompt) => prompt.id === selectedPromptId);

  useEffect(() => {
    if (!isRecording) {
      return;
    }

    const timer = window.setInterval(() => {
      setElapsed((Date.now() - startedAtRef.current) / 1000);
    }, 250);

    return () => window.clearInterval(timer);
  }, [isRecording]);

  const applyPrompt = (prompt: SamplePrompt) => {
    setSelectedPromptId(prompt.id);
    setLabel(prompt.label);
    setTranscript(prompt.text);
  };

  const startRecording = async () => {
    setRecorderError("");
    if (!selectedSource) {
      setRecorderError("소스를 먼저 선택하세요.");
      return;
    }
    if (!navigator.mediaDevices?.getUserMedia) {
      setRecorderError("현재 브라우저에서 녹음을 사용할 수 없습니다.");
      return;
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream);
      chunksRef.current = [];
      recorderRef.current = recorder;
      startedAtRef.current = Date.now();
      setElapsed(0);

      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };

      recorder.onstop = async () => {
        const blob = new Blob(chunksRef.current, { type: recorder.mimeType || "audio/webm" });
        stream.getTracks().forEach((track) => track.stop());
        setIsRecording(false);
        const duration = Math.max(1, (Date.now() - startedAtRef.current) / 1000);
        const audioUrl = URL.createObjectURL(blob);
        await onAddSample({
          promptId: selectedPrompt?.id,
          label: label.trim() || selectedPrompt?.label || "녹음 샘플",
          text: transcript.trim() || selectedPrompt?.text || "",
          duration,
          origin: "recording",
          audioName: "browser-recording.webm",
          audioUrl,
          dataBase64: await blobToDataUrl(blob)
        });
      };

      recorder.start();
      setIsRecording(true);
    } catch {
      setRecorderError("마이크 권한을 확인하세요.");
    }
  };

  const stopRecording = () => {
    recorderRef.current?.stop();
  };

  const handleUpload = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    for (const file of files) {
      await onAddSample({
        promptId: selectedPrompt?.id,
        label: label.trim() || file.name,
        text: transcript.trim() || file.name,
        duration: 0,
        origin: "upload",
        audioName: file.name,
        audioUrl: URL.createObjectURL(file),
        dataBase64: await blobToDataUrl(file)
      });
    }
    event.target.value = "";
  };

  const progress = selectedSource ? progressOf(selectedSource) : 0;

  return (
    <section className="record-grid">
      <div className="recorder-panel">
        <div className="section-head">
          <div>
            <FieldLabel>직접 녹음</FieldLabel>
            <h2>{selectedSource?.name ?? "소스 없음"}</h2>
          </div>
          <span className="pill">샘플 {selectedSource?.samples.length ?? 0}/{selectedSource?.targetSamples ?? MIN_SAMPLE_TARGET}</span>
        </div>

        <ProgressBar value={progress} />

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

        <div className="sample-form">
          <label>
            <FieldLabel>샘플 이름</FieldLabel>
            <input value={label} onChange={(event) => setLabel(event.target.value)} />
          </label>
          <label>
            <FieldLabel>읽은 문장</FieldLabel>
            <textarea value={transcript} onChange={(event) => setTranscript(event.target.value)} rows={4} />
          </label>
        </div>

        <div className="recorder-controls">
          <button className="primary" type="button" onClick={startRecording} disabled={isRecording || !selectedSource}>
            <span className="btn-icon">●</span>
            녹음 시작
          </button>
          <button type="button" onClick={stopRecording} disabled={!isRecording}>
            <span className="btn-icon">■</span>
            정지
          </button>
          <span className="meter">{isRecording ? `${elapsed.toFixed(1)}초` : "대기"}</span>
        </div>
        {recorderError ? <p className="error-line">{recorderError}</p> : null}
      </div>

      <div className="upload-panel">
        <div className="section-head">
          <div>
            <FieldLabel>업로드</FieldLabel>
            <h2>파일 샘플</h2>
          </div>
          <label className="file-button">
            <span className="btn-icon">↑</span>
            오디오 선택
            <input type="file" accept="audio/*" multiple onChange={handleUpload} disabled={!selectedSource} />
          </label>
        </div>

        <div className="sample-list">
          {selectedSource?.samples.length ? (
            selectedSource.samples.map((sample) => (
              <div className="sample-row" key={sample.id}>
                <div>
                  <strong>{sample.label}</strong>
                  <span>
                    {sample.origin === "recording" ? "녹음" : "업로드"} · {sample.audioName ?? "메타데이터"} ·{" "}
                    {formatDate(sample.createdAt)}
                  </span>
                </div>
                <button type="button" onClick={() => onDeleteSample(sample.id)} title="샘플 삭제">
                  ×
                </button>
              </div>
            ))
          ) : (
            <span className="empty-line">저장된 샘플 없음</span>
          )}
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
  onCreate: (input: { name: string; speaker: string; note: string; targetSamples: number }) => void;
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
    setEditTarget(selectedSource?.targetSamples ?? MIN_SAMPLE_TARGET);
  }, [selectedSource]);

  const submitCreate = (event: FormEvent) => {
    event.preventDefault();
    onCreate({
      name: newName,
      speaker: newSpeaker,
      note: newNote,
      targetSamples: newTarget
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
      targetSamples: editTarget
    });
  };

  return (
    <section className="manage-grid">
      <form className="manage-form" onSubmit={submitCreate}>
        <div className="section-head">
          <div>
            <FieldLabel>생성</FieldLabel>
            <h2>새 목소리 소스</h2>
          </div>
          <button className="primary" type="submit">
            <span className="btn-icon">+</span>
            생성
          </button>
        </div>
        <label>
          <FieldLabel>소스 이름</FieldLabel>
          <input value={newName} onChange={(event) => setNewName(event.target.value)} placeholder="예: 상담 안내 목소리" />
        </label>
        <label>
          <FieldLabel>화자</FieldLabel>
          <input value={newSpeaker} onChange={(event) => setNewSpeaker(event.target.value)} placeholder="예: 김구니" />
        </label>
        <label>
          <FieldLabel>메모</FieldLabel>
          <textarea value={newNote} onChange={(event) => setNewNote(event.target.value)} rows={4} />
        </label>
        <label>
          <FieldLabel>목표 샘플 수</FieldLabel>
          <input
            type="number"
            min={3}
            max={80}
            value={newTarget}
            onChange={(event) => setNewTarget(Number(event.target.value))}
          />
        </label>
      </form>

      <form className="manage-form" onSubmit={submitUpdate}>
        <div className="section-head">
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
          <textarea value={editNote} onChange={(event) => setEditNote(event.target.value)} rows={4} disabled={!selectedSource} />
        </label>
        <label>
          <FieldLabel>목표 샘플 수</FieldLabel>
          <input
            type="number"
            min={3}
            max={80}
            value={editTarget}
            onChange={(event) => setEditTarget(Number(event.target.value))}
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
  const sourceComplete = Boolean(selectedSource && selectedSource.samples.length >= selectedSource.targetSamples);
  const coveredPromptLabels = new Set(selectedSource?.samples.map((sample) => sample.label));
  const coveredPromptIds = new Set(selectedSource?.samples.map((sample) => sample.promptId ?? sample.label));
  const nextPrompts = SAMPLE_PROMPTS.filter(
    (prompt) => !coveredPromptIds.has(prompt.id) && !coveredPromptLabels.has(prompt.label)
  ).slice(0, 5);

  return (
    <aside className="status-panel">
      <div className="status-block">
        <FieldLabel>엔진</FieldLabel>
        <div className="status-line">
          <span className={`status-dot ${engineStatus?.ready ? "ready" : ""}`} />
          <strong>{engineStatus?.label ?? "확인 중"}</strong>
        </div>
        <p>{engineStatus?.message ?? "상태를 불러오는 중"}</p>
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
              {selectedSource?.samples.length ?? 0}/{selectedSource?.targetSamples ?? MIN_SAMPLE_TARGET}
            </dd>
          </div>
          <div>
            <dt>커버</dt>
            <dd>{analysis?.coverage ?? 0}%</dd>
          </div>
        </dl>
        <ProgressBar value={progress} />
      </div>

      <div className="status-block">
        <FieldLabel>버튼 상태</FieldLabel>
        <dl className="metric-list">
          <div>
            <dt>미리듣기</dt>
            <dd>{sourceComplete ? "가능" : "대기"}</dd>
          </div>
          <div>
            <dt>MP3 저장</dt>
            <dd>{preview?.status === "ready" ? "가능" : "대기"}</dd>
          </div>
        </dl>
      </div>

      <div className="status-block">
        <FieldLabel>다음 샘플</FieldLabel>
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
  const [text, setText] = useState("안녕하세요. 오늘은 구보이스 목소리 소스를 테스트합니다.");
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
    }, 280);

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
    const created = await voiceApi.createSource(input);
    await refreshSources(created.id);
    setActiveTab("record");
    setNotice("새 목소리 소스를 만들었습니다.");
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

  const handleAddSample = async (sample: Omit<VoiceSample, "id" | "createdAt">) => {
    if (!selectedSource) {
      return;
    }
    const updated = await voiceApi.addSample(selectedSource.id, sample);
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
        anchor.download = `${selectedSource.name.replace(/\s+/g, "-")}-guvoice.mp3`;
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
        onCreate={() => setActiveTab("manage")}
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
          />
        ) : null}

        {activeTab === "record" ? (
          <RecordTab selectedSource={selectedSource} onAddSample={handleAddSample} onDeleteSample={handleDeleteSample} />
        ) : null}

        {activeTab === "manage" ? (
          <ManageTab
            selectedSource={selectedSource}
            onCreate={handleCreateSource}
            onUpdate={handleUpdateSource}
            onDelete={handleDeleteSource}
          />
        ) : null}
      </main>

      <StatusPanel selectedSource={selectedSource} analysis={analysis} engineStatus={engineStatus} preview={preview} />
    </div>
  );
}

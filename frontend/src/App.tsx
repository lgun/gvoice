import { ChangeEvent, FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { voiceApi } from "./lib/adapter";
import { decodeAudioFileToMonoWav, wavDataUrlFromPcm } from "./lib/audio";
import {
  AnalysisResult,
  EngineStatus,
  MIN_SAMPLE_TARGET,
  OutputDirectorySettings,
  PreviewResult,
  SAMPLE_PROMPTS,
  SAMPLE_TARGET_OPTIONS,
  SentencePrompt,
  SentenceSampleCandidate,
  SpeechItem,
  SpeechLibrarySettings,
  SamplePrompt,
  SynthesisOptions,
  TabId,
  VoiceSample,
  VoiceSource,
  normalizeTargetSamples
} from "./types";

const initialOptions: SynthesisOptions = {
  speed: 1,
  pitch: 0,
  clarity: 72,
  noiseReduction: 48
};

const defaultSpeakText = "안녕하세요. 오늘은 구보이스 목소리 소스를 테스트합니다.";

const targetOptionValue = normalizeTargetSamples;

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

const formatDuration = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "-";
  }
  const minutes = Math.floor(seconds / 60);
  const rest = Math.round(seconds % 60)
    .toString()
    .padStart(2, "0");
  return `${minutes}:${rest}`;
};

const requiredPromptsFor = (source?: VoiceSource) =>
  SAMPLE_PROMPTS.slice(0, normalizeTargetSamples(source?.targetSamples));

const filledPromptIdsOf = (source?: VoiceSource) =>
  new Set(source?.samples.map((sample) => sample.promptId ?? sample.label) ?? []);

const filledCountOf = (source?: VoiceSource) => {
  const required = requiredPromptsFor(source);
  const filledIds = filledPromptIdsOf(source);
  return required.filter((prompt) => filledIds.has(prompt.id)).length;
};

const progressOf = (source: VoiceSource) =>
  Math.min(100, Math.round((filledCountOf(source) / normalizeTargetSamples(source.targetSamples)) * 100));

type RecordingMode = "single" | "sentence";

const hardBlockingCandidateStatuses = ["reject", "rejected", "failed", "error", "empty", "silence"];
const autoBlockedCandidateStatuses = [...hardBlockingCandidateStatuses, "review", "warning", "warn"];
const reviewCandidateStatuses = ["review", "warning", "warn"];
const usefulCandidateStatuses = ["ready", "usable", "good", "ok", "accepted"];
const minManualCandidateConfidence = 0.15;
const minAutoCandidateConfidence = 0.75;
const minCandidateDuration = 0.08;

const normalizedCandidateStatus = (candidate: SentenceSampleCandidate) =>
  (candidate.status ?? "").trim().toLowerCase();

const hasCandidateStatus = (candidate: SentenceSampleCandidate, statuses: string[]) => {
  const status = normalizedCandidateStatus(candidate);
  return statuses.some((item) => status === item || status.includes(item));
};

const hasHardBlockingCandidateStatus = (candidate: SentenceSampleCandidate) =>
  hasCandidateStatus(candidate, hardBlockingCandidateStatuses);

const hasAutoBlockedCandidateStatus = (candidate: SentenceSampleCandidate) =>
  hasCandidateStatus(candidate, autoBlockedCandidateStatuses);

const hasReviewCandidateStatus = (candidate: SentenceSampleCandidate) =>
  hasCandidateStatus(candidate, reviewCandidateStatuses);

const hasUsefulCandidateStatus = (candidate: SentenceSampleCandidate) =>
  hasCandidateStatus(candidate, usefulCandidateStatuses);

type WindowWithWebkitAudio = Window & {
  webkitAudioContext?: typeof AudioContext;
};

const getAudioContextConstructor = () =>
  window.AudioContext ?? (window as WindowWithWebkitAudio).webkitAudioContext;

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
          <h1>목소리 프리셋</h1>
        </div>
        <button className="icon-button" type="button" onClick={onCreate} title="새 프리셋">
          +
        </button>
      </div>

      {sources.length === 0 ? (
        <button className="source-empty" type="button" onClick={onCreate}>
          <strong>새 프리셋 만들기</strong>
          <span>생성 화면에서 녹음 타입을 고른 뒤 시작합니다.</span>
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
                  {source.speaker} · 필수 {filledCountOf(source)}/{normalizeTargetSamples(source.targetSamples)}
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
    { id: "library", label: "보관함" },
    { id: "manage", label: "프리셋 관리" }
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
  isSavingToLibrary,
  onPreview,
  onExport,
  onSaveToLibrary,
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
  isSavingToLibrary: boolean;
  onPreview: () => void;
  onExport: () => void;
  onSaveToLibrary: () => void;
  onGoRecord: () => void;
}) {
  const sampleCount = filledCountOf(selectedSource);
  const targetCount = normalizeTargetSamples(selectedSource?.targetSamples);
  const hasBlockingMissing = Boolean(analysis?.missing.some((item) => item.severity === "missing"));
  const sourceComplete = Boolean(selectedSource && sampleCount >= targetCount);
  const canPreview = Boolean(selectedSource && text.trim() && sourceComplete && !hasBlockingMissing);
  const canExport = canPreview && Boolean(preview?.audioUrl || preview?.status === "ready");
  const canSaveToLibrary = canPreview;

  const previewHint = !selectedSource
    ? "목소리 프리셋을 만들거나 선택하세요."
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
          max={5}
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
          {isExporting ? "저장 중" : "MP3 저장"}
        </button>
        <button type="button" onClick={onSaveToLibrary} disabled={!canSaveToLibrary || isSavingToLibrary}>
          {isSavingToLibrary ? "보관 중" : "보관함 저장"}
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

function SampleTargetPicker({
  value,
  onChange,
  disabled = false
}: {
  value: number;
  onChange: (value: number) => void;
  disabled?: boolean;
}) {
  return (
    <div className="target-picker" role="radiogroup" aria-label="녹음 타입">
      {SAMPLE_TARGET_OPTIONS.map((option) => (
        <button
          className={value === option.value ? "is-active" : ""}
          key={option.value}
          type="button"
          role="radio"
          aria-checked={value === option.value}
          onClick={() => onChange(option.value)}
          disabled={disabled}
        >
          <span>
            <strong>{option.label}</strong>
            <em>{option.value}개</em>
          </span>
          <small>{option.description}</small>
        </button>
      ))}
    </div>
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
  const [recordMode, setRecordMode] = useState<RecordingMode>("single");
  const [isRecording, setIsRecording] = useState(false);
  const [isPreparing, setIsPreparing] = useState(false);
  const [isExtractingSentence, setIsExtractingSentence] = useState(false);
  const [isSavingSentenceCandidates, setIsSavingSentenceCandidates] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  const [recorderError, setRecorderError] = useState("");
  const [recorderNotice, setRecorderNotice] = useState("앱 안에서 마이크 권한을 요청하고 바로 녹음합니다.");
  const [lastPreviewUrl, setLastPreviewUrl] = useState("");
  const [sentencePrompts, setSentencePrompts] = useState<SentencePrompt[]>([]);
  const [selectedSentencePromptId, setSelectedSentencePromptId] = useState("");
  const [sentenceText, setSentenceText] = useState("");
  const [sentenceCandidates, setSentenceCandidates] = useState<SentenceSampleCandidate[]>([]);
  const [sentenceWarnings, setSentenceWarnings] = useState<string[]>([]);
  const [sentenceRecordingUrl, setSentenceRecordingUrl] = useState("");
  const [savedCandidateIds, setSavedCandidateIds] = useState<Set<string>>(() => new Set());
  const audioContextRef = useRef<AudioContext | null>(null);
  const sourceNodeRef = useRef<MediaStreamAudioSourceNode | null>(null);
  const processorRef = useRef<ScriptProcessorNode | null>(null);
  const monitorGainRef = useRef<GainNode | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const pcmChunksRef = useRef<Float32Array[]>([]);
  const recordingSourceRef = useRef<VoiceSource | null>(null);
  const recordingPromptRef = useRef<SamplePrompt | null>(null);
  const recordingModeRef = useRef<RecordingMode>("single");
  const recordingSentencePromptRef = useRef<SentencePrompt | null>(null);
  const recordingSentenceTextRef = useRef("");
  const recordingLabelRef = useRef("");
  const recordingTranscriptRef = useRef("");
  const startedAtRef = useRef(0);

  const recordingSupported = Boolean(navigator.mediaDevices?.getUserMedia) && Boolean(getAudioContextConstructor());
  const requiredPrompts = useMemo(() => requiredPromptsFor(selectedSource), [selectedSource?.targetSamples]);
  const selectedPrompt =
    requiredPrompts.find((prompt) => prompt.id === selectedPromptId) ??
    SAMPLE_PROMPTS.find((prompt) => prompt.id === selectedPromptId) ??
    requiredPrompts[0] ??
    SAMPLE_PROMPTS[0];
  const selectedSentencePrompt =
    sentencePrompts.find((prompt) => prompt.id === selectedSentencePromptId) ?? sentencePrompts[0];
  const filledPromptIds = useMemo(() => filledPromptIdsOf(selectedSource), [selectedSource?.samples]);
  const missingPrompts = useMemo(
    () => requiredPrompts.filter((prompt) => !filledPromptIds.has(prompt.id)),
    [filledPromptIds, requiredPrompts]
  );
  const selectedPromptFilled = filledPromptIds.has(selectedPrompt.id);
  const nextMissingPrompt = missingPrompts[0];

  useEffect(() => {
    let active = true;
    voiceApi
      .listSentencePrompts()
      .then((prompts) => {
        if (!active) {
          return;
        }
        setSentencePrompts(prompts);
        if (prompts[0]) {
          setSelectedSentencePromptId((current) => current || prompts[0].id);
          setSentenceText((current) => current || prompts[0].text);
        }
      })
      .catch((error) => {
        if (active) {
          setRecorderError(error instanceof Error ? error.message : "문장 녹음 묶음을 불러오지 못했습니다.");
        }
      });

    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!isRecording) {
      return;
    }

    const timer = window.setInterval(() => {
      setElapsed((Date.now() - startedAtRef.current) / 1000);
    }, 200);

    return () => window.clearInterval(timer);
  }, [isRecording]);

  useEffect(() => {
    if (isRecording || requiredPrompts.some((prompt) => prompt.id === selectedPromptId)) {
      return;
    }

    applyPrompt(nextMissingPrompt ?? requiredPrompts[0] ?? SAMPLE_PROMPTS[0]);
  }, [isRecording, nextMissingPrompt, requiredPrompts, selectedPromptId]);

  const applyPrompt = (prompt: SamplePrompt) => {
    setSelectedPromptId(prompt.id);
    setLabel(prompt.label);
    setTranscript(prompt.text);
  };

  const applySentencePrompt = (prompt: SentencePrompt) => {
    setSelectedSentencePromptId(prompt.id);
    setSentenceText(prompt.text);
    setSentenceCandidates([]);
    setSentenceWarnings([]);
    setSavedCandidateIds(new Set());
  };

  const candidateDisabledReason = (candidate: SentenceSampleCandidate) => {
    if (savedCandidateIds.has(candidate.id)) {
      return "저장됨";
    }
    if (!candidate.promptId) {
      return "항목 없음";
    }
    if (!candidate.audioUrl && !candidate.dataBase64) {
      return "오디오 없음";
    }
    if (hasHardBlockingCandidateStatus(candidate) || candidate.duration < minCandidateDuration) {
      return "저장 불가";
    }
    if (candidate.confidence < minManualCandidateConfidence) {
      return "검수 불가";
    }
    return "";
  };

  const candidateCanSave = (candidate: SentenceSampleCandidate) =>
    !isSavingSentenceCandidates && !candidateDisabledReason(candidate);

  const candidateIsUseful = (candidate: SentenceSampleCandidate) =>
    candidateCanSave(candidate) &&
    hasUsefulCandidateStatus(candidate) &&
    candidate.confidence >= minAutoCandidateConfidence &&
    !candidate.warning &&
    !hasAutoBlockedCandidateStatus(candidate);

  const candidateNeedsReview = (candidate: SentenceSampleCandidate) =>
    Boolean(candidate.warning) || hasReviewCandidateStatus(candidate);

  const candidateSaveLabel = (candidate: SentenceSampleCandidate) => {
    const disabledReason = candidateDisabledReason(candidate);
    if (disabledReason) {
      return disabledReason;
    }
    return candidateNeedsReview(candidate) ? "검수 저장" : "저장";
  };

  const sampleFromCandidate = (candidate: SentenceSampleCandidate): Omit<VoiceSample, "id" | "createdAt"> => ({
    promptId: candidate.promptId,
    label: candidate.label,
    text: candidate.text,
    duration: candidate.duration,
    origin: "recording",
    audioName: candidate.audioName,
    audioUrl: candidate.audioUrl,
    dataBase64: candidate.dataBase64 || candidate.audioUrl
  });

  const markCandidatesSaved = (ids: string[]) => {
    setSavedCandidateIds((current) => {
      const next = new Set(current);
      ids.forEach((id) => next.add(id));
      return next;
    });
  };

  const findNextMissingPrompt = (completedPromptId?: string, skipPromptId?: string) => {
    const treatedAsFilled = new Set(filledPromptIds);
    if (completedPromptId) {
      treatedAsFilled.add(completedPromptId);
    }

    const startIndex = Math.max(0, requiredPrompts.findIndex((prompt) => prompt.id === selectedPromptId));
    const ordered = [...requiredPrompts.slice(startIndex + 1), ...requiredPrompts.slice(0, startIndex + 1)];
    return ordered.find((prompt) => prompt.id !== skipPromptId && !treatedAsFilled.has(prompt.id));
  };

  const moveToNextMissing = (completedPromptId?: string, skipPromptId?: string) => {
    const nextPrompt = findNextMissingPrompt(completedPromptId, skipPromptId);
    if (nextPrompt) {
      applyPrompt(nextPrompt);
      return true;
    }
    return false;
  };

  const goToNextMissing = () => {
    if (moveToNextMissing()) {
      setRecorderNotice("다음 누락 항목으로 이동했습니다.");
      return;
    }
    setRecorderNotice("필수 샘플이 모두 채워졌습니다.");
  };

  const skipCurrentPrompt = () => {
    if (moveToNextMissing(undefined, selectedPrompt.id)) {
      setRecorderNotice("현재 항목을 건너뛰고 다음 누락 항목으로 이동했습니다.");
      return;
    }
    setRecorderNotice("건너뛸 다음 누락 항목이 없습니다.");
  };

  const rerecordCurrentPrompt = async () => {
    if (isRecording || isPreparing) {
      return;
    }
    await startRecording();
  };

  const resolveSource = async () => {
    if (selectedSource) {
      return selectedSource;
    }
    setRecorderNotice("목소리 프리셋을 만들고 있습니다.");
    return onEnsureSource();
  };

  const saveSentenceCandidate = async (candidate: SentenceSampleCandidate) => {
    if (!candidateCanSave(candidate)) {
      const reason = candidateDisabledReason(candidate);
      if (reason) {
        setRecorderNotice(`${candidate.label} ${reason}`);
      }
      return;
    }
    const source = await resolveSource();
    if (!source) {
      setRecorderError("목소리 프리셋을 만들지 못했습니다.");
      return;
    }

    try {
      await onAddSample(source.id, sampleFromCandidate(candidate));
      markCandidatesSaved([candidate.id]);
      setRecorderNotice(
        candidateNeedsReview(candidate)
          ? `${candidate.label} 후보를 검수 저장했습니다. 경고가 있었으니 나중에 한 번 더 들어보세요.`
          : `${candidate.label} 후보를 샘플로 저장했습니다.`
      );
    } catch (error) {
      setRecorderError(error instanceof Error ? error.message : `${candidate.label} 후보를 저장하지 못했습니다.`);
    }
  };

  const saveUsefulSentenceCandidates = async () => {
    const targets = sentenceCandidates.filter(candidateIsUseful);
    if (!targets.length) {
      setRecorderNotice("자동 저장할 만큼 확실한 후보가 없습니다. review/warning 후보는 개별로 들어보고 저장하세요.");
      return;
    }

    const source = await resolveSource();
    if (!source) {
      setRecorderError("목소리 프리셋을 만들지 못했습니다.");
      return;
    }

    setIsSavingSentenceCandidates(true);
    setRecorderError("");
    const savedIds: string[] = [];
    const failures: string[] = [];

    for (const candidate of targets) {
      try {
        await onAddSample(source.id, sampleFromCandidate(candidate));
        savedIds.push(candidate.id);
        markCandidatesSaved([candidate.id]);
      } catch (error) {
        const message = error instanceof Error ? error.message : "알 수 없는 오류";
        failures.push(`${candidate.label}: ${message}`);
      }
    }

    if (savedIds.length) {
      setRecorderNotice(`${savedIds.length}개 후보를 샘플로 저장했습니다.`);
    }
    if (failures.length) {
      setRecorderError(`${failures.length}개 후보 저장 실패: ${failures.slice(0, 3).join(" / ")}`);
    }
    setIsSavingSentenceCandidates(false);
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

  const startRecording = async (mode: RecordingMode = "single") => {
    setRecorderError("");
    setRecorderNotice("");
    setIsPreparing(true);

    try {
      const sentencePrompt = selectedSentencePrompt;
      const activeSentenceText = sentenceText.trim() || sentencePrompt?.text || "";
      if (mode === "sentence" && !activeSentenceText) {
        setRecorderError("문장 녹음에 사용할 텍스트를 입력하세요.");
        return;
      }
      const source = await resolveSource();
      if (!source) {
        setRecorderError("목소리 프리셋을 만들지 못했습니다.");
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
      recordingPromptRef.current = mode === "single" ? selectedPrompt : null;
      recordingModeRef.current = mode;
      recordingSentencePromptRef.current = mode === "sentence" ? sentencePrompt ?? null : null;
      recordingSentenceTextRef.current = mode === "sentence" ? activeSentenceText : "";
      recordingLabelRef.current = label.trim() || selectedPrompt.label;
      recordingTranscriptRef.current = transcript.trim() || selectedPrompt.text;
      startedAtRef.current = Date.now();
      setElapsed(0);
      setLastPreviewUrl("");
      if (mode === "sentence") {
        setSentenceRecordingUrl("");
        setSentenceCandidates([]);
        setSentenceWarnings([]);
        setSavedCandidateIds(new Set());
      }

      setIsRecording(true);
      setRecorderNotice(
        mode === "sentence"
          ? "문장 녹음 중입니다. 문장을 자연스럽게 읽고 후보 추출을 누르세요."
          : "녹음 중입니다. 제시된 샘플을 읽은 뒤 정지를 누르세요."
      );
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
    const mode = recordingModeRef.current;
    const sentencePrompt = recordingSentencePromptRef.current;
    const sentenceTextAtStart = recordingSentenceTextRef.current;
    const chunks = pcmChunksRef.current;
    const sampleRate = audioContextRef.current?.sampleRate ?? 44100;
    cleanupRecording();
    pcmChunksRef.current = [];
    recordingSourceRef.current = null;
    recordingPromptRef.current = null;
    recordingSentencePromptRef.current = null;
    recordingSentenceTextRef.current = "";
    setIsRecording(false);

    if (!source) {
      setRecorderError("목소리 프리셋을 찾지 못했습니다.");
      return;
    }
    if (!chunks.length) {
      setRecorderError("녹음 데이터가 비어 있습니다. 마이크 입력을 확인하세요.");
      return;
    }

    try {
      const { dataUrl, previewUrl, duration, trimmedSamples } = wavDataUrlFromPcm(
        chunks,
        sampleRate,
        mode === "sentence" ? `sentence-${sentencePrompt?.id ?? "custom"}.wav` : `recording-${prompt.id}.wav`
      );
      setLastPreviewUrl(previewUrl);

      if (mode === "sentence") {
        setSentenceRecordingUrl(previewUrl);
        setIsExtractingSentence(true);
        try {
          const result = await voiceApi.extractSentenceSamples({
            promptId: sentencePrompt?.id,
            sentencePromptId: sentencePrompt?.id,
            targetSamples: normalizeTargetSamples(source.targetSamples),
            text: sentenceTextAtStart,
            audioName: `sentence-${sentencePrompt?.id ?? "custom"}.wav`,
            audioUrl: previewUrl,
            dataBase64: dataUrl
          });
          setSentenceCandidates(result.candidates);
          setSentenceWarnings(result.warnings ?? []);
          setSavedCandidateIds(new Set());
          if (result.candidates.length) {
            setRecorderNotice(
              `${result.totalCandidates || result.candidates.length}개 후보를 추출했습니다. 재생해서 확인한 뒤 저장하세요.${
                trimmedSamples > 0 ? " 앞뒤 공백은 잘라 저장했습니다." : ""
              }`
            );
          } else {
            setRecorderNotice("녹음이 너무 짧거나 무음이라 후보를 만들지 못했습니다. 조금 더 길게 다시 녹음해 주세요.");
          }
        } finally {
          setIsExtractingSentence(false);
        }
        return;
      }

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
      const moved = moveToNextMissing(prompt.id);
      const trimNote = trimmedSamples > 0 ? " 앞뒤 공백을 잘라 저장했습니다." : "";
      setRecorderNotice(
        moved
          ? `저장했습니다.${trimNote} 다음 누락 항목으로 이동했습니다.`
          : `필수 샘플을 모두 채웠습니다.${trimNote}`
      );
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
      setRecorderError("목소리 프리셋을 만들지 못했습니다.");
      return;
    }
    setRecorderError("");
    try {
      let trimmedAny = false;
      for (const file of files) {
        const converted = await decodeAudioFileToMonoWav(file);
        trimmedAny = trimmedAny || converted.trimmedSamples > 0;
        await onAddSample(source.id, {
          promptId: selectedPrompt.id,
          label: label.trim() || selectedPrompt.label || file.name,
          text: transcript.trim() || selectedPrompt.text || file.name,
          duration: converted.duration,
          origin: "upload",
          audioName: converted.fileName,
          audioUrl: converted.previewUrl,
          dataBase64: converted.dataUrl
        });
      }
      moveToNextMissing(selectedPrompt.id);
      setRecorderNotice(
        `${files.length}개 파일을 현재 샘플 항목에 등록했습니다.${trimmedAny ? " 앞뒤 공백을 잘라 저장했습니다." : ""}`
      );
    } catch (error) {
      setRecorderError(error instanceof Error ? error.message : "WAV 파일을 업로드하지 못했습니다.");
      event.target.value = "";
      return;
    }
    event.target.value = "";
  };

  const progress = selectedSource ? progressOf(selectedSource) : 0;
  const statusText = isRecording
    ? `${elapsed.toFixed(1)}초 녹음 중`
    : isPreparing
      ? "마이크 준비 중"
      : isExtractingSentence
        ? "후보 추출 중"
        : "대기";

  return (
    <section className="record-grid">
      <div className="recorder-panel">
        <div className="section-head compact">
          <div>
            <FieldLabel>앱 내 직접 녹음</FieldLabel>
            <h2>{selectedSource?.name ?? "새 프리셋이 자동으로 만들어집니다"}</h2>
          </div>
          <span className="pill">
            필수 {filledCountOf(selectedSource)}/{normalizeTargetSamples(selectedSource?.targetSamples)}
          </span>
        </div>

        <ProgressBar value={progress} />

        <div className="record-mode-tabs" role="tablist" aria-label="녹음 모드">
          <button
            className={recordMode === "single" ? "is-active" : ""}
            type="button"
            onClick={() => setRecordMode("single")}
            disabled={isRecording}
          >
            낱개 녹음
          </button>
          <button
            className={recordMode === "sentence" ? "is-active" : ""}
            type="button"
            onClick={() => setRecordMode("sentence")}
            disabled={isRecording}
          >
            문장 녹음
          </button>
        </div>

        {recordMode === "single" ? (
        <div className="record-main">
          <div className="current-prompt">
            <div className="prompt-card-head">
              <div>
                <FieldLabel>읽을 샘플</FieldLabel>
                <strong>{selectedPrompt.label}</strong>
              </div>
              <span className={`prompt-state ${selectedPromptFilled ? "is-filled" : ""}`}>
                {selectedPromptFilled ? "저장됨" : "누락"}
              </span>
            </div>
            <p>{selectedPrompt.text}</p>
            <div className="prompt-actions">
              <button type="button" onClick={goToNextMissing} disabled={!missingPrompts.length || isRecording}>
                다음 누락
              </button>
              <button type="button" onClick={skipCurrentPrompt} disabled={missingPrompts.length <= 1 || isRecording}>
                건너뛰기
              </button>
              <button type="button" onClick={rerecordCurrentPrompt} disabled={isRecording || isPreparing}>
                재녹음
              </button>
              <span>저장 후 자동 진행 · 남은 {missingPrompts.length}개</span>
            </div>
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
            <button className="primary" type="button" onClick={() => startRecording("single")} disabled={isRecording || isPreparing}>
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
        ) : (
        <div className="record-main sentence-main">
          <div className="sentence-reader">
            <label>
              <FieldLabel>문장 묶음</FieldLabel>
              <select
                value={selectedSentencePrompt?.id ?? ""}
                onChange={(event) => {
                  const prompt = sentencePrompts.find((item) => item.id === event.target.value);
                  if (prompt) {
                    applySentencePrompt(prompt);
                  }
                }}
              >
                {sentencePrompts.map((prompt) => (
                  <option key={prompt.id} value={prompt.id}>
                    {prompt.title}
                  </option>
                ))}
              </select>
            </label>
            {selectedSentencePrompt?.description ? (
              <p className="sentence-description">{selectedSentencePrompt.description}</p>
            ) : null}
            <label>
              <FieldLabel>읽을 문장</FieldLabel>
              <textarea value={sentenceText} onChange={(event) => setSentenceText(event.target.value)} rows={5} />
            </label>
          </div>

          <div className="recorder-controls">
            <button
              className="primary"
              type="button"
              onClick={() => startRecording("sentence")}
              disabled={isRecording || isPreparing || isExtractingSentence || !sentenceText.trim()}
            >
              문장 녹음 시작
            </button>
            <button type="button" onClick={stopRecording} disabled={!isRecording || recordMode !== "sentence"}>
              정지 후 후보 추출
            </button>
            <span className={`meter ${isRecording ? "is-live" : ""}`}>{statusText}</span>
          </div>

          {sentenceRecordingUrl ? (
            <audio className="preview-player" controls src={sentenceRecordingUrl}>
              <track kind="captions" />
            </audio>
          ) : null}
          {sentenceWarnings.length ? (
            <div className="warning-list">
              {sentenceWarnings.map((warning) => (
                <span key={warning}>{warning}</span>
              ))}
            </div>
          ) : null}
          {recorderError ? <p className="error-line">{recorderError}</p> : <p className="info-line">{recorderNotice}</p>}
        </div>
        )}
      </div>

      <div className="record-side">
        {recordMode === "single" ? (
        <div className="upload-panel">
          <div className="section-head compact">
            <div>
              <FieldLabel>프롬프트</FieldLabel>
              <h2>녹음할 항목</h2>
            </div>
            <label className="file-button">
              파일 업로드
              <input type="file" accept="audio/*,.wav" multiple onChange={handleUpload} />
            </label>
          </div>
          <div className="prompt-grid">
            {requiredPrompts.map((prompt) => (
              <button
                className={[
                  prompt.id === selectedPromptId ? "is-active" : "",
                  filledPromptIds.has(prompt.id) ? "is-filled" : "",
                  missingPrompts.some((item) => item.id === prompt.id) ? "is-missing" : ""
                ]
                  .filter(Boolean)
                  .join(" ")}
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
        ) : (
        <div className="upload-panel candidate-panel">
          <div className="section-head compact">
            <div>
              <FieldLabel>추출 후보</FieldLabel>
              <h2>문장 녹음 후보</h2>
            </div>
            <button
              className="primary"
              type="button"
              onClick={saveUsefulSentenceCandidates}
              disabled={isSavingSentenceCandidates || !sentenceCandidates.some(candidateIsUseful)}
            >
              {isSavingSentenceCandidates ? "저장 중" : "쓸만한 후보 모두 저장"}
            </button>
          </div>
          <div className="candidate-list">
            {sentenceCandidates.length ? (
              sentenceCandidates.map((candidate) => {
                const saved = savedCandidateIds.has(candidate.id);
                const canSave = candidateCanSave(candidate);
                const confidence = Math.round(candidate.confidence * 100);
                return (
                  <div className={`candidate-row ${saved ? "is-saved" : ""}`} key={candidate.id}>
                    <div className="candidate-main">
                      <div className="candidate-head">
                        <strong>{candidate.label}</strong>
                        <span className="candidate-meta">
                          {candidate.status || "review"} · {confidence}% · {formatDuration(candidate.duration)}
                        </span>
                      </div>
                      <p>{candidate.text}</p>
                      {candidate.warning ? <span className="candidate-warning">{candidate.warning}</span> : null}
                      {candidate.audioUrl || candidate.dataBase64 ? (
                        <audio controls src={candidate.audioUrl || candidate.dataBase64}>
                          <track kind="captions" />
                        </audio>
                      ) : null}
                    </div>
                    <button type="button" onClick={() => saveSentenceCandidate(candidate)} disabled={!canSave}>
                      {candidateSaveLabel(candidate)}
                    </button>
                  </div>
                );
              })
            ) : (
              <span className="empty-line">
                {sentenceWarnings.length
                  ? "녹음이 너무 짧거나 무음이라 후보를 만들지 못했습니다."
                  : "문장을 녹음하면 잘라낸 후보가 여기에 표시됩니다."}
              </span>
            )}
          </div>
        </div>
        )}

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

function LibraryTab({
  speechItems,
  speechLibrarySettings,
  onSetSpeechLibraryDirectory,
  onChooseSpeechLibraryDirectory,
  onGetSpeechItemAudio,
  onDeleteSpeechItem
}: {
  speechItems: SpeechItem[];
  speechLibrarySettings: SpeechLibrarySettings | null;
  onSetSpeechLibraryDirectory: (path: string) => Promise<void>;
  onChooseSpeechLibraryDirectory: () => Promise<void>;
  onGetSpeechItemAudio: (id: string) => Promise<string>;
  onDeleteSpeechItem: (id: string) => Promise<void>;
}) {
  const [libraryPath, setLibraryPath] = useState("");
  const [activeItemId, setActiveItemId] = useState("");
  const [audioUrls, setAudioUrls] = useState<Record<string, string>>({});
  const [loadingAudioId, setLoadingAudioId] = useState("");
  const [audioErrors, setAudioErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    setLibraryPath(speechLibrarySettings?.isDefault ? "" : speechLibrarySettings?.path ?? "");
  }, [speechLibrarySettings?.isDefault, speechLibrarySettings?.path]);

  const submitLibraryDirectory = async (event: FormEvent) => {
    event.preventDefault();
    await onSetSpeechLibraryDirectory(libraryPath);
  };

  const libraryPathPlaceholder = speechLibrarySettings?.defaultPath
    ? `비워두면 기본 보관함 폴더를 사용합니다: ${speechLibrarySettings.defaultPath}`
    : "비워두면 기본 말하기 보관함 폴더를 사용합니다.";

  const prepareAudio = async (item: SpeechItem) => {
    if (audioUrls[item.id] || item.audioUrl || loadingAudioId === item.id) {
      setActiveItemId(item.id);
      return;
    }

    setActiveItemId(item.id);
    setLoadingAudioId(item.id);
    setAudioErrors((current) => {
      const next = { ...current };
      delete next[item.id];
      return next;
    });

    try {
      const audioUrl = await onGetSpeechItemAudio(item.id);
      setAudioUrls((current) => ({ ...current, [item.id]: audioUrl }));
    } catch (error) {
      setAudioErrors((current) => ({
        ...current,
        [item.id]: error instanceof Error ? error.message : "오디오를 불러오지 못했습니다."
      }));
    } finally {
      setLoadingAudioId((current) => (current === item.id ? "" : current));
    }
  };

  return (
    <section className="work-surface library-surface">
      <form className="library-settings" onSubmit={submitLibraryDirectory}>
        <div className="section-head compact">
          <div>
            <FieldLabel>앱 안 보관</FieldLabel>
            <h2>말하기 저장 폴더</h2>
          </div>
          <div className="button-pair">
            <button type="button" onClick={onChooseSpeechLibraryDirectory}>
              찾아보기
            </button>
            <button className="primary" type="submit">
              저장
            </button>
          </div>
        </div>
        <label>
          <FieldLabel>보관함 폴더 경로</FieldLabel>
          <input
            value={libraryPath}
            onChange={(event) => setLibraryPath(event.target.value)}
            placeholder={libraryPathPlaceholder}
          />
        </label>
        <p className="info-line">
          {speechLibrarySettings?.message ??
            "현재 말하기 결과를 앱 보관함 폴더에 저장하고 이 목록에서 다시 재생합니다."}
          {speechLibrarySettings?.isDefault && speechLibrarySettings.defaultPath
            ? ` 기본 보관함: ${speechLibrarySettings.defaultPath}`
            : ""}
          {speechLibrarySettings?.source === "browser"
            ? " 브라우저 데모 값은 localStorage와 데모 WAV로만 동작합니다."
            : ""}
        </p>
      </form>

      <div className="library-list-panel">
        <div className="section-head compact">
          <div>
            <FieldLabel>재생목록</FieldLabel>
            <h2>저장된 말하기 {speechItems.length}개</h2>
          </div>
        </div>

        <div className="speech-list">
          {speechItems.length ? (
            speechItems.map((item) => {
              const title = item.title?.trim() || item.text.trim() || "저장된 말하기";
              const audioUrl = audioUrls[item.id] || item.audioUrl || "";
              const hasPlayableAudio = Boolean(audioUrl);
              const isAudioLoading = loadingAudioId === item.id;
              const audioError = audioErrors[item.id];
              return (
                <article
                  className={`speech-item ${activeItemId === item.id ? "is-playing" : ""}`}
                  key={item.id}
                  onClick={() => setActiveItemId(item.id)}
                >
                  <div className="speech-item-head">
                    <div>
                      <strong>{title}</strong>
                      <span>
                        {item.sourceName} · {formatDate(item.createdAt)} · {formatDuration(item.duration)}
                        {item.audioName ? ` · ${item.audioName}` : item.path ? ` · ${item.path}` : ""}
                      </span>
                    </div>
                    <button
                      className="danger"
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation();
                        onDeleteSpeechItem(item.id);
                      }}
                    >
                      삭제
                    </button>
                  </div>
                  <p>{item.text}</p>
                  {hasPlayableAudio ? (
                    <audio
                      controls
                      src={audioUrl}
                      onPlay={() => setActiveItemId(item.id)}
                    >
                      <track kind="captions" />
                    </audio>
                  ) : (
                    <button
                      className="audio-load-button"
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation();
                        prepareAudio(item);
                      }}
                      disabled={isAudioLoading}
                    >
                      <span>{isAudioLoading ? "오디오 불러오는 중" : "재생 준비"}</span>
                      <small>{audioError || "누르면 보관함 오디오를 불러옵니다."}</small>
                    </button>
                  )}
                </article>
              );
            })
          ) : (
            <div className="library-empty">
              <strong>저장된 말하기가 없습니다.</strong>
              <span>말하기 탭에서 현재 문장을 만든 뒤 보관함 저장을 누르세요.</span>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

function ManageTab({
  selectedSource,
  outputDirectory,
  onCreate,
  onUpdate,
  onDelete,
  onSetOutputDirectory,
  onChooseOutputDirectory
}: {
  selectedSource?: VoiceSource;
  outputDirectory: OutputDirectorySettings | null;
  onCreate: (input: { name: string; speaker: string; note: string; targetSamples: number }) => Promise<VoiceSource>;
  onUpdate: (patch: Partial<VoiceSource>) => void;
  onDelete: () => void;
  onSetOutputDirectory: (path: string) => Promise<void>;
  onChooseOutputDirectory: () => Promise<void>;
}) {
  const [newName, setNewName] = useState("");
  const [newSpeaker, setNewSpeaker] = useState("");
  const [newNote, setNewNote] = useState("");
  const [newTarget, setNewTarget] = useState(targetOptionValue(MIN_SAMPLE_TARGET));
  const [editName, setEditName] = useState("");
  const [editSpeaker, setEditSpeaker] = useState("");
  const [editNote, setEditNote] = useState("");
  const [editTarget, setEditTarget] = useState(targetOptionValue(MIN_SAMPLE_TARGET));
  const [outputPath, setOutputPath] = useState("");

  useEffect(() => {
    setEditName(selectedSource?.name ?? "");
    setEditSpeaker(selectedSource?.speaker ?? "");
    setEditNote(selectedSource?.note ?? "");
    setEditTarget(targetOptionValue(selectedSource?.targetSamples));
  }, [selectedSource]);

  useEffect(() => {
    setOutputPath(outputDirectory?.isDefault ? "" : outputDirectory?.path ?? "");
  }, [outputDirectory?.isDefault, outputDirectory?.path]);

  const submitCreate = async (event: FormEvent) => {
    event.preventDefault();
    await onCreate({
      name: newName,
      speaker: newSpeaker,
      note: newNote,
      targetSamples: targetOptionValue(newTarget)
    });
    setNewName("");
    setNewSpeaker("");
    setNewNote("");
    setNewTarget(targetOptionValue(MIN_SAMPLE_TARGET));
  };

  const submitUpdate = (event: FormEvent) => {
    event.preventDefault();
    onUpdate({
      name: editName,
      speaker: editSpeaker,
      note: editNote,
      targetSamples: targetOptionValue(editTarget)
    });
  };

  const submitOutputDirectory = async (event: FormEvent) => {
    event.preventDefault();
    await onSetOutputDirectory(outputPath);
  };

  const outputPathPlaceholder = outputDirectory?.defaultPath
    ? `비워두면 기본 폴더를 사용합니다: ${outputDirectory.defaultPath}`
    : "비워두면 기본 exports 폴더를 사용합니다.";

  return (
    <section className="manage-grid">
      <form className="manage-form" onSubmit={submitCreate}>
        <div className="section-head compact">
          <div>
            <FieldLabel>생성</FieldLabel>
            <h2>새 목소리 프리셋</h2>
          </div>
          <button className="primary" type="submit">
            생성
          </button>
        </div>
        <label>
          <FieldLabel>프리셋 이름</FieldLabel>
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
        <div className="target-field">
          <FieldLabel>녹음 타입</FieldLabel>
          <SampleTargetPicker value={newTarget} onChange={setNewTarget} />
        </div>
      </form>

      <form className="manage-form" onSubmit={submitUpdate}>
        <div className="section-head compact">
          <div>
            <FieldLabel>편집</FieldLabel>
            <h2>{selectedSource?.name ?? "선택된 프리셋 없음"}</h2>
          </div>
          <button type="submit" disabled={!selectedSource}>
            저장
          </button>
        </div>
        <label>
          <FieldLabel>프리셋 이름</FieldLabel>
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
        <div className="target-field">
          <FieldLabel>녹음 타입</FieldLabel>
          <SampleTargetPicker value={editTarget} onChange={setEditTarget} disabled={!selectedSource} />
        </div>
        <button className="danger" type="button" onClick={onDelete} disabled={!selectedSource}>
          프리셋 삭제
        </button>
      </form>

      <form className="manage-form output-form" onSubmit={submitOutputDirectory}>
        <div className="section-head compact">
          <div>
            <FieldLabel>저장 위치</FieldLabel>
            <h2>MP3 파일 저장 폴더</h2>
          </div>
          <div className="button-pair">
            <button type="button" onClick={onChooseOutputDirectory}>
              찾아보기
            </button>
            <button className="primary" type="submit">
              저장
            </button>
          </div>
        </div>
        <label>
          <FieldLabel>폴더 경로</FieldLabel>
          <input
            value={outputPath}
            onChange={(event) => setOutputPath(event.target.value)}
            placeholder={outputPathPlaceholder}
          />
        </label>
        <p className="info-line">
          {outputDirectory?.message ??
            "백엔드 설정 API가 연결되면 이 위치에 MP3 저장 결과가 만들어집니다."}
          {outputDirectory?.isDefault && outputDirectory.defaultPath ? ` 기본 폴더: ${outputDirectory.defaultPath}` : ""}
          {outputDirectory?.source === "browser" ? " 브라우저 데모 값은 localStorage에만 저장됩니다." : ""}
        </p>
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
  const sourceComplete = Boolean(selectedSource && filledCountOf(selectedSource) >= normalizeTargetSamples(selectedSource.targetSamples));
  const coveredPromptLabels = new Set(selectedSource?.samples.map((sample) => sample.label));
  const coveredPromptIds = new Set(selectedSource?.samples.map((sample) => sample.promptId ?? sample.label));
  const requiredPrompts = selectedSource ? requiredPromptsFor(selectedSource) : [];
  const nextPrompts = requiredPrompts.filter(
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
        <FieldLabel>선택 프리셋</FieldLabel>
        <h2>{selectedSource?.name ?? "없음"}</h2>
        <dl className="metric-list">
          <div>
            <dt>화자</dt>
            <dd>{selectedSource?.speaker ?? "-"}</dd>
          </div>
          <div>
            <dt>샘플</dt>
            <dd>
              {filledCountOf(selectedSource)}/{normalizeTargetSamples(selectedSource?.targetSamples)}
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
            <dt>MP3 저장</dt>
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
  const [isSavingToLibrary, setIsSavingToLibrary] = useState(false);
  const [engineStatus, setEngineStatus] = useState<EngineStatus | null>(null);
  const [outputDirectory, setOutputDirectory] = useState<OutputDirectorySettings | null>(null);
  const [speechLibrarySettings, setSpeechLibrarySettings] = useState<SpeechLibrarySettings | null>(null);
  const [speechItems, setSpeechItems] = useState<SpeechItem[]>([]);
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

  const refreshSpeechItems = async () => {
    setSpeechItems(await voiceApi.listSpeechItems());
  };

  useEffect(() => {
    refreshSources();
    refreshSpeechItems();
    voiceApi.getEngineStatus().then(setEngineStatus);
    voiceApi.getOutputDirectory().then(setOutputDirectory);
    voiceApi.getSpeechLibrarySettings().then(setSpeechLibrarySettings);
  }, []);

  useEffect(() => {
    setPreview(null);
  }, [selectedSourceId, selectedSource?.updatedAt, text, options]);

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
      targetSamples: targetOptionValue(input.targetSamples)
    });
    await refreshSources(created.id);
    setActiveTab("record");
    setNotice("새 목소리 프리셋을 만들었습니다. 녹음 탭에서 바로 샘플을 채울 수 있습니다.");
    return created;
  };

  const ensureRecordSource = async () => {
    if (selectedSource) {
      return selectedSource;
    }
    return handleCreateSource({
      name: "내 목소리",
      speaker: "나",
      note: "앱에서 직접 녹음한 목소리 프리셋",
      targetSamples: MIN_SAMPLE_TARGET
    });
  };

  const handleUpdateSource = async (patch: Partial<VoiceSource>) => {
    if (!selectedSource) {
      return;
    }
    const updated = await voiceApi.updateSource(selectedSource.id, patch);
    await refreshSources(updated.id);
    setNotice("프리셋 정보를 저장했습니다.");
  };

  const handleDeleteSource = async () => {
    if (!selectedSource) {
      return;
    }
    await voiceApi.deleteSource(selectedSource.id);
    await refreshSources();
    setNotice("프리셋을 삭제했습니다.");
  };

  const handleAddSample = async (sourceId: string, sample: Omit<VoiceSample, "id" | "createdAt">) => {
    const updated = await voiceApi.addSample(sourceId, sample);
    setSources((current) => {
      const existing = current.some((source) => source.id === updated.id);
      return existing ? current.map((source) => (source.id === updated.id ? updated : source)) : [...current, updated];
    });
    setSelectedSourceId(updated.id);
    try {
      await refreshSources(updated.id);
    } catch (error) {
      console.warn("Failed to refresh sources after saving sample.", error);
    }
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

  const handleSetOutputDirectory = async (path: string) => {
    try {
      const settings = await voiceApi.setOutputDirectory(path);
      setOutputDirectory(settings);
      setNotice(settings.message);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "MP3 저장 위치를 저장하지 못했습니다.");
    }
  };

  const handleChooseOutputDirectory = async () => {
    try {
      const settings = await voiceApi.chooseOutputDirectory();
      setOutputDirectory(settings);
      setNotice(settings.message);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "MP3 저장 위치를 선택하지 못했습니다.");
    }
  };

  const handleSetSpeechLibraryDirectory = async (path: string) => {
    try {
      const settings = await voiceApi.setSpeechLibraryDirectory(path);
      setSpeechLibrarySettings(settings);
      setNotice(settings.message);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "말하기 저장 폴더를 저장하지 못했습니다.");
    }
  };

  const handleChooseSpeechLibraryDirectory = async () => {
    try {
      const settings = await voiceApi.chooseSpeechLibraryDirectory();
      setSpeechLibrarySettings(settings);
      setNotice(settings.message);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "말하기 저장 폴더를 선택하지 못했습니다.");
    }
  };

  const handleDeleteSpeechItem = async (id: string) => {
    await voiceApi.deleteSpeechItem(id);
    await refreshSpeechItems();
    setNotice("보관함 항목을 삭제했습니다.");
  };

  const handleGetSpeechItemAudio = async (id: string) => {
    return voiceApi.getSpeechItemAudio(id);
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

  const handleSaveSpeechItem = async () => {
    if (!selectedSource || !text.trim()) {
      return;
    }
    setIsSavingToLibrary(true);
    try {
      const item = await voiceApi.saveSpeechItem({
        sourceId: selectedSource.id,
        text,
        options
      });
      await refreshSpeechItems();
      setNotice(item.path ? `보관함에 저장했습니다: ${item.path}` : "보관함에 저장했습니다.");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "보관함에 저장하지 못했습니다.");
    } finally {
      setIsSavingToLibrary(false);
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
              isSavingToLibrary={isSavingToLibrary}
              onPreview={handlePreview}
              onExport={handleExport}
              onSaveToLibrary={handleSaveSpeechItem}
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

          {activeTab === "library" ? (
            <LibraryTab
              speechItems={speechItems}
              speechLibrarySettings={speechLibrarySettings}
              onSetSpeechLibraryDirectory={handleSetSpeechLibraryDirectory}
              onChooseSpeechLibraryDirectory={handleChooseSpeechLibraryDirectory}
              onGetSpeechItemAudio={handleGetSpeechItemAudio}
              onDeleteSpeechItem={handleDeleteSpeechItem}
            />
          ) : null}

          {activeTab === "manage" ? (
            <ManageTab
              selectedSource={selectedSource}
              outputDirectory={outputDirectory}
              onCreate={handleCreateSource}
              onUpdate={handleUpdateSource}
              onDelete={handleDeleteSource}
              onSetOutputDirectory={handleSetOutputDirectory}
              onChooseOutputDirectory={handleChooseOutputDirectory}
            />
          ) : null}
        </div>
      </main>

      <StatusPanel selectedSource={selectedSource} analysis={analysis} engineStatus={engineStatus} preview={preview} />
    </div>
  );
}

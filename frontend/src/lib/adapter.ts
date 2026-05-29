import {
  AnalysisResult,
  EngineStatus,
  ExportResult,
  MIN_SAMPLE_TARGET,
  MissingSample,
  OutputDirectorySettings,
  PreviewResult,
  SAMPLE_PROMPTS,
  SAMPLE_TARGET_OPTIONS,
  SaveSpeechItemInput,
  SentenceExtractionInput,
  SentenceExtractionResult,
  SentencePrompt,
  SentenceSampleCandidate,
  SpeechItem,
  SpeechLibrarySettings,
  SynthesisRequest,
  VoiceSample,
  VoiceSource,
  normalizeTargetSamples
} from "../types";

type CreateSourceInput = Partial<
  Pick<VoiceSource, "name" | "speaker" | "note" | "targetSamples">
>;

type AddSampleInput = Omit<VoiceSample, "id" | "createdAt">;

type LooseRecord = Record<string, unknown>;

type LegacySentencePrompt = Partial<SentencePrompt> & {
  ID?: string;
  Title?: string;
  Text?: string;
  Description?: string;
  CoveredPromptIDs?: string[];
  CoveredPromptIds?: string[];
  PromptID?: string;
  PromptId?: string;
  PromptIDs?: string[];
  PromptIds?: string[];
  covered_prompt_ids?: string[];
  prompt_ids?: string[];
};

type LegacySentenceCandidate = Partial<SentenceSampleCandidate> & {
  ID?: string;
  PromptID?: string;
  PromptId?: string;
  Label?: string;
  Text?: string;
  StartSeconds?: number;
  Start?: number;
  EndSeconds?: number;
  End?: number;
  Duration?: number;
  DurationSeconds?: number;
  Confidence?: number;
  Score?: number;
  Status?: string;
  Warning?: string;
  AudioName?: string;
  AudioURL?: string;
  AudioUrl?: string;
  DataBase64?: string;
  data?: string;
};

type LegacySentenceResult = Partial<SentenceExtractionResult> & {
  Prompt?: LegacySentencePrompt;
  prompt?: LegacySentencePrompt;
  PromptID?: string;
  PromptId?: string;
  Text?: string;
  TotalCandidates?: number;
  Total?: number;
  SourceDuration?: number;
  TrimmedDuration?: number;
  Candidates?: LegacySentenceCandidate[];
  Warnings?: string[];
  warning?: string;
};

type LegacySpeechItem = Partial<SpeechItem> & {
  durationMillis?: number;
  durationSeconds?: number;
  fileName?: string;
  title?: string;
  AudioURL?: string;
  AudioUrl?: string;
  CreatedAt?: string;
  FileName?: string;
  SourceID?: string;
  SourceName?: string;
  Title?: string;
  Duration?: number;
  DurationMillis?: number;
};

interface WailsApp {
  ListSources?: () => Promise<VoiceSource[]> | VoiceSource[];
  CreateSource?: (input: CreateSourceInput) => Promise<VoiceSource> | VoiceSource;
  UpdateSource?: (
    id: string,
    patch: Partial<VoiceSource>
  ) => Promise<VoiceSource> | VoiceSource;
  DeleteSource?: (id: string) => Promise<void> | void;
  AddSample?: (
    sourceId: string,
    sample: AddSampleInput
  ) => Promise<VoiceSource> | VoiceSource;
  AnalyzeText?: (
    sourceId: string,
    text: string
  ) => Promise<AnalysisResult> | AnalysisResult;
  Synthesize?: (
    request: SynthesisRequest
  ) => Promise<PreviewResult> | PreviewResult;
  ExportMP3?: (request: SynthesisRequest) => Promise<ExportResult> | ExportResult;
  GetEngineStatus?: () => Promise<EngineStatus> | EngineStatus;
  GetOutputDirectory?: () =>
    | Promise<Partial<OutputDirectorySettings> | string>
    | Partial<OutputDirectorySettings>
    | string;
  SetOutputDirectory?: (
    path: string
  ) => Promise<Partial<OutputDirectorySettings> | string> | Partial<OutputDirectorySettings> | string;
  ChooseOutputDirectory?: () =>
    | Promise<Partial<OutputDirectorySettings> | string>
    | Partial<OutputDirectorySettings>
    | string;
  OpenOutputDirectory?: () =>
    | Promise<Partial<OutputDirectorySettings> | string>
    | Partial<OutputDirectorySettings>
    | string;
  GetSpeechLibrarySettings?: () =>
    | Promise<Partial<SpeechLibrarySettings> | string>
    | Partial<SpeechLibrarySettings>
    | string;
  SetSpeechLibraryDirectory?: (
    path: string
  ) => Promise<Partial<SpeechLibrarySettings> | string> | Partial<SpeechLibrarySettings> | string;
  ChooseSpeechLibraryDirectory?: () =>
    | Promise<Partial<SpeechLibrarySettings> | string>
    | Partial<SpeechLibrarySettings>
    | string;
  OpenSpeechLibraryDirectory?: () =>
    | Promise<Partial<SpeechLibrarySettings> | string>
    | Partial<SpeechLibrarySettings>
    | string;
  ListSpeechItems?: () => Promise<SpeechItem[]> | SpeechItem[];
  GetSpeechItemAudio?: (id: string) => Promise<string> | string;
  GetSpeechItemAudioURL?: (id: string) => Promise<string> | string;
  SaveSpeechItem?: (input: SaveSpeechItemInput) => Promise<SpeechItem> | SpeechItem;
  DeleteSpeechItem?: (id: string) => Promise<void> | void;
  ListSentencePrompts?: () => Promise<LegacySentencePrompt[]> | LegacySentencePrompt[];
  ExtractSentenceSamples?: (
    request: SentenceExtractionInput
  ) => Promise<LegacySentenceResult> | LegacySentenceResult;
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: WailsApp;
      };
    };
  }
}

const STORAGE_KEY = "guvoice.sources.v3";
const OUTPUT_DIRECTORY_KEY = "guvoice.outputDirectory.v1";
const SPEECH_LIBRARY_KEY = "guvoice.speechLibrary.items.v1";
const SPEECH_LIBRARY_DIRECTORY_KEY = "guvoice.speechLibrary.directory.v1";
const DEFAULT_SAMPLE_TARGET = SAMPLE_TARGET_OPTIONS[0]?.value ?? MIN_SAMPLE_TARGET;

const DEMO_SENTENCE_PROMPTS: SentencePrompt[] = [
  {
    id: "sentence-basic-1",
    title: "기본 음절 한 번에 읽기",
    text: "아 어 오 우 으 이 애 에 야 여 요 유 가 나 다 라 마 바 사 자 차 카 타 파 하",
    description: "필수 음절 후보를 한 번 녹음으로 추출합니다.",
    promptIds: SAMPLE_PROMPTS.slice(0, 25).map((prompt) => prompt.id)
  },
  {
    id: "sentence-tone-1",
    title: "짧은 말투 문장",
    text: "안녕하세요. 오늘은 맑고 차분하게 말합니다. 작고 빠른 소리도 읽어 봅니다.",
    description: "말투 샘플 후보를 함께 확인합니다.",
    promptIds: SAMPLE_PROMPTS.slice(18, 25).map((prompt) => prompt.id)
  }
];

const nowIso = () => new Date().toISOString();

const createId = (prefix: string) => {
  if (crypto.randomUUID) {
    return `${prefix}-${crypto.randomUUID()}`;
  }
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
};

const wailsApp = () => window.go?.main?.App;

const hasWails = () => Boolean(wailsApp());

const seedSources = (): VoiceSource[] => {
  const createdAt = nowIso();
  return [
    {
      id: "src-demo",
      name: "데모 목소리",
      speaker: "구보이스",
      note: "브라우저 fallback 샘플",
      targetSamples: DEFAULT_SAMPLE_TARGET,
      createdAt,
      updatedAt: createdAt,
      samples: SAMPLE_PROMPTS.slice(0, DEFAULT_SAMPLE_TARGET).map((prompt, index) => ({
        id: `sample-demo-${index + 1}`,
        promptId: prompt.id,
        label: prompt.label,
        text: prompt.text,
        duration: 2 + index * 0.2,
        origin: index % 2 === 0 ? "recording" : "upload",
        createdAt
      }))
    }
  ];
};

const readSources = (): VoiceSource[] => {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    const seeded = seedSources();
    writeSources(seeded);
    return seeded;
  }

  try {
    const parsed = JSON.parse(raw) as VoiceSource[];
    return Array.isArray(parsed) ? parsed : seedSources();
  } catch {
    return seedSources();
  }
};

const writeSources = (sources: VoiceSource[]) => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(sources));
};

const normalizeSource = (source: VoiceSource): VoiceSource => ({
  ...source,
  name: source.name || "새 목소리",
  speaker: source.speaker || "이름 없음",
  note: source.note || "",
  samples: Array.isArray(source.samples) ? source.samples : [],
  targetSamples: normalizeTargetSamples(source.targetSamples)
});

const normalizeOutputDirectory = (
  value?: Partial<OutputDirectorySettings> | string | null,
  source: OutputDirectorySettings["source"] = hasWails() ? "wails" : "browser"
): OutputDirectorySettings => {
  if (typeof value === "string") {
    const path = value.trim();
    return {
      path,
      defaultPath: "",
      isDefault: !path,
      source,
      message: path ? "저장 위치를 불러왔습니다." : "기본 저장 위치를 사용합니다."
    };
  }

  const path = value?.path?.trim() ?? "";
  const defaultPath = value?.defaultPath?.trim() ?? "";
  const isDefault = value?.isDefault ?? !path;
  return {
    path: isDefault ? "" : path,
    defaultPath,
    isDefault,
    source: value?.source ?? source,
    message: value?.message ?? (isDefault ? "기본 저장 위치를 사용합니다." : "저장 위치를 불러왔습니다.")
  };
};

const normalizeSpeechLibrarySettings = (
  value?: Partial<SpeechLibrarySettings> | string | null,
  source: SpeechLibrarySettings["source"] = hasWails() ? "wails" : "browser"
): SpeechLibrarySettings => {
  if (typeof value === "string") {
    const path = value.trim();
    return {
      path,
      defaultPath: "",
      isDefault: !path,
      source,
      message: path ? "말하기 저장 폴더를 불러왔습니다." : "기본 말하기 저장 폴더를 사용합니다."
    };
  }

  const path = value?.path?.trim() ?? "";
  const defaultPath = value?.defaultPath?.trim() ?? "";
  const isDefault = value?.isDefault ?? !path;
  return {
    path: isDefault ? "" : path,
    defaultPath,
    isDefault,
    source: value?.source ?? source,
    message:
      value?.message ??
      (isDefault ? "기본 말하기 저장 폴더를 사용합니다." : "말하기 저장 폴더를 불러왔습니다.")
  };
};

const normalizeSpeechItem = (item: LegacySpeechItem): SpeechItem => {
  const duration =
    Number.isFinite(item.duration) && item.duration !== undefined
      ? item.duration
      : Number.isFinite(item.Duration) && item.Duration !== undefined
        ? item.Duration
        : Number.isFinite(item.durationSeconds) && item.durationSeconds !== undefined
          ? item.durationSeconds
          : Number.isFinite(item.durationMillis) && item.durationMillis !== undefined
            ? item.durationMillis / 1000
            : Number.isFinite(item.DurationMillis) && item.DurationMillis !== undefined
              ? item.DurationMillis / 1000
              : 0;
  const title = item.title?.trim() || item.Title?.trim() || "";
  const text = item.text?.trim() || title;
  const sourceName = item.sourceName?.trim() || item.SourceName?.trim() || "이름 없는 소스";

  return {
    ...item,
    id: item.id || createId("speech"),
    sourceId: item.sourceId || item.SourceID || "",
    sourceName,
    title: title || undefined,
    text,
    duration,
    createdAt: item.createdAt || item.CreatedAt || nowIso(),
    audioName: item.audioName || item.fileName || item.FileName,
    audioUrl: item.audioUrl || item.AudioURL || item.AudioUrl,
    path: item.path
  };
};

const asRecord = (value: unknown): LooseRecord =>
  value && typeof value === "object" ? (value as LooseRecord) : {};

const stringValue = (...values: unknown[]) => {
  for (const value of values) {
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return "";
};

const numberValue = (...values: unknown[]) => {
  for (const value of values) {
    const number = typeof value === "number" ? value : typeof value === "string" ? Number(value) : Number.NaN;
    if (Number.isFinite(number)) {
      return number;
    }
  }
  return 0;
};

const stringArrayValue = (...values: unknown[]) => {
  for (const value of values) {
    if (Array.isArray(value)) {
      return value.map((item) => String(item).trim()).filter(Boolean);
    }
  }
  return undefined;
};

const normalizeAudioDataUrl = (value?: string) => {
  const trimmed = value?.trim() ?? "";
  if (!trimmed) {
    return "";
  }
  return trimmed.startsWith("data:") ? trimmed : `data:audio/wav;base64,${trimmed}`;
};

const promptById = (promptId: string) => SAMPLE_PROMPTS.find((prompt) => prompt.id === promptId);

const stableIdPart = (value: string | number) =>
  String(value)
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9가-힣_-]+/gi, "-")
    .replace(/^-+|-+$/g, "") || "unknown";

const normalizeSentencePrompt = (prompt: LegacySentencePrompt): SentencePrompt => {
  const record = asRecord(prompt);
  const id =
    stringValue(prompt.id, prompt.ID, record.promptID, record.promptId, record.prompt_id, prompt.PromptID, prompt.PromptId) ||
    createId("sentence-prompt");
  const text = stringValue(prompt.text, prompt.Text, record.sentenceText, record.sentence_text);
  const title = stringValue(prompt.title, prompt.Title, record.name) || (text ? speechTitleFromText(text) : id);
  const coveredPromptIds = stringArrayValue(
    prompt.coveredPromptIds,
    prompt.CoveredPromptIDs,
    prompt.CoveredPromptIds,
    prompt.covered_prompt_ids,
    record.coveredPromptIDs,
    record.coveredPromptIds,
    record.covered_prompt_ids
  );
  const promptIds =
    stringArrayValue(
      prompt.promptIds,
      prompt.PromptIDs,
      prompt.PromptIds,
      prompt.prompt_ids,
      record.promptIDs,
      record.promptIds,
      record.prompt_ids
    ) ?? coveredPromptIds;

  return {
    id,
    title,
    text,
    description: stringValue(prompt.description, prompt.Description) || undefined,
    coveredPromptIds,
    promptIds
  };
};

const normalizeSentenceCandidate = (candidate: LegacySentenceCandidate, index = 0): SentenceSampleCandidate => {
  const record = asRecord(candidate);
  const promptId = stringValue(
    candidate.promptId,
    candidate.PromptID,
    candidate.PromptId,
    record.promptID,
    record.promptId,
    record.prompt_id
  );
  const prompt = promptById(promptId);
  const startSeconds = Math.max(
    0,
    numberValue(candidate.startSeconds, candidate.StartSeconds, candidate.Start, record.start, record.start_seconds)
  );
  const duration = Math.max(
    0,
    numberValue(candidate.duration, candidate.Duration, candidate.DurationSeconds, record.duration_seconds)
  );
  const endSeconds = Math.max(
    startSeconds,
    numberValue(candidate.endSeconds, candidate.EndSeconds, candidate.End, record.end, record.end_seconds) ||
      startSeconds + duration
  );
  const normalizedDuration = duration || Math.max(0, endSeconds - startSeconds);
  const confidenceValue = numberValue(candidate.confidence, candidate.Confidence, candidate.Score, record.score);
  const dataBase64 = normalizeAudioDataUrl(
    stringValue(candidate.dataBase64, candidate.DataBase64, candidate.data, record.dataBase64, record.data_base64)
  );
  const audioUrl =
    stringValue(candidate.audioUrl, candidate.AudioURL, candidate.AudioUrl, record.audioURL, record.audioUrl, record.audio_url) ||
    dataBase64;
  const audioName =
    stringValue(candidate.audioName, candidate.AudioName) ||
    `${promptId || `candidate-${index + 1}`}.wav`;
  const fallbackId = `candidate-${stableIdPart(promptId || index + 1)}-${stableIdPart(audioName)}-${Math.round(
    startSeconds * 1000
  )}-${Math.round(endSeconds * 1000)}`;

  return {
    id: stringValue(candidate.id, candidate.ID, record.id, record.ID) || fallbackId,
    promptId,
    label: stringValue(candidate.label, candidate.Label) || prompt?.label || promptId || `후보 ${index + 1}`,
    text: stringValue(candidate.text, candidate.Text) || prompt?.text || "",
    startSeconds,
    endSeconds,
    duration: normalizedDuration,
    confidence: Math.max(0, Math.min(1, confidenceValue > 1 ? confidenceValue / 100 : confidenceValue)),
    status: stringValue(candidate.status, candidate.Status) || undefined,
    warning: stringValue(candidate.warning, candidate.Warning) || undefined,
    audioName,
    audioUrl,
    dataBase64
  };
};

const normalizeSentenceResult = (result?: LegacySentenceResult | null): SentenceExtractionResult => {
  const record = asRecord(result);
  const promptRaw = result?.prompt ?? result?.Prompt ?? record.prompt;
  const prompt = promptRaw && typeof promptRaw === "object" ? normalizeSentencePrompt(promptRaw as LegacySentencePrompt) : undefined;
  const candidatesRaw = result?.candidates ?? result?.Candidates ?? record.candidates ?? [];
  const candidates = Array.isArray(candidatesRaw)
    ? candidatesRaw.map((candidate, index) => normalizeSentenceCandidate(candidate, index))
    : [];
  const warnings =
    stringArrayValue(result?.warnings, result?.Warnings, record.warnings) ??
    [stringValue(result?.warning, record.warning)].filter(Boolean);
  const promptId =
    stringValue(result?.promptId, result?.PromptID, result?.PromptId, record.promptID, record.promptId, record.prompt_id) ||
    prompt?.id ||
    undefined;

  return {
    prompt,
    promptId,
    text: stringValue(result?.text, result?.Text, record.text, prompt?.text),
    sourceDuration:
      numberValue(result?.sourceDuration, result?.SourceDuration, record.sourceDuration, record.source_duration) || undefined,
    trimmedDuration:
      numberValue(result?.trimmedDuration, result?.TrimmedDuration, record.trimmedDuration, record.trimmed_duration) || undefined,
    totalCandidates:
      numberValue(result?.totalCandidates, result?.TotalCandidates, result?.Total, record.totalCandidates, record.total_candidates) ||
      candidates.length,
    candidates,
    warnings
  };
};

const readSpeechItems = (): SpeechItem[] => {
  const raw = localStorage.getItem(SPEECH_LIBRARY_KEY);
  if (!raw) {
    return [];
  }

  try {
    const parsed = JSON.parse(raw) as SpeechItem[];
    return Array.isArray(parsed) ? parsed.map(normalizeSpeechItem) : [];
  } catch {
    return [];
  }
};

const writeSpeechItems = (items: SpeechItem[]) => {
  localStorage.setItem(SPEECH_LIBRARY_KEY, JSON.stringify(items.map(normalizeSpeechItem)));
};

const speechTitleFromText = (text: string) => {
  const normalized = text.trim().replace(/\s+/g, " ");
  return normalized.length > 28 ? `${normalized.slice(0, 28)}...` : normalized;
};

const extractTokens = (text: string) => {
  const normalized = text
    .replace(/[^\p{Script=Hangul}a-zA-Z0-9]/gu, " ")
    .replace(/\s+/g, " ")
    .trim();

  if (!normalized) {
    return [];
  }

  return Array.from(new Set(Array.from(normalized.replace(/\s/g, ""))));
};

const analyzeFromSamples = (source: VoiceSource, text: string): AnalysisResult => {
  const requiredTokens = extractTokens(text);
  const target = normalizeTargetSamples(source.targetSamples);
  const requiredPrompts = SAMPLE_PROMPTS.slice(0, target);
  const filledPromptIds = new Set(source.samples.map((sample) => sample.promptId ?? sample.label));
  const missingPrompts = requiredPrompts.filter((prompt) => !filledPromptIds.has(prompt.id));
  const filled = requiredPrompts.length - missingPrompts.length;
  const missing: MissingSample[] = [];

  for (const prompt of missingPrompts.slice(0, 10)) {
    missing.push({
      token: prompt.text,
      reason: `${prompt.label} 샘플이 없습니다.`,
      severity: "missing"
    });
  }

  if (missingPrompts.length > 10) {
    missing.push({
      token: `+${missingPrompts.length - 10}개`,
      reason: "추가 필수 샘플이 더 필요합니다.",
      severity: "missing"
    });
  }

  if (!text.trim()) {
    missing.push({
      token: "텍스트",
      reason: "말할 문장을 입력해야 합니다.",
      severity: "warn"
    });
  }

  return {
    coverage: Math.round((filled / target) * 100),
    matched: filled,
    required: Math.max(target, requiredTokens.length),
    missing
  };
};

const createDemoWav = (text: string) => {
  const sampleRate = 8000;
  const duration = Math.min(2.2, Math.max(0.8, text.length / 34));
  const sampleCount = Math.floor(sampleRate * duration);
  const dataSize = sampleCount * 2;
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

  for (let i = 0; i < sampleCount; i += 1) {
    const envelope = 1 - i / sampleCount;
    const frequency = 180 + (text.charCodeAt(i % Math.max(text.length, 1)) % 90);
    const sample = Math.sin((i / sampleRate) * Math.PI * 2 * frequency);
    view.setInt16(44 + i * 2, sample * 0x4fff * envelope, true);
  }

  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.length; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }

  return `data:audio/wav;base64,${btoa(binary)}`;
};

const demoSpeechDuration = (text: string) => Math.min(2.2, Math.max(0.8, text.length / 34));

const fallbackApi = {
  async listSources() {
    return readSources().map(normalizeSource);
  },

  async createSource(input: CreateSourceInput) {
    const createdAt = nowIso();
    const source: VoiceSource = {
      id: createId("src"),
      name: input.name?.trim() || "새 목소리",
      speaker: input.speaker?.trim() || "이름 없음",
      note: input.note?.trim() || "",
      targetSamples: normalizeTargetSamples(input.targetSamples),
      samples: [],
      createdAt,
      updatedAt: createdAt
    };
    const sources = [source, ...readSources()];
    writeSources(sources);
    return source;
  },

  async updateSource(id: string, patch: Partial<VoiceSource>) {
    const sources = readSources();
    const nextSources = sources.map((source) =>
      source.id === id
        ? normalizeSource({
            ...source,
            ...patch,
            id: source.id,
            updatedAt: nowIso()
          })
        : source
    );
    writeSources(nextSources);
    return nextSources.find((source) => source.id === id) ?? nextSources[0];
  },

  async deleteSource(id: string) {
    writeSources(readSources().filter((source) => source.id !== id));
  },

  async addSample(sourceId: string, sample: AddSampleInput) {
    const sources = readSources();
    const nextSample: VoiceSample = {
      ...sample,
      id: createId("sample"),
      createdAt: nowIso()
    };
    const nextSources = sources.map((source) =>
      source.id === sourceId
        ? {
            ...source,
            samples: [nextSample, ...source.samples],
            updatedAt: nowIso()
          }
        : source
    );
    writeSources(nextSources);
    return nextSources.find((source) => source.id === sourceId) ?? nextSources[0];
  },

  async analyzeText(sourceId: string, text: string) {
    const source = readSources().find((item) => item.id === sourceId);
    if (!source) {
      return {
        coverage: 0,
        matched: 0,
        required: 0,
        missing: []
      };
    }
    return analyzeFromSamples(source, text);
  },

  async synthesize(request: SynthesisRequest): Promise<PreviewResult> {
    return {
      id: createId("preview"),
      status: "ready",
      message: "브라우저 데모 미리듣기를 생성했습니다.",
      audioUrl: createDemoWav(request.text)
    } satisfies PreviewResult;
  },

  async exportMP3(_request: SynthesisRequest): Promise<ExportResult> {
    return {
      status: "error",
      message: "브라우저 데모는 MP3 저장을 지원하지 않습니다. Wails 앱에서 저장해 주세요."
    } satisfies ExportResult;
  },

  async getEngineStatus() {
    return {
      mode: "browser",
      label: "브라우저 데모",
      ready: true,
      message: "Wails 바인딩이 없어 localStorage fallback을 사용합니다."
    } satisfies EngineStatus;
  },

  async getOutputDirectory() {
    return normalizeOutputDirectory(localStorage.getItem(OUTPUT_DIRECTORY_KEY) ?? "", "browser");
  },

  async setOutputDirectory(path: string) {
    const trimmed = path.trim();
    localStorage.setItem(OUTPUT_DIRECTORY_KEY, trimmed);
    return normalizeOutputDirectory(trimmed, "browser");
  },

  async chooseOutputDirectory() {
    return {
      ...(await fallbackApi.getOutputDirectory()),
      message: "브라우저 데모에서는 폴더 선택 창을 열 수 없습니다. 경로를 직접 입력해 주세요."
    } satisfies OutputDirectorySettings;
  },

  async openOutputDirectory() {
    return {
      ...(await fallbackApi.getOutputDirectory()),
      message: "브라우저에서는 로컬 폴더를 직접 열 수 없습니다. Wails 데스크톱 앱에서 실행하거나 경로를 직접 열어 주세요."
    } satisfies OutputDirectorySettings;
  },

  async getSpeechLibrarySettings() {
    return normalizeSpeechLibrarySettings(
      localStorage.getItem(SPEECH_LIBRARY_DIRECTORY_KEY) ?? "",
      "browser"
    );
  },

  async setSpeechLibraryDirectory(path: string) {
    const trimmed = path.trim();
    localStorage.setItem(SPEECH_LIBRARY_DIRECTORY_KEY, trimmed);
    return normalizeSpeechLibrarySettings(trimmed, "browser");
  },

  async chooseSpeechLibraryDirectory() {
    return {
      ...(await fallbackApi.getSpeechLibrarySettings()),
      message: "브라우저 데모에서는 말하기 저장 폴더 선택 창을 열 수 없습니다. 경로를 직접 입력해 주세요."
    } satisfies SpeechLibrarySettings;
  },

  async openSpeechLibraryDirectory() {
    return {
      ...(await fallbackApi.getSpeechLibrarySettings()),
      message: "브라우저에서는 로컬 폴더를 직접 열 수 없습니다. Wails 데스크톱 앱에서 실행하거나 경로를 직접 열어 주세요."
    } satisfies SpeechLibrarySettings;
  },

  async listSpeechItems() {
    return readSpeechItems();
  },

  async getSpeechItemAudio(id: string) {
    const item = readSpeechItems().find((speechItem) => speechItem.id === id);
    return item?.audioUrl || createDemoWav(item?.text || item?.title || "guvoice");
  },

  async saveSpeechItem(input: SaveSpeechItemInput) {
    const sourceName =
      input.sourceName?.trim() ||
      readSources().find((source) => source.id === input.sourceId)?.name ||
      "구보이스";
    const item: SpeechItem = {
      id: createId("speech"),
      sourceId: input.sourceId,
      sourceName,
      title: speechTitleFromText(input.text),
      text: input.text,
      duration: demoSpeechDuration(input.text),
      createdAt: nowIso(),
      audioName: `${sourceName.replace(/\s+/g, "-") || "guvoice"}-${Date.now()}.wav`,
      audioUrl: createDemoWav(input.text)
    };
    const items = [item, ...readSpeechItems()];
    writeSpeechItems(items);
    return item;
  },

  async deleteSpeechItem(id: string) {
    writeSpeechItems(readSpeechItems().filter((item) => item.id !== id));
  },

  async listSentencePrompts() {
    return DEMO_SENTENCE_PROMPTS;
  },

  async extractSentenceSamples(input: SentenceExtractionInput): Promise<SentenceExtractionResult> {
    const promptPack =
      DEMO_SENTENCE_PROMPTS.find((prompt) => prompt.id === (input.sentencePromptId || input.promptId)) ??
      DEMO_SENTENCE_PROMPTS[0];
    const target = normalizeTargetSamples(input.targetSamples);
    const requiredPromptIds = new Set(SAMPLE_PROMPTS.slice(0, target).map((prompt) => prompt.id));
    const packPromptIds = (promptPack.promptIds?.length ? promptPack.promptIds : SAMPLE_PROMPTS.map((prompt) => prompt.id)).filter(
      (promptId) => requiredPromptIds.has(promptId)
    );
    const promptIds = (packPromptIds.length ? packPromptIds : Array.from(requiredPromptIds)).slice(0, 8);
    const candidates = promptIds.map((promptId, index) => {
      const prompt = promptById(promptId) ?? SAMPLE_PROMPTS[index % SAMPLE_PROMPTS.length];
      const duration = Math.min(0.9, Math.max(0.28, prompt.text.length * 0.08));
      const startSeconds = index * 0.44;
      const audioUrl = createDemoWav(prompt.text || input.text);
      return normalizeSentenceCandidate(
        {
          id: `demo-candidate-${promptId}`,
          promptId,
          label: prompt.label,
          text: prompt.text,
          startSeconds,
          endSeconds: startSeconds + duration,
          duration,
          confidence: Math.max(0.58, 0.92 - index * 0.04),
          status: index % 5 === 4 ? "review" : "usable",
          warning: index % 5 === 4 ? "데모 후보라 저장 전 확인해 주세요." : undefined,
          audioName: `demo-${promptId}.wav`,
          audioUrl,
          dataBase64: audioUrl
        },
        index
      );
    });

    return {
      prompt: promptPack,
      promptId: promptPack.id,
      text: input.text,
      totalCandidates: candidates.length,
      candidates,
      warnings: ["Browser fallback이 만든 데모 후보입니다."]
    };
  }
};

const callWails = async <T>(
  methodName: keyof WailsApp,
  fallback: () => Promise<T>,
  ...args: unknown[]
): Promise<T> => {
  const app = wailsApp();
  const method = app?.[methodName];
  if (!method) {
    return fallback();
  }

  return (await (method as (...methodArgs: unknown[]) => Promise<T> | T)(...args)) as T;
};

export const voiceApi = {
  mode: () => (hasWails() ? "wails" : "browser"),

  listSources: () =>
    callWails("ListSources", fallbackApi.listSources).then((sources) =>
      sources.map(normalizeSource)
    ),

  createSource: (input: CreateSourceInput) => {
    const normalizedInput = {
      ...input,
      targetSamples: normalizeTargetSamples(input.targetSamples)
    };
    return callWails("CreateSource", () => fallbackApi.createSource(normalizedInput), normalizedInput).then(normalizeSource);
  },

  updateSource: (id: string, patch: Partial<VoiceSource>) => {
    const normalizedPatch =
      patch.targetSamples === undefined
        ? patch
        : {
            ...patch,
            targetSamples: normalizeTargetSamples(patch.targetSamples)
          };
    return callWails("UpdateSource", () => fallbackApi.updateSource(id, normalizedPatch), id, normalizedPatch).then(
      normalizeSource
    );
  },

  deleteSource: (id: string) =>
    callWails("DeleteSource", () => fallbackApi.deleteSource(id), id),

  addSample: (sourceId: string, sample: AddSampleInput) =>
    callWails("AddSample", () => fallbackApi.addSample(sourceId, sample), sourceId, sample),

  analyzeText: (sourceId: string, text: string) =>
    callWails("AnalyzeText", () => fallbackApi.analyzeText(sourceId, text), sourceId, text),

  synthesize: (request: SynthesisRequest) =>
    callWails("Synthesize", () => fallbackApi.synthesize(request), request),

  exportMP3: (request: SynthesisRequest) =>
    callWails("ExportMP3", () => fallbackApi.exportMP3(request), request),

  getEngineStatus: () => callWails("GetEngineStatus", fallbackApi.getEngineStatus),

  getOutputDirectory: () =>
    callWails("GetOutputDirectory", fallbackApi.getOutputDirectory).then((value) =>
      normalizeOutputDirectory(value)
    ),

  setOutputDirectory: (path: string) =>
    callWails("SetOutputDirectory", () => fallbackApi.setOutputDirectory(path), path).then((value) =>
      normalizeOutputDirectory(value)
    ),

  chooseOutputDirectory: () =>
    callWails("ChooseOutputDirectory", fallbackApi.chooseOutputDirectory).then((value) =>
      normalizeOutputDirectory(value)
    ),

  openOutputDirectory: () =>
    callWails("OpenOutputDirectory", fallbackApi.openOutputDirectory).then((value) =>
      normalizeOutputDirectory(value)
    ),

  getSpeechLibrarySettings: () =>
    callWails("GetSpeechLibrarySettings", fallbackApi.getSpeechLibrarySettings).then((value) =>
      normalizeSpeechLibrarySettings(value)
    ),

  setSpeechLibraryDirectory: (path: string) =>
    callWails(
      "SetSpeechLibraryDirectory",
      () => fallbackApi.setSpeechLibraryDirectory(path),
      path
    ).then((value) => normalizeSpeechLibrarySettings(value)),

  chooseSpeechLibraryDirectory: () =>
    callWails("ChooseSpeechLibraryDirectory", fallbackApi.chooseSpeechLibraryDirectory).then((value) =>
      normalizeSpeechLibrarySettings(value)
    ),

  openSpeechLibraryDirectory: () =>
    callWails("OpenSpeechLibraryDirectory", fallbackApi.openSpeechLibraryDirectory).then((value) =>
      normalizeSpeechLibrarySettings(value)
    ),

  listSpeechItems: () =>
    callWails("ListSpeechItems", fallbackApi.listSpeechItems).then((items) =>
      items.map(normalizeSpeechItem)
    ),

  getSpeechItemAudio: async (id: string) => {
    const app = wailsApp();
    const method = app?.GetSpeechItemAudio ?? app?.GetSpeechItemAudioURL;
    if (!method) {
      return fallbackApi.getSpeechItemAudio(id);
    }
    return (await method(id)) || fallbackApi.getSpeechItemAudio(id);
  },

  saveSpeechItem: (input: SaveSpeechItemInput) =>
    callWails("SaveSpeechItem", () => fallbackApi.saveSpeechItem(input), input).then(
      normalizeSpeechItem
    ),

  deleteSpeechItem: (id: string) =>
    callWails("DeleteSpeechItem", () => fallbackApi.deleteSpeechItem(id), id),

  listSentencePrompts: () =>
    callWails("ListSentencePrompts", fallbackApi.listSentencePrompts).then((prompts) =>
      (Array.isArray(prompts) ? prompts : DEMO_SENTENCE_PROMPTS).map(normalizeSentencePrompt)
    ),

  extractSentenceSamples: (input: SentenceExtractionInput) => {
    const normalizedInput = {
      ...input,
      targetSamples: normalizeTargetSamples(input.targetSamples)
    };
    return callWails("ExtractSentenceSamples", () => fallbackApi.extractSentenceSamples(normalizedInput), normalizedInput).then(
      normalizeSentenceResult
    );
  }
};

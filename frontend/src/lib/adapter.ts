import {
  AnalysisResult,
  EngineStatus,
  ExportResult,
  MAX_SAMPLE_TARGET,
  MIN_SAMPLE_TARGET,
  MissingSample,
  OutputDirectorySettings,
  PreviewResult,
  SAMPLE_PROMPTS,
  SynthesisRequest,
  VoiceSample,
  VoiceSource
} from "../types";

type CreateSourceInput = Partial<
  Pick<VoiceSource, "name" | "speaker" | "note" | "targetSamples">
>;

type AddSampleInput = Omit<VoiceSample, "id" | "createdAt">;

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

const nowIso = () => new Date().toISOString();

const createId = (prefix: string) => {
  if (crypto.randomUUID) {
    return `${prefix}-${crypto.randomUUID()}`;
  }
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
};

const wailsApp = () => window.go?.main?.App;

const hasWails = () => Boolean(wailsApp());

const clampTargetSamples = (value?: number) =>
  Math.min(MAX_SAMPLE_TARGET, Math.max(MIN_SAMPLE_TARGET, value || MIN_SAMPLE_TARGET));

const seedSources = (): VoiceSource[] => {
  const createdAt = nowIso();
  return [
    {
      id: "src-demo",
      name: "데모 목소리",
      speaker: "구보이스",
      note: "브라우저 fallback 샘플",
      targetSamples: MIN_SAMPLE_TARGET,
      createdAt,
      updatedAt: createdAt,
      samples: SAMPLE_PROMPTS.slice(0, MIN_SAMPLE_TARGET).map((prompt, index) => ({
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
  targetSamples: clampTargetSamples(source.targetSamples)
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
  const target = clampTargetSamples(source.targetSamples);
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
      targetSamples: clampTargetSamples(input.targetSamples),
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
      ...(await this.getOutputDirectory()),
      message: "브라우저 데모에서는 폴더 선택 창을 열 수 없습니다. 경로를 직접 입력해 주세요."
    } satisfies OutputDirectorySettings;
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

  createSource: (input: CreateSourceInput) =>
    callWails("CreateSource", () => fallbackApi.createSource(input), input),

  updateSource: (id: string, patch: Partial<VoiceSource>) =>
    callWails("UpdateSource", () => fallbackApi.updateSource(id, patch), id, patch),

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
    )
};

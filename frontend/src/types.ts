export type SourceOrigin = "recording" | "upload";

export type TabId = "speak" | "record" | "library" | "manage";

export interface VoiceSample {
  id: string;
  promptId?: string;
  label: string;
  text: string;
  duration: number;
  origin: SourceOrigin;
  createdAt: string;
  audioName?: string;
  audioUrl?: string;
  dataBase64?: string;
}

export interface VoiceSource {
  id: string;
  name: string;
  speaker: string;
  note: string;
  targetSamples: number;
  samples: VoiceSample[];
  createdAt: string;
  updatedAt: string;
}

export interface MissingSample {
  token: string;
  promptId?: string;
  reason: string;
  severity: "missing" | "warn";
}

export interface AnalysisResult {
  coverage: number;
  matched: number;
  required: number;
  missing: MissingSample[];
}

export interface SynthesisOptions {
  speed: number;
  pitch: number;
  clarity: number;
  noiseReduction: number;
}

export interface SynthesisRequest {
  sourceId: string;
  text: string;
  options: SynthesisOptions;
}

export interface PreviewResult {
  id: string;
  status: "ready" | "queued" | "error";
  message: string;
  audioUrl?: string;
}

export interface ExportResult {
  status: "saved" | "ready" | "error";
  message: string;
  path?: string;
  downloadUrl?: string;
}

export interface EngineStatus {
  mode: "wails" | "browser";
  label: string;
  ready: boolean;
  message: string;
}

export interface OutputDirectorySettings {
  path: string;
  defaultPath: string;
  isDefault: boolean;
  source: "wails" | "browser";
  message: string;
}

export interface SpeechLibrarySettings {
  path: string;
  defaultPath: string;
  isDefault: boolean;
  source: "wails" | "browser";
  message: string;
}

export interface SpeechItem {
  id: string;
  sourceId: string;
  sourceName: string;
  title?: string;
  text: string;
  duration: number;
  createdAt: string;
  audioName?: string;
  audioUrl?: string;
  path?: string;
}

export interface SaveSpeechItemInput extends SynthesisRequest {
  sourceName?: string;
}

export interface SamplePrompt {
  id: string;
  label: string;
  text: string;
}

export interface SampleTargetOption {
  value: number;
  label: string;
  description: string;
}

export interface SentencePrompt {
  id: string;
  title: string;
  text: string;
  description?: string;
  coveredPromptIds?: string[];
  promptIds?: string[];
}

export interface SentenceExtractionInput {
  promptId?: string;
  sentencePromptId?: string;
  targetSamples?: number;
  text: string;
  audioName: string;
  dataBase64?: string;
  audioUrl?: string;
}

export interface SentenceSampleCandidate {
  id: string;
  promptId: string;
  label: string;
  text: string;
  startSeconds: number;
  endSeconds: number;
  duration: number;
  confidence: number;
  status?: string;
  warning?: string;
  audioName: string;
  audioUrl?: string;
  dataBase64?: string;
}

export interface SentenceExtractionResult {
  prompt?: SentencePrompt;
  promptId?: string;
  text: string;
  sourceDuration?: number;
  trimmedDuration?: number;
  totalCandidates: number;
  candidates: SentenceSampleCandidate[];
  warnings?: string[];
}

const HANGUL_BASE = 0xac00;
const HANGUL_CHOSEONG_COUNT = 19;
const HANGUL_JUNGSEONG_COUNT = 21;
const HANGUL_JONGSEONG_COUNT = 28;
const MAX_GENERATED_SAMPLE_TARGET = 300;
const BALANCED_JUNGSEONG_ORDER = [0, 4, 1, 5, 8, 13, 18, 20, 2, 6, 12, 17, 9, 14, 11, 16, 3, 7, 10, 15, 19];
const COMMON_FINAL_CONSONANT_ORDER = [4, 21, 8, 16, 1, 17, 7, 19, 20, 22, 27];

const guvoiceMinimalPromptCatalog: SamplePrompt[] = [
  { id: "vowel-a", label: "모음 아", text: "아" },
  { id: "vowel-eo", label: "모음 어", text: "어" },
  { id: "vowel-o", label: "모음 오", text: "오" },
  { id: "vowel-u", label: "모음 우", text: "우" },
  { id: "vowel-eu", label: "모음 으", text: "으" },
  { id: "vowel-i", label: "모음 이", text: "이" },
  { id: "vowel-ae", label: "모음 애", text: "애" },
  { id: "vowel-e", label: "모음 에", text: "에" },
  { id: "vowel-ya", label: "모음 야", text: "야" },
  { id: "vowel-yeo", label: "모음 여", text: "여" },
  { id: "vowel-yo", label: "모음 요", text: "요" },
  { id: "vowel-yu", label: "모음 유", text: "유" },
  { id: "rep-ga", label: "대표음 가", text: "가" },
  { id: "rep-na", label: "대표음 나", text: "나" },
  { id: "rep-da", label: "대표음 다", text: "다" },
  { id: "rep-ra", label: "대표음 라", text: "라" },
  { id: "rep-ma", label: "대표음 마", text: "마" },
  { id: "rep-ba", label: "대표음 바", text: "바" },
  { id: "rep-sa", label: "대표음 사", text: "사" },
  { id: "rep-ja", label: "대표음 자", text: "자" },
  { id: "rep-cha", label: "대표음 차", text: "차" },
  { id: "rep-ka", label: "대표음 카", text: "카" },
  { id: "rep-ta", label: "대표음 타", text: "타" },
  { id: "rep-pa", label: "대표음 파", text: "파" },
  { id: "rep-ha", label: "대표음 하", text: "하" }
];

const composeHangulSyllable = (choseongIndex: number, jungseongIndex: number, jongseongIndex: number) =>
  HANGUL_BASE + ((choseongIndex * HANGUL_JUNGSEONG_COUNT + jungseongIndex) * HANGUL_JONGSEONG_COUNT + jongseongIndex);

const syllablePromptId = (codePoint: number) =>
  `syllable-${codePoint.toString(16).toUpperCase().padStart(4, "0")}`;

const appendExactSyllablePrompt = (
  catalog: SamplePrompt[],
  minimalPromptTexts: Set<string>,
  choseongIndex: number,
  jungseongIndex: number,
  jongseongIndex: number
) => {
  if (catalog.length >= MAX_GENERATED_SAMPLE_TARGET) {
    return;
  }
  const codePoint = composeHangulSyllable(choseongIndex, jungseongIndex, jongseongIndex);
  const text = String.fromCodePoint(codePoint);
  if (minimalPromptTexts.has(text)) {
    return;
  }
  catalog.push({
    id: syllablePromptId(codePoint),
    label: `\uC815\uD655 \uC74C\uC808 ${text}`,
    text
  });
};

const buildGuvoicePromptCatalog = (): SamplePrompt[] => {
  const catalog = [...guvoiceMinimalPromptCatalog];
  const minimalPromptTexts = new Set(guvoiceMinimalPromptCatalog.map((prompt) => prompt.text));

  for (const jungseongIndex of BALANCED_JUNGSEONG_ORDER) {
    for (let choseongIndex = 0; choseongIndex < HANGUL_CHOSEONG_COUNT; choseongIndex += 1) {
      appendExactSyllablePrompt(catalog, minimalPromptTexts, choseongIndex, jungseongIndex, 0);
      if (catalog.length >= MAX_GENERATED_SAMPLE_TARGET) {
        return catalog;
      }
    }
  }

  for (const jongseongIndex of COMMON_FINAL_CONSONANT_ORDER) {
    for (const jungseongIndex of BALANCED_JUNGSEONG_ORDER) {
      for (let choseongIndex = 0; choseongIndex < HANGUL_CHOSEONG_COUNT; choseongIndex += 1) {
        appendExactSyllablePrompt(catalog, minimalPromptTexts, choseongIndex, jungseongIndex, jongseongIndex);
        if (catalog.length >= MAX_GENERATED_SAMPLE_TARGET) {
          return catalog;
        }
      }
    }
  }
  return catalog;
};

export const SAMPLE_PROMPTS: SamplePrompt[] = buildGuvoicePromptCatalog();

export const SAMPLE_TARGET_OPTIONS: SampleTargetOption[] = [
  {
    value: 25,
    label: "부정확",
    description: "25개, 지금 같은 작은 소스로 빠르게 시험합니다."
  },
  {
    value: 100,
    label: "나름 정확",
    description: "100개, 자주 쓰는 정확 음절을 더 녹음합니다."
  },
  {
    value: 200,
    label: "꽤 정확",
    description: "200개, 더 넓은 음절을 커버합니다."
  },
  {
    value: 300,
    label: "아주 정확함",
    description: "300개, 현재 프리셋에서 가장 촘촘하게 녹음합니다."
  }
];

export const MIN_SAMPLE_TARGET = 25;
export const MAX_SAMPLE_TARGET = MAX_GENERATED_SAMPLE_TARGET;
export const MIN_PREVIEW_SAMPLES = Math.min(3, SAMPLE_PROMPTS.length);

export const normalizeTargetSamples = (value?: number) => {
  const fallback = SAMPLE_TARGET_OPTIONS[0]?.value ?? MIN_SAMPLE_TARGET;
  const numeric = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(numeric)) {
    return fallback;
  }
  return (
    SAMPLE_TARGET_OPTIONS.find((option) => numeric <= option.value)?.value ??
    SAMPLE_TARGET_OPTIONS[SAMPLE_TARGET_OPTIONS.length - 1]?.value ??
    MAX_SAMPLE_TARGET
  );
};

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

export const SAMPLE_PROMPTS: SamplePrompt[] = [
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
  { id: "rep-ha", label: "대표음 하", text: "하" },
  { id: "tone-soft", label: "부드러운 톤", text: "오늘은 맑고 차분하게 말합니다." },
  { id: "tone-fast", label: "빠른 톤", text: "작은 소리도 또렷하게 읽어 보겠습니다." },
  { id: "tone-question", label: "질문 톤", text: "이 설정으로 미리듣기를 만들어 볼까요?" }
];

export const MIN_SAMPLE_TARGET = Math.min(25, SAMPLE_PROMPTS.length);
export const MAX_SAMPLE_TARGET = SAMPLE_PROMPTS.length;
export const MIN_PREVIEW_SAMPLES = Math.min(3, SAMPLE_PROMPTS.length);

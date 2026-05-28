export type SourceOrigin = "recording" | "upload";

export type TabId = "speak" | "record" | "manage";

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

export interface SamplePrompt {
  id: string;
  label: string;
  text: string;
}

export const MIN_SAMPLE_TARGET = 12;
export const MIN_PREVIEW_SAMPLES = 3;

export const SAMPLE_PROMPTS: SamplePrompt[] = [
  {
    id: "vowel-basic",
    label: "기본 모음",
    text: "아 에 이 오 우 으 어 애 야 여 요 유"
  },
  {
    id: "soft-sentence",
    label: "부드러운 문장",
    text: "오늘은 맑고 차분한 목소리로 천천히 안내합니다."
  },
  {
    id: "fast-sentence",
    label: "빠른 문장",
    text: "작은 변화도 놓치지 않고 정확하게 읽어 보겠습니다."
  },
  {
    id: "numbers",
    label: "숫자와 단위",
    text: "일 이 삼 사 오 육 칠 팔 구 십, 스물 하나와 백 퍼센트"
  },
  {
    id: "question",
    label: "질문 억양",
    text: "지금 이 설정으로 미리듣기를 만들어 볼까요?"
  },
  {
    id: "low-tone",
    label: "낮은 톤",
    text: "조용한 밤에도 또렷하게 들리도록 낮게 말합니다."
  },
  {
    id: "bright-tone",
    label: "밝은 톤",
    text: "반갑습니다. 구보이스가 새 목소리 소스를 준비했습니다."
  },
  {
    id: "long-vowels",
    label: "긴 발성",
    text: "가나다라마바사 아자차카타파하를 길게 이어 읽습니다."
  },
  {
    id: "consonants",
    label: "자음 대비",
    text: "바 파 다 타 가 카 자 차 사 싸 라 마 나"
  },
  {
    id: "quiet",
    label: "작은 음량",
    text: "가까운 거리에서 작지만 안정적인 음량으로 녹음합니다."
  },
  {
    id: "project",
    label: "프로젝트 문장",
    text: "선택한 텍스트의 누락 샘플을 검사하고 저장 상태를 확인합니다."
  },
  {
    id: "closing",
    label: "마무리 문장",
    text: "이 샘플을 저장하면 최소 샘플셋 진행률이 업데이트됩니다."
  }
];

package catalog

import (
	"fmt"
	"time"

	"guvoice/internal/hangul"
	"guvoice/internal/model"
)

const MinimumKoreanSampleSetID = "minimal-ko-v1"

func MinimumKoreanSampleSet() model.SampleSet {
	items := []model.SamplePrompt{}

	for index, token := range hangul.ChoseongTokens() {
		items = append(items, model.SamplePrompt{
			ID:       hangul.ChoseongPromptID(token),
			Kind:     "jamo_choseong",
			Label:    "초성 " + token,
			Text:     string(hangul.Compose(index, 0, 0)) + " " + string(hangul.Compose(index, 0, 0)) + " " + string(hangul.Compose(index, 0, 0)),
			Tokens:   []string{token},
			Required: true,
			Notes:    "초성 음색 fallback에 사용하는 최소 샘플입니다.",
		})
	}

	for index, token := range hangul.JungseongTokens() {
		items = append(items, model.SamplePrompt{
			ID:       hangul.JungseongPromptID(token),
			Kind:     "jamo_jungseong",
			Label:    "중성 " + token,
			Text:     string(hangul.Compose(11, index, 0)) + " " + string(hangul.Compose(11, index, 0)) + " " + string(hangul.Compose(11, index, 0)),
			Tokens:   []string{token},
			Required: true,
			Notes:    "모음/중성 fallback에 사용하는 최소 샘플입니다.",
		})
	}

	for index, token := range hangul.JongseongTokens() {
		jongIndex := index + 1
		items = append(items, model.SamplePrompt{
			ID:       hangul.JongseongPromptID(token),
			Kind:     "jamo_jongseong",
			Label:    "종성 " + token,
			Text:     string(hangul.Compose(11, 0, jongIndex)) + " " + string(hangul.Compose(11, 0, jongIndex)) + " " + string(hangul.Compose(11, 0, jongIndex)),
			Tokens:   []string{token},
			Required: true,
			Notes:    "받침 fallback에 사용하는 최소 샘플입니다.",
		})
	}

	for i, sentence := range []string{
		"구보이스는 내 목소리를 작은 샘플로 기록합니다.",
		"오늘의 문장은 또렷하고 자연스럽게 읽습니다.",
		"가나다라마바사 아자차카타파하.",
		"밝은 톤과 낮은 톤을 천천히 번갈아 냅니다.",
		"짧은 문장도 긴 문장도 같은 속도로 말합니다.",
	} {
		items = append(items, model.SamplePrompt{
			ID:       fmt.Sprintf("sentence-ko-%03d", i+1),
			Kind:     "sentence",
			Label:    fmt.Sprintf("문장 %d", i+1),
			Text:     sentence,
			Required: false,
			Notes:    "음색 보정과 MVP fallback에 쓰는 문장 샘플입니다.",
		})
	}

	return model.SampleSet{
		ID:          MinimumKoreanSampleSetID,
		Name:        "구보이스 최소 한글 샘플셋",
		Locale:      "ko-KR",
		Description: "한글 음절을 초성/중성/종성으로 분해해 MVP 합성 fallback을 구성하기 위한 최소 샘플셋입니다.",
		Items:       items,
		CreatedAt:   time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
	}
}

func DefaultFallbackPolicy() []model.FallbackPolicy {
	return []model.FallbackPolicy{
		{
			Order:       1,
			Method:      "exact_syllable",
			Description: "텍스트 음절과 같은 promptId(syllable-XXXX)의 샘플이 있으면 우선 사용합니다.",
		},
		{
			Order:       2,
			Method:      "jamo_composition",
			Description: "정확한 음절 샘플이 없으면 초성/중성/종성 jamo 샘플을 조합 대상으로 선택합니다.",
		},
		{
			Order:       3,
			Method:      "source_fallback",
			Description: "jamo 샘플이 부족하지만 해당 목소리 소스에 업로드/문장 샘플이 있으면 MVP 대체 음색으로 표시합니다.",
		},
		{
			Order:       4,
			Method:      "placeholder",
			Description: "사용 가능한 샘플이 없으면 합성 골격 검증을 위한 placeholder WAV를 생성합니다.",
		},
	}
}

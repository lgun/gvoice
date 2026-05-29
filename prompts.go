package main

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"guvoice/internal/hangul"
	"guvoice/internal/model"
	"guvoice/internal/synth"
)

type promptDefinition struct {
	ID    string
	Label string
	Text  string
}

type sentencePromptDefinition struct {
	ID          string
	Title       string
	Text        string
	Description string
}

var guvoicePromptCatalog = []promptDefinition{
	{ID: "vowel-a", Label: "모음 아", Text: "아"},
	{ID: "vowel-eo", Label: "모음 어", Text: "어"},
	{ID: "vowel-o", Label: "모음 오", Text: "오"},
	{ID: "vowel-u", Label: "모음 우", Text: "우"},
	{ID: "vowel-eu", Label: "모음 으", Text: "으"},
	{ID: "vowel-i", Label: "모음 이", Text: "이"},
	{ID: "vowel-ae", Label: "모음 애", Text: "애"},
	{ID: "vowel-e", Label: "모음 에", Text: "에"},
	{ID: "vowel-ya", Label: "모음 야", Text: "야"},
	{ID: "vowel-yeo", Label: "모음 여", Text: "여"},
	{ID: "vowel-yo", Label: "모음 요", Text: "요"},
	{ID: "vowel-yu", Label: "모음 유", Text: "유"},
	{ID: "rep-ga", Label: "대표음 가", Text: "가"},
	{ID: "rep-na", Label: "대표음 나", Text: "나"},
	{ID: "rep-da", Label: "대표음 다", Text: "다"},
	{ID: "rep-ra", Label: "대표음 라", Text: "라"},
	{ID: "rep-ma", Label: "대표음 마", Text: "마"},
	{ID: "rep-ba", Label: "대표음 바", Text: "바"},
	{ID: "rep-sa", Label: "대표음 사", Text: "사"},
	{ID: "rep-ja", Label: "대표음 자", Text: "자"},
	{ID: "rep-cha", Label: "대표음 차", Text: "차"},
	{ID: "rep-ka", Label: "대표음 카", Text: "카"},
	{ID: "rep-ta", Label: "대표음 타", Text: "타"},
	{ID: "rep-pa", Label: "대표음 파", Text: "파"},
	{ID: "rep-ha", Label: "대표음 하", Text: "하"},
	{ID: "tone-soft", Label: "부드러운 톤", Text: "오늘은 맑고 차분하게 말합니다."},
	{ID: "tone-fast", Label: "빠른 톤", Text: "작은 소리로 또렷하게 읽어 보겠습니다."},
	{ID: "tone-question", Label: "질문 톤", Text: "이 설정으로 미리듣기를 만들어 볼까요?"},
}

var guvoiceSentencePrompts = []sentencePromptDefinition{
	{
		ID:          "minimum-all-25",
		Title:       "필수 소리 한 번에 읽기",
		Text:        "아 어 오 우 으 이 애 에 야 여 요 유를 말하고, 가 나 다 라 마 바 사 자 차 카 타 파 하를 또박또박 이어 말해요.",
		Description: "기본 필수 25개 prompt를 한 녹음에서 최대한 많이 뽑기 위한 점검 문장입니다.",
	},
	{
		ID:          "vowels-then-town",
		Title:       "모음과 마을 문장",
		Text:        "아이가 어여쁜 오리와 우유를 보고 으쓱이며 애에게 에너지와 야유회 이야기를 해요. 가게 나무 다리 라디오 마차 바다 사자 자동차 카메라 타자 파도 하늘도 읽어요.",
		Description: "모음 계열을 먼저 읽고 대표 자음 계열을 자연스러운 낱말 속에서 확인합니다.",
	},
	{
		ID:          "short-review-line",
		Title:       "짧은 검수 줄",
		Text:        "아, 어, 오, 우, 으, 이, 애, 에, 야, 여, 요, 유. 가 나 다 라 마 바 사 자 차 카 타 파 하.",
		Description: "짧게 다시 녹음하거나 누락된 후보를 빠르게 검수할 때 쓰는 문장입니다.",
	},
}

var repPromptByChoseong = map[string]string{
	"ㄱ": "rep-ga",
	"ㄴ": "rep-na",
	"ㄷ": "rep-da",
	"ㄹ": "rep-ra",
	"ㅁ": "rep-ma",
	"ㅂ": "rep-ba",
	"ㅅ": "rep-sa",
	"ㅈ": "rep-ja",
	"ㅊ": "rep-cha",
	"ㅋ": "rep-ka",
	"ㅌ": "rep-ta",
	"ㅍ": "rep-pa",
	"ㅎ": "rep-ha",
}

var vowelPromptByJungseong = map[string]string{
	"ㅏ": "vowel-a",
	"ㅓ": "vowel-eo",
	"ㅗ": "vowel-o",
	"ㅜ": "vowel-u",
	"ㅡ": "vowel-eu",
	"ㅣ": "vowel-i",
	"ㅐ": "vowel-ae",
	"ㅔ": "vowel-e",
	"ㅑ": "vowel-ya",
	"ㅕ": "vowel-yeo",
	"ㅛ": "vowel-yo",
	"ㅠ": "vowel-yu",
}

func requiredPromptDefinitions(target int) []promptDefinition {
	target = normalizeTarget(target)
	if target > len(guvoicePromptCatalog) {
		target = len(guvoicePromptCatalog)
	}
	return append([]promptDefinition(nil), guvoicePromptCatalog[:target]...)
}

func promptDefinitionForID(promptID string) promptDefinition {
	for _, prompt := range guvoicePromptCatalog {
		if prompt.ID == promptID {
			return prompt
		}
	}
	return promptDefinition{ID: promptID, Label: promptID, Text: promptID}
}

func sentencePromptDefinitionForID(promptID string) (sentencePromptDefinition, bool) {
	for _, prompt := range guvoiceSentencePrompts {
		if prompt.ID == promptID {
			return prompt, true
		}
	}
	return sentencePromptDefinition{}, false
}

func sentencePromptCoveredPromptIDs(text string) []string {
	_, promptIDs := sequenceForText(text)
	return promptIDs
}

func missingRequiredPromptIDs(target int, samples []model.Sample) []string {
	filled := filledUsablePromptIDs(samples)
	missing := []string{}
	for _, prompt := range requiredPromptDefinitions(target) {
		if !filled[prompt.ID] {
			missing = append(missing, prompt.ID)
		}
	}
	return missing
}

func filledUsablePromptIDs(samples []model.Sample) map[string]bool {
	filled := map[string]bool{}
	for _, sample := range samples {
		if sample.PromptID != "" && sampleUsableForSynthesis(sample) {
			filled[sample.PromptID] = true
		}
	}
	return filled
}

func latestUsableSamplesByPrompt(samples []model.Sample) map[string]model.Sample {
	latest := map[string]model.Sample{}
	for _, sample := range samples {
		if sample.PromptID == "" || !sampleUsableForSynthesis(sample) {
			continue
		}
		existing, ok := latest[sample.PromptID]
		if !ok || sample.CreatedAt.After(existing.CreatedAt) {
			latest[sample.PromptID] = sample
		}
	}
	return latest
}

func sampleUsableForSynthesis(sample model.Sample) bool {
	mimeType := strings.ToLower(strings.TrimSpace(sample.MimeType))
	ext := strings.ToLower(filepath.Ext(sample.FileName))
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(sample.Path))
	}
	return mimeType == "audio/wav" || mimeType == "audio/wave" || mimeType == "audio/x-wav" || ext == ".wav"
}

func sequenceForText(text string) ([]synth.SequenceStep, []string) {
	steps := []synth.SequenceStep{}
	usedPromptIDs := map[string]bool{}
	pendingSpaces := 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			pendingSpaces++
			continue
		}
		if pendingSpaces > 0 {
			steps = appendProsodySilence(steps, spacePauseMillis(pendingSpaces))
			pendingSpaces = 0
		}
		if isStretchMark(r) {
			applyStretchMark(steps)
			continue
		}
		if isEmphasisMark(r) {
			applyEmphasisMark(steps)
			steps = appendProsodySilence(steps, punctuationPauseMillis(r))
			continue
		}
		if isQuestionMark(r) {
			applyQuestionMark(steps)
			steps = appendProsodySilence(steps, punctuationPauseMillis(r))
			continue
		}
		if pauseMillis := punctuationPauseMillis(r); pauseMillis > 0 {
			steps = appendProsodySilence(steps, pauseMillis)
			continue
		}
		parts, ok := hangul.DecomposeRune(r)
		if !ok {
			steps = append(steps, synth.SequenceStep{SilenceMillis: 70})
			continue
		}
		promptID := promptIDForHangul(parts)
		usedPromptIDs[promptID] = true
		steps = append(steps, synth.SequenceStep{PromptID: promptID})
	}
	if pendingSpaces > 0 {
		steps = appendProsodySilence(steps, spacePauseMillis(pendingSpaces))
	}

	promptIDs := make([]string, 0, len(usedPromptIDs))
	for promptID := range usedPromptIDs {
		promptIDs = append(promptIDs, promptID)
	}
	sort.Strings(promptIDs)
	return steps, promptIDs
}

func promptIDForHangul(parts hangul.Parts) string {
	if promptID, ok := repPromptByChoseong[parts.Choseong]; ok {
		return promptID
	}
	if promptID, ok := vowelPromptByJungseong[parts.Jungseong]; ok {
		return promptID
	}
	return "vowel-a"
}

func isSentencePause(r rune) bool {
	switch r {
	case '.', ',', '!', '?', ':', ';', '…', '。', '，', '！', '？', '、':
		return true
	default:
		return false
	}
}

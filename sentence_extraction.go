package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"guvoice/internal/hangul"
	"guvoice/internal/storage"
	"guvoice/internal/synth"
)

type sentenceScriptUnit struct {
	promptID string
	text     string
	weight   float64
	order    int
}

type sentencePromptOccurrence struct {
	promptID string
	text     string
}

type sentenceSampleCandidateWork struct {
	candidate UISentenceSampleCandidate
	score     int
	order     int
}

type sampleInterval struct {
	start int
	end   int
}

type sentenceRecordingGate struct {
	ok       bool
	warnings []string
}

func (a *App) ListSentencePrompts() ([]UISentencePrompt, error) {
	prompts := make([]UISentencePrompt, 0, len(guvoiceSentencePrompts))
	for _, prompt := range guvoiceSentencePrompts {
		prompts = append(prompts, sentencePromptToUI(prompt))
	}
	return prompts, nil
}

func (a *App) ExtractSentenceSamples(req UISentenceExtractionRequest) (UISentenceExtractionResult, error) {
	return extractSentenceSamples(req)
}

func extractSentenceSamples(req UISentenceExtractionRequest) (UISentenceExtractionResult, error) {
	prompt, text, err := resolveSentenceExtractionPrompt(req)
	if err != nil {
		return UISentenceExtractionResult{}, err
	}
	targetSamples := normalizeTarget(req.TargetSamples)
	units, promptCount := sentenceScriptUnitsWithTarget(text, targetSamples)
	if promptCount == 0 {
		return UISentenceExtractionResult{}, errors.New("sentence text must contain Hangul syllables")
	}

	blob := firstNonEmpty(req.DataBase64, req.AudioURL)
	audioName := strings.TrimSpace(req.AudioName)
	if audioName == "" {
		audioName = "sentence-recording.wav"
	}
	data, mimeType, err := decodeSampleBlob(blob, "")
	if err != nil {
		return UISentenceExtractionResult{}, err
	}
	if mimeType != "" && !isWAVMime(mimeType) && strings.ToLower(filepath.Ext(audioName)) != ".wav" {
		return UISentenceExtractionResult{}, fmt.Errorf("sentence extraction requires WAV audio, got %s", mimeType)
	}
	buffer, err := synth.DecodeWAV(data, filepath.Base(audioName))
	if err != nil {
		return UISentenceExtractionResult{}, err
	}
	if len(buffer.Samples) == 0 {
		return UISentenceExtractionResult{}, errors.New("WAV audio has no PCM samples")
	}

	sourceDuration := secondsForSamples(len(buffer.Samples), buffer.SampleRate)
	trimmedPCM, trimOffset := trimSentenceRecording(buffer.Samples, buffer.SampleRate)
	trimmedDuration := secondsForSamples(len(trimmedPCM), buffer.SampleRate)
	threshold := sentenceAmplitudeThreshold(trimmedPCM)
	voiced := detectVoicedIntervals(trimmedPCM, buffer.SampleRate, threshold)
	totalWeight := sentenceScriptWeight(units)
	if totalWeight <= 0 {
		return UISentenceExtractionResult{}, errors.New("sentence prompt sequence is empty")
	}

	warnings := []string{
		"스크립트와 에너지 경계를 이용한 후보 추출입니다. 저장 전에는 각 후보를 들어보고 검수하세요.",
	}
	if !synth.HasAudibleContent(trimmedPCM, buffer.SampleRate) {
		warnings = append(warnings, "녹음 전체가 무음에 가깝습니다.")
	}
	if len(voiced) == 0 {
		warnings = append(warnings, "뚜렷한 발화 구간을 찾지 못해 문장 길이 비례로만 나눴습니다.")
	}

	gate := evaluateSentenceRecordingGate(trimmedPCM, buffer.SampleRate, promptCount, threshold, voiced)
	warnings = append(warnings, gate.warnings...)
	if !gate.ok {
		return UISentenceExtractionResult{
			PromptID:        prompt.ID,
			Prompt:          sentencePromptToUI(prompt),
			Text:            text,
			SourceDuration:  sourceDuration,
			TrimmedDuration: trimmedDuration,
			TotalCandidates: 0,
			Candidates:      []UISentenceSampleCandidate{},
			Warnings:        warnings,
		}, nil
	}

	bestByPrompt := map[string]sentenceSampleCandidateWork{}
	cursor := 0.0
	for _, unit := range units {
		nextCursor := cursor + unit.weight
		if unit.promptID == "" {
			cursor = nextCursor
			continue
		}
		approxStart := int(math.Round(cursor / totalWeight * float64(len(trimmedPCM))))
		approxEnd := int(math.Round(nextCursor / totalWeight * float64(len(trimmedPCM))))
		start, end, foundVoice := refineSentenceCandidateBounds(trimmedPCM, buffer.SampleRate, approxStart, approxEnd, threshold)
		if end <= start {
			cursor = nextCursor
			continue
		}
		pcm := append([]int16(nil), trimmedPCM[start:end]...)
		pcm = synth.TrimSampleSilence(pcm, buffer.SampleRate)
		if len(pcm) == 0 {
			cursor = nextCursor
			continue
		}
		encoded, err := synth.EncodeWAV(buffer.SampleRate, pcm)
		if err != nil {
			cursor = nextCursor
			continue
		}

		promptDefinition := promptDefinitionForID(unit.promptID)
		duration := secondsForSamples(len(pcm), buffer.SampleRate)
		vadOverlap := intervalOverlapRatio(sampleInterval{start: start, end: end}, voiced)
		confidence, status, warning := sentenceCandidateQuality(pcm, buffer.SampleRate, duration, foundVoice, vadOverlap)
		dataBase64 := base64.StdEncoding.EncodeToString(encoded)
		candidate := UISentenceSampleCandidate{
			ID:           sentenceCandidateID(unit.promptID, unit.order),
			PromptID:     unit.promptID,
			Label:        promptDefinition.Label,
			Text:         firstNonEmpty(unit.text, promptDefinition.Text),
			StartSeconds: secondsForSamples(trimOffset+start, buffer.SampleRate),
			EndSeconds:   secondsForSamples(trimOffset+start+len(pcm), buffer.SampleRate),
			Duration:     duration,
			Confidence:   confidence,
			Status:       status,
			Warning:      warning,
			AudioName:    sentenceCandidateAudioName(unit.promptID, unit.order),
			AudioURL:     "data:audio/wav;base64," + dataBase64,
			DataBase64:   dataBase64,
		}
		work := sentenceSampleCandidateWork{
			candidate: candidate,
			score:     confidence,
			order:     unit.order,
		}
		if existing, ok := bestByPrompt[unit.promptID]; !ok || work.score > existing.score {
			bestByPrompt[unit.promptID] = work
		}
		cursor = nextCursor
	}

	works := make([]sentenceSampleCandidateWork, 0, len(bestByPrompt))
	for _, work := range bestByPrompt {
		works = append(works, work)
	}
	sort.SliceStable(works, func(i, j int) bool {
		return works[i].order < works[j].order
	})
	candidates := make([]UISentenceSampleCandidate, 0, len(works))
	for _, work := range works {
		candidates = append(candidates, work.candidate)
	}
	if len(candidates) == 0 {
		warnings = append(warnings, "저장 가능한 WAV 후보를 만들지 못했습니다.")
	}

	return UISentenceExtractionResult{
		PromptID:        prompt.ID,
		Prompt:          sentencePromptToUI(prompt),
		Text:            text,
		SourceDuration:  sourceDuration,
		TrimmedDuration: trimmedDuration,
		TotalCandidates: len(candidates),
		Candidates:      candidates,
		Warnings:        warnings,
	}, nil
}

func resolveSentenceExtractionPrompt(req UISentenceExtractionRequest) (sentencePromptDefinition, string, error) {
	promptID := firstNonEmpty(req.SentencePromptID, req.PromptID)
	text := strings.TrimSpace(req.Text)
	if promptID != "" {
		prompt, ok := sentencePromptDefinitionForID(promptID)
		if !ok {
			return sentencePromptDefinition{}, "", fmt.Errorf("sentence prompt %q not found", promptID)
		}
		if text == "" {
			text = prompt.Text
		}
		return prompt, text, nil
	}
	if text == "" {
		return sentencePromptDefinition{}, "", errors.New("sentencePromptId or text is required")
	}
	return sentencePromptDefinition{
		ID:          "custom",
		Title:       "사용자 문장",
		Text:        text,
		Description: "사용자가 입력한 스크립트입니다.",
	}, text, nil
}

func sentencePromptToUI(prompt sentencePromptDefinition) UISentencePrompt {
	promptIDs := sentencePromptCoveredPromptIDs(prompt.Text)
	return UISentencePrompt{
		ID:               prompt.ID,
		Title:            prompt.Title,
		Text:             prompt.Text,
		Description:      prompt.Description,
		CoveredPromptIDs: promptIDs,
		PromptIDs:        append([]string(nil), promptIDs...),
	}
}

func sentencePromptOccurrences(text string) []sentencePromptOccurrence {
	return sentencePromptOccurrencesWithTarget(text, defaultTargetSamples)
}

func sentencePromptOccurrencesWithTarget(text string, target int) []sentencePromptOccurrence {
	occurrences := []sentencePromptOccurrence{}
	for _, r := range text {
		parts, ok := hangul.DecomposeRune(r)
		if !ok {
			continue
		}
		occurrences = append(occurrences, sentencePromptOccurrence{
			promptID: promptIDForHangulWithTarget(r, parts, target),
			text:     string(r),
		})
	}
	return occurrences
}

func sentenceScriptUnits(text string) ([]sentenceScriptUnit, int) {
	return sentenceScriptUnitsWithTarget(text, defaultTargetSamples)
}

func sentenceScriptUnitsWithTarget(text string, target int) ([]sentenceScriptUnit, int) {
	steps, _ := sequenceForTextWithTarget(text, target)
	occurrences := sentencePromptOccurrencesWithTarget(text, target)
	units := []sentenceScriptUnit{}
	promptIndex := 0
	promptCount := 0
	for _, step := range steps {
		if step.PromptID == "" {
			if step.SilenceMillis > 0 {
				units = append(units, sentenceScriptUnit{weight: math.Max(0.2, float64(step.SilenceMillis)/120)})
			}
			continue
		}
		text := promptDefinitionForID(step.PromptID).Text
		if promptIndex < len(occurrences) && occurrences[promptIndex].promptID == step.PromptID {
			text = occurrences[promptIndex].text
		}
		units = append(units, sentenceScriptUnit{
			promptID: step.PromptID,
			text:     text,
			weight:   1,
			order:    promptCount,
		})
		promptIndex++
		promptCount++
	}
	return units, promptCount
}

func sentenceScriptWeight(units []sentenceScriptUnit) float64 {
	total := 0.0
	for _, unit := range units {
		total += unit.weight
	}
	return total
}

func trimSentenceRecording(pcm []int16, sampleRate int) ([]int16, int) {
	if len(pcm) == 0 {
		return nil, 0
	}
	threshold := sentenceAmplitudeThreshold(pcm)
	pad := max(1, sampleRate/80)
	start, end, ok := voicedBoundsInRange(pcm, 0, len(pcm), threshold, pad)
	if !ok {
		return synth.TrimSampleSilence(pcm, sampleRate), 0
	}
	return append([]int16(nil), pcm[start:end]...), start
}

func refineSentenceCandidateBounds(pcm []int16, sampleRate int, approxStart int, approxEnd int, threshold int) (int, int, bool) {
	if len(pcm) == 0 {
		return 0, 0, false
	}
	approxStart = clampInt(approxStart, 0, len(pcm))
	approxEnd = clampInt(approxEnd, approxStart+1, len(pcm))
	approxLen := max(1, approxEnd-approxStart)
	searchPad := max(sampleRate*80/1000, approxLen/2)
	searchStart := max(0, approxStart-searchPad)
	searchEnd := min(len(pcm), approxEnd+searchPad)
	start, end, ok := voicedBoundsInRange(pcm, searchStart, searchEnd, threshold, sampleRate/125)
	if !ok {
		start, end = approxStart, approxEnd
	}
	minLen := max(1, sampleRate*45/1000)
	if end-start < minLen {
		center := (start + end) / 2
		start = max(0, center-minLen/2)
		end = min(len(pcm), start+minLen)
		start = max(0, end-minLen)
	}
	return start, end, ok
}

func voicedBoundsInRange(pcm []int16, start int, end int, threshold int, pad int) (int, int, bool) {
	start = clampInt(start, 0, len(pcm))
	end = clampInt(end, start, len(pcm))
	first := -1
	last := -1
	for index := start; index < end; index++ {
		if absInt16(pcm[index]) >= threshold {
			if first < 0 {
				first = index
			}
			last = index
		}
	}
	if first < 0 {
		return start, end, false
	}
	first = max(start, first-pad)
	last = min(end-1, last+pad)
	return first, last + 1, true
}

func sentenceAmplitudeThreshold(pcm []int16) int {
	if len(pcm) == 0 {
		return 300
	}
	peak := 0
	var total int64
	step := max(1, len(pcm)/20000)
	count := 0
	for index := 0; index < len(pcm); index += step {
		value := absInt16(pcm[index])
		if value > peak {
			peak = value
		}
		total += int64(value)
		count++
	}
	mean := float64(total) / float64(max(1, count))
	threshold := int(math.Round(math.Max(float64(peak)*0.08, mean*1.8)))
	return clampInt(threshold, 220, 2200)
}

func detectVoicedIntervals(pcm []int16, sampleRate int, threshold int) []sampleInterval {
	if len(pcm) == 0 {
		return nil
	}
	frameSize := max(1, sampleRate/100)
	activeThreshold := max(80, threshold/4)
	intervals := []sampleInterval{}
	activeStart := -1
	for start := 0; start < len(pcm); start += frameSize {
		end := min(len(pcm), start+frameSize)
		if meanAbs(pcm[start:end]) >= activeThreshold {
			if activeStart < 0 {
				activeStart = start
			}
			continue
		}
		if activeStart >= 0 {
			intervals = append(intervals, sampleInterval{start: activeStart, end: end})
			activeStart = -1
		}
	}
	if activeStart >= 0 {
		intervals = append(intervals, sampleInterval{start: activeStart, end: len(pcm)})
	}
	return mergeVoicedIntervals(intervals, sampleRate*60/1000, sampleRate*25/1000)
}

func evaluateSentenceRecordingGate(pcm []int16, sampleRate int, promptCount int, threshold int, voiced []sampleInterval) sentenceRecordingGate {
	warnings := []string{}
	ok := true
	trimmedDuration := secondsForSamples(len(pcm), sampleRate)
	voicedDuration := secondsForSamples(totalIntervalSamples(voiced), sampleRate)
	minTrimmedDuration := math.Max(0.14, float64(promptCount)*0.045)
	minVoicedDuration := math.Max(0.09, float64(promptCount)*0.04)
	minBursts := minSentenceVoicedBursts(promptCount)

	if len(pcm) == 0 || !synth.HasAudibleContent(pcm, sampleRate) {
		warnings = append(warnings, "녹음이 무음이거나 거의 무음이라 후보를 만들지 않았습니다.")
		ok = false
	}
	if len(voiced) == 0 {
		warnings = append(warnings, "실제 발화로 볼 수 있는 구간이 없어 후보를 만들지 않았습니다.")
		ok = false
	}
	if trimmedDuration < minTrimmedDuration {
		warnings = append(warnings, fmt.Sprintf("녹음이 너무 짧습니다. 이 문장은 최소 %.2f초 정도의 발화가 필요합니다.", minTrimmedDuration))
		ok = false
	}
	if voicedDuration < minVoicedDuration {
		warnings = append(warnings, fmt.Sprintf("발화 시간이 prompt 수에 비해 부족합니다. 감지된 발화는 %.2f초입니다.", voicedDuration))
		ok = false
	}
	if minBursts > 1 && countVoicedBursts(pcm, sampleRate, threshold) < minBursts {
		warnings = append(warnings, "한두 소리만 감지되어 문장 전체를 읽은 녹음으로 보기 어렵습니다.")
		ok = false
	}
	return sentenceRecordingGate{ok: ok, warnings: warnings}
}

func minSentenceVoicedBursts(promptCount int) int {
	switch {
	case promptCount <= 1:
		return 1
	case promptCount < 8:
		return 2
	default:
		return clampInt(promptCount/8, 2, 4)
	}
}

func countVoicedBursts(pcm []int16, sampleRate int, threshold int) int {
	if len(pcm) == 0 || sampleRate <= 0 {
		return 0
	}
	frameSize := max(1, sampleRate/200)
	activeThreshold := max(80, threshold/4)
	bursts := 0
	inBurst := false
	inactiveFrames := 0
	maxInactiveFrames := 1
	for start := 0; start < len(pcm); start += frameSize {
		end := min(len(pcm), start+frameSize)
		active := meanAbs(pcm[start:end]) >= activeThreshold
		if active {
			if !inBurst {
				bursts++
				inBurst = true
			}
			inactiveFrames = 0
			continue
		}
		if inBurst {
			inactiveFrames++
			if inactiveFrames > maxInactiveFrames {
				inBurst = false
				inactiveFrames = 0
			}
		}
	}
	return bursts
}

func totalIntervalSamples(intervals []sampleInterval) int {
	total := 0
	for _, interval := range intervals {
		if interval.end > interval.start {
			total += interval.end - interval.start
		}
	}
	return total
}

func mergeVoicedIntervals(intervals []sampleInterval, maxGap int, minLength int) []sampleInterval {
	if len(intervals) == 0 {
		return nil
	}
	merged := []sampleInterval{intervals[0]}
	for _, interval := range intervals[1:] {
		last := &merged[len(merged)-1]
		if interval.start-last.end <= maxGap {
			last.end = interval.end
			continue
		}
		merged = append(merged, interval)
	}
	filtered := merged[:0]
	for _, interval := range merged {
		if interval.end-interval.start >= minLength {
			filtered = append(filtered, interval)
		}
	}
	return filtered
}

func sentenceCandidateQuality(pcm []int16, sampleRate int, duration float64, foundVoice bool, vadOverlap float64) (int, string, string) {
	confidence := 35
	audible := synth.HasAudibleContent(pcm, sampleRate)
	if audible {
		confidence += 25
	}
	if foundVoice {
		confidence += 15
	}
	if vadOverlap >= 0.35 {
		confidence += 10
	}
	switch {
	case duration >= 0.07 && duration <= 0.55:
		confidence += 15
	case duration >= 0.045 && duration <= 0.8:
		confidence += 8
	case duration < 0.045:
		confidence -= 15
	default:
		confidence -= 10
	}
	if peakAbsInt16(pcm) >= 3000 {
		confidence += 5
	}
	confidence = clampInt(confidence, 0, 100)

	warning := ""
	switch {
	case !audible:
		warning = "무음에 가까운 후보입니다."
	case duration < 0.045:
		warning = "구간이 너무 짧아 다시 확인이 필요합니다."
	case duration > 0.8:
		warning = "구간이 길어 주변 발음이 섞였을 수 있습니다."
	case vadOverlap < 0.2:
		warning = "에너지 경계가 약해 수동 확인이 필요합니다."
	}
	status := "ready"
	if confidence < 75 || warning != "" {
		status = "review"
	}
	if confidence < 45 {
		status = "warning"
	}
	return confidence, status, warning
}

func sentenceCandidateAudioName(promptID string, order int) string {
	return storage.SafeFileBase(fmt.Sprintf("%02d-%s", order+1, promptID)) + ".wav"
}

func sentenceCandidateID(promptID string, order int) string {
	return storage.SafeFileBase(fmt.Sprintf("sentence-%02d-%s", order+1, promptID))
}

func intervalOverlapRatio(target sampleInterval, intervals []sampleInterval) float64 {
	length := target.end - target.start
	if length <= 0 || len(intervals) == 0 {
		return 0
	}
	overlap := 0
	for _, interval := range intervals {
		start := max(target.start, interval.start)
		end := min(target.end, interval.end)
		if end > start {
			overlap += end - start
		}
	}
	return float64(overlap) / float64(length)
}

func meanAbs(pcm []int16) int {
	if len(pcm) == 0 {
		return 0
	}
	var total int64
	for _, sample := range pcm {
		total += int64(absInt16(sample))
	}
	return int(total / int64(len(pcm)))
}

func peakAbsInt16(pcm []int16) int {
	peak := 0
	for _, sample := range pcm {
		if value := absInt16(sample); value > peak {
			peak = value
		}
	}
	return peak
}

func absInt16(value int16) int {
	if value < 0 {
		return -int(value)
	}
	return int(value)
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func secondsForSamples(samples int, sampleRate int) float64 {
	if sampleRate <= 0 {
		return 0
	}
	return math.Round(float64(samples)/float64(sampleRate)*1000) / 1000
}

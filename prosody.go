package main

import "guvoice/internal/synth"

func appendProsodySilence(steps []synth.SequenceStep, millis int) []synth.SequenceStep {
	if millis <= 0 {
		return steps
	}
	const maxMergedSilenceMillis = 520
	if len(steps) > 0 && steps[len(steps)-1].PromptID == "" && steps[len(steps)-1].SilenceMillis > 0 {
		steps[len(steps)-1].SilenceMillis = min(maxMergedSilenceMillis, steps[len(steps)-1].SilenceMillis+millis)
		return steps
	}
	return append(steps, synth.SequenceStep{SilenceMillis: min(maxMergedSilenceMillis, millis)})
}

func applyEmphasisMark(steps []synth.SequenceStep) {
	index := previousPromptStepIndex(steps)
	if index < 0 {
		return
	}
	step := &steps[index]
	step.Gain = clampFloat(defaultFloat(step.Gain, 1)+0.18, 1, 1.45)
	step.Speed = clampFloat(defaultFloat(step.Speed, 1)+0.08, 0.5, 1.25)
	step.GapMillis = min(110, step.GapMillis+35)
}

func applyQuestionMark(steps []synth.SequenceStep) {
	index := previousPromptStepIndex(steps)
	if index < 0 {
		return
	}
	step := &steps[index]
	step.Gain = clampFloat(defaultFloat(step.Gain, 1)+0.06, 1, 1.25)
	step.Speed = clampFloat(defaultFloat(step.Speed, 1)-0.07, 0.78, 1.15)
	step.Stretch = clampFloat(defaultFloat(step.Stretch, 1)+0.12, 1, 1.45)
	step.GapMillis = min(150, step.GapMillis+55)
}

func applyStretchMark(steps []synth.SequenceStep) {
	index := previousPromptStepIndex(steps)
	if index < 0 {
		return
	}
	step := &steps[index]
	step.Stretch = clampFloat(defaultFloat(step.Stretch, 1)+0.28, 1, 1.95)
	step.GapMillis = min(90, step.GapMillis+12)
}

func previousPromptStepIndex(steps []synth.SequenceStep) int {
	for index := len(steps) - 1; index >= 0; index-- {
		if steps[index].PromptID != "" {
			return index
		}
	}
	return -1
}

func defaultFloat(value float64, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func clampFloat(value float64, minValue float64, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func spacePauseMillis(count int) int {
	if count <= 0 {
		return 0
	}
	return min(190, 80+(count-1)*35)
}

func punctuationPauseMillis(r rune) int {
	switch r {
	case ',', '\u3001', '\uFF0C':
		return 115
	case '.', '\u3002', '\uFF0E':
		return 210
	case ':', ';', '\uFF1A', '\uFF1B':
		return 155
	case '!', '\uFF01':
		return 90
	case '?', '\uFF1F':
		return 145
	default:
		return 0
	}
}

func isEmphasisMark(r rune) bool {
	return r == '!' || r == '\uFF01'
}

func isQuestionMark(r rune) bool {
	return r == '?' || r == '\uFF1F'
}

func isStretchMark(r rune) bool {
	return r == '~' || r == '\u301C' || r == '\uFF5E'
}

func isProsodyRune(r rune) bool {
	return isStretchMark(r) || punctuationPauseMillis(r) > 0
}

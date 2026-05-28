package main

import (
	"path/filepath"
	"testing"

	"guvoice/internal/synth"
)

func TestSequenceForTextAddsSilenceForSpace(t *testing.T) {
	steps, promptIDs := sequenceForText("\uAC00 \uB098")
	if len(steps) != 3 {
		t.Fatalf("expected prompt, silence, prompt steps, got %#v", steps)
	}
	if steps[0].PromptID == "" || steps[2].PromptID == "" {
		t.Fatalf("expected Hangul prompt steps around the space, got %#v", steps)
	}
	if steps[1].SilenceMillis <= 0 {
		t.Fatalf("expected a silence step for the space, got %#v", steps[1])
	}
	if len(promptIDs) == 0 {
		t.Fatal("expected Hangul prompt IDs, got none")
	}
}

func TestSequenceForTextAppliesPunctuationProsody(t *testing.T) {
	plain, _ := sequenceForText("\uAC00")
	emphatic, _ := sequenceForText("\uAC00!")
	question, _ := sequenceForText("\uAC00?")
	stretched, _ := sequenceForText("\uAC00~")
	if len(plain) != 1 || len(emphatic) < 1 || len(question) < 1 || len(stretched) != 1 {
		t.Fatalf("unexpected sequence shapes: plain=%#v !=%#v ?=%#v ~=%#v", plain, emphatic, question, stretched)
	}
	if emphatic[0].Gain <= plain[0].Gain || emphatic[0].Speed <= plain[0].Speed {
		t.Fatalf("expected ! to emphasize the previous prompt, plain=%#v emphatic=%#v", plain[0], emphatic[0])
	}
	if question[0].Stretch <= plain[0].Stretch || question[0].GapMillis <= plain[0].GapMillis {
		t.Fatalf("expected ? to alter the previous prompt ending, plain=%#v question=%#v", plain[0], question[0])
	}
	if stretched[0].Stretch <= plain[0].Stretch {
		t.Fatalf("expected ~ to stretch the previous prompt, plain=%#v stretched=%#v", plain[0], stretched[0])
	}
}

func TestProsodyAffectsSynthesizedDuration(t *testing.T) {
	dir := t.TempDir()
	samplePath := filepath.Join(dir, "ga.wav")
	writeTestToneWAV(t, samplePath)

	plain, _ := sequenceForText("\uAC00")
	emphatic, _ := sequenceForText("\uAC00!")
	question, _ := sequenceForText("\uAC00?")
	stretched, _ := sequenceForText("\uAC00~")
	withPause, _ := sequenceForText("\uAC00.")
	attachPath(plain, samplePath)
	attachPath(emphatic, samplePath)
	attachPath(question, samplePath)
	attachPath(stretched, samplePath)
	attachPath(withPause, samplePath)

	plainResult := renderTestSequence(t, dir, "plain.wav", plain)
	emphaticResult := renderTestSequence(t, dir, "emphatic.wav", emphatic)
	questionResult := renderTestSequence(t, dir, "question.wav", question)
	stretchedResult := renderTestSequence(t, dir, "stretched.wav", stretched)
	pauseResult := renderTestSequence(t, dir, "pause.wav", withPause)

	if maxPeak(emphaticResult.Samples) <= maxPeak(plainResult.Samples) {
		t.Fatalf("expected ! rendering to increase PCM peak: plain=%d emphatic=%d", maxPeak(plainResult.Samples), maxPeak(emphaticResult.Samples))
	}
	if equalPCMPrefix(plainResult.Samples, emphaticResult.Samples, 200) {
		t.Fatal("expected ! rendering to alter PCM sample data, but the rendered prefix matched plain audio")
	}
	if questionResult.DurationMillis <= plainResult.DurationMillis {
		t.Fatalf("expected ? rendering to lengthen duration through stretch/gap/pause: plain=%d question=%d", plainResult.DurationMillis, questionResult.DurationMillis)
	}
	if stretchedResult.DurationMillis <= plainResult.DurationMillis {
		t.Fatalf("expected ~ duration to exceed plain: plain=%d stretched=%d", plainResult.DurationMillis, stretchedResult.DurationMillis)
	}
	if pauseResult.DurationMillis <= plainResult.DurationMillis {
		t.Fatalf("expected punctuation pause duration to exceed plain: plain=%d pause=%d", plainResult.DurationMillis, pauseResult.DurationMillis)
	}
}

type renderedSequence struct {
	DurationMillis int
	Samples        []int16
}

func renderTestSequence(t *testing.T, dir string, name string, steps []synth.SequenceStep) renderedSequence {
	t.Helper()
	path := filepath.Join(dir, name)
	duration, err := synth.WriteSequenceWAV(path, steps, synth.Options{SampleRate: 8000, Speed: 1})
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := synth.ReadWAV(path)
	if err != nil {
		t.Fatal(err)
	}
	return renderedSequence{DurationMillis: duration, Samples: buffer.Samples}
}

func attachPath(steps []synth.SequenceStep, path string) {
	for index := range steps {
		if steps[index].PromptID != "" {
			steps[index].Path = path
		}
	}
}

func writeTestToneWAV(t *testing.T, path string) {
	t.Helper()
	const sampleRate = 8000
	pcm := make([]int16, sampleRate/8)
	for i := range pcm {
		if i%32 < 16 {
			pcm[i] = 9000
		} else {
			pcm[i] = -9000
		}
	}
	if err := synth.WriteWAV(path, sampleRate, pcm); err != nil {
		t.Fatal(err)
	}
}

func maxPeak(samples []int16) int {
	peak := 0
	for _, sample := range samples {
		value := int(sample)
		if value < 0 {
			value = -value
		}
		if value > peak {
			peak = value
		}
	}
	return peak
}

func equalPCMPrefix(left []int16, right []int16, limit int) bool {
	if len(left) < limit || len(right) < limit {
		return false
	}
	for index := 0; index < limit; index++ {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

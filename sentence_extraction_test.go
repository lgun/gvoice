package main

import (
	"encoding/base64"
	"math"
	"reflect"
	"strings"
	"testing"

	"guvoice/internal/hangul"
	"guvoice/internal/synth"
)

func TestExtractSentenceSamplesReturnsDecodableWAVCandidates(t *testing.T) {
	app := &App{}
	prompts, err := app.ListSentencePrompts()
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) == 0 {
		t.Fatal("expected sentence prompts")
	}
	dataURL := syntheticSentenceWAVDataURL(t, prompts[0].Text)
	result, err := app.ExtractSentenceSamples(UISentenceExtractionRequest{
		SentencePromptID: prompts[0].ID,
		AudioName:        "sentence.wav",
		DataBase64:       dataURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != prompts[0].Text || result.SourceDuration <= 0 || result.TrimmedDuration <= 0 {
		t.Fatalf("unexpected extraction metadata: %#v", result)
	}
	if result.PromptID != prompts[0].ID {
		t.Fatalf("expected promptId %q, got %q", prompts[0].ID, result.PromptID)
	}
	if result.TotalCandidates != len(result.Candidates) {
		t.Fatalf("totalCandidates should match candidates length: %d != %d", result.TotalCandidates, len(result.Candidates))
	}
	if len(result.Prompt.PromptIDs) == 0 || !reflect.DeepEqual(result.Prompt.PromptIDs, result.Prompt.CoveredPromptIDs) {
		t.Fatalf("promptIds alias should mirror coveredPromptIds: %#v", result.Prompt)
	}
	if len(result.Candidates) < len(requiredPromptDefinitions(25)) {
		t.Fatalf("expected candidates for the required prompt set, got %d", len(result.Candidates))
	}

	candidatesByPrompt := map[string]UISentenceSampleCandidate{}
	candidateIDs := map[string]bool{}
	for _, candidate := range result.Candidates {
		if candidate.ID == "" || candidate.PromptID == "" || candidate.AudioName == "" {
			t.Fatalf("candidate is missing prompt/audio metadata: %#v", candidate)
		}
		if candidateIDs[candidate.ID] {
			t.Fatalf("candidate id is not stable/unique within the result: %q", candidate.ID)
		}
		candidateIDs[candidate.ID] = true
		if !strings.HasPrefix(candidate.AudioURL, "data:audio/wav;base64,") {
			t.Fatalf("candidate audio URL should be a WAV data URL, got %q", candidate.AudioURL)
		}
		if candidate.DataBase64 == "" || strings.HasPrefix(candidate.DataBase64, "data:") {
			t.Fatalf("candidate dataBase64 should be plain base64, got %q", candidate.DataBase64)
		}
		decoded, err := base64.StdEncoding.DecodeString(candidate.DataBase64)
		if err != nil {
			t.Fatalf("candidate %s has invalid base64: %v", candidate.PromptID, err)
		}
		buffer, err := synth.DecodeWAV(decoded, candidate.AudioName)
		if err != nil {
			t.Fatalf("candidate %s should decode as WAV: %v", candidate.PromptID, err)
		}
		if len(buffer.Samples) == 0 || buffer.SampleRate != 8000 {
			t.Fatalf("candidate %s decoded with unexpected audio shape: %#v", candidate.PromptID, buffer)
		}
		candidatesByPrompt[candidate.PromptID] = candidate
	}
	for _, prompt := range requiredPromptDefinitions(25) {
		if _, ok := candidatesByPrompt[prompt.ID]; !ok {
			t.Fatalf("expected candidate for required prompt %s", prompt.ID)
		}
	}
}

func TestExtractSentenceSamplesReturnsNoCandidatesForSilentWAV(t *testing.T) {
	app := &App{}
	prompts, err := app.ListSentencePrompts()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		pcm  []int16
	}{
		{name: "silent", pcm: make([]int16, 8000)},
		{name: "nearly-silent", pcm: syntheticToneWithAmplitude(8000, 1000, 220, 120)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := app.ExtractSentenceSamples(UISentenceExtractionRequest{
				SentencePromptID: prompts[0].ID,
				AudioName:        tc.name + ".wav",
				DataBase64:       wavDataURLFromPCM(t, 8000, tc.pcm),
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.PromptID != prompts[0].ID {
				t.Fatalf("expected promptId %q, got %q", prompts[0].ID, result.PromptID)
			}
			if result.TotalCandidates != 0 || len(result.Candidates) != 0 {
				t.Fatalf("%s WAV should not return candidates: %#v", tc.name, result)
			}
			if len(result.Warnings) == 0 {
				t.Fatalf("%s WAV should return a warning", tc.name)
			}
		})
	}
}

func TestExtractSentenceSamplesReturnsNoCandidatesForTooShortVoicedWAV(t *testing.T) {
	app := &App{}
	prompts, err := app.ListSentencePrompts()
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]int16, 0, 2000)
	pcm = append(pcm, make([]int16, 400)...)
	pcm = append(pcm, syntheticTone(8000, 80, 440)...)
	pcm = append(pcm, make([]int16, 400)...)
	result, err := app.ExtractSentenceSamples(UISentenceExtractionRequest{
		SentencePromptID: prompts[0].ID,
		AudioName:        "too-short.wav",
		DataBase64:       wavDataURLFromPCM(t, 8000, pcm),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCandidates != 0 || len(result.Candidates) != 0 {
		t.Fatalf("too-short voiced WAV should not return candidates: %#v", result)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("too-short voiced WAV should return a warning")
	}
}

func TestExtractSentenceSamplesUsesTargetAwareExactPrompt(t *testing.T) {
	app := &App{}
	exactText := "\uAC1C"
	exactID := hangul.SyllablePromptID('\uAC1C')
	pcm := make([]int16, 0, 3200)
	pcm = append(pcm, make([]int16, 400)...)
	pcm = append(pcm, syntheticTone(8000, 220, 440)...)
	pcm = append(pcm, make([]int16, 400)...)

	result, err := app.ExtractSentenceSamples(UISentenceExtractionRequest{
		Text:          exactText,
		TargetSamples: 100,
		AudioName:     "exact-syllable.wav",
		DataBase64:    wavDataURLFromPCM(t, 8000, pcm),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCandidates != 1 || len(result.Candidates) != 1 {
		t.Fatalf("expected one exact prompt candidate, got %#v", result)
	}
	candidate := result.Candidates[0]
	if candidate.PromptID != exactID || candidate.Text != exactText {
		t.Fatalf("expected exact prompt %s for %q, got %#v", exactID, exactText, candidate)
	}
}

func TestExtractSentenceSamplesRejectsEmptyAndInvalidBase64(t *testing.T) {
	app := &App{}
	if _, err := app.ExtractSentenceSamples(UISentenceExtractionRequest{
		Text:       "아",
		DataBase64: "",
	}); err == nil {
		t.Fatal("expected empty dataBase64 to fail")
	}
	if _, err := app.ExtractSentenceSamples(UISentenceExtractionRequest{
		Text:       "아",
		DataBase64: "not-base64",
	}); err == nil {
		t.Fatal("expected invalid base64 to fail")
	}
}

func TestSentencePromptPackCoversRequiredPrompts(t *testing.T) {
	app := &App{}
	prompts, err := app.ListSentencePrompts()
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) < 2 {
		t.Fatalf("expected a small curated sentence pack, got %d", len(prompts))
	}
	covered := map[string]bool{}
	for _, prompt := range prompts {
		if prompt.ID == "" || prompt.Title == "" || prompt.Text == "" || prompt.Description == "" {
			t.Fatalf("sentence prompt is missing display fields: %#v", prompt)
		}
		if len(prompt.CoveredPromptIDs) == 0 {
			t.Fatalf("sentence prompt should report prompt coverage: %#v", prompt)
		}
		if !reflect.DeepEqual(prompt.PromptIDs, prompt.CoveredPromptIDs) {
			t.Fatalf("promptIds should mirror coveredPromptIds: %#v", prompt)
		}
		for _, promptID := range prompt.CoveredPromptIDs {
			covered[promptID] = true
		}
	}
	for _, required := range requiredPromptDefinitions(25) {
		if !covered[required.ID] {
			t.Fatalf("sentence pack does not cover required prompt %s", required.ID)
		}
	}
}

func syntheticSentenceWAVDataURL(t *testing.T, text string) string {
	t.Helper()
	const sampleRate = 8000
	units, _ := sentenceScriptUnits(text)
	pcm := make([]int16, 0, sampleRate*4)
	pcm = append(pcm, make([]int16, sampleRate/5)...)
	promptIndex := 0
	for _, unit := range units {
		if unit.promptID == "" {
			silenceSamples := int(float64(sampleRate) * 0.12 * unit.weight)
			pcm = append(pcm, make([]int16, silenceSamples)...)
			continue
		}
		frequency := 220 + (promptIndex%14)*31
		pcm = append(pcm, syntheticTone(sampleRate, 90, frequency)...)
		pcm = append(pcm, make([]int16, sampleRate/80)...)
		promptIndex++
	}
	pcm = append(pcm, make([]int16, sampleRate/5)...)
	return wavDataURLFromPCM(t, sampleRate, pcm)
}

func wavDataURLFromPCM(t *testing.T, sampleRate int, pcm []int16) string {
	t.Helper()
	data, err := synth.EncodeWAV(sampleRate, pcm)
	if err != nil {
		t.Fatal(err)
	}
	return "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(data)
}

func syntheticTone(sampleRate int, millis int, frequency int) []int16 {
	return syntheticToneWithAmplitude(sampleRate, millis, frequency, 11000)
}

func syntheticToneWithAmplitude(sampleRate int, millis int, frequency int, amplitude int) []int16 {
	samples := max(1, sampleRate*millis/1000)
	pcm := make([]int16, samples)
	for i := range pcm {
		value := math.Sin(float64(i) / float64(sampleRate) * math.Pi * 2 * float64(frequency))
		fade := 1.0
		fadeSamples := max(1, sampleRate/200)
		if i < fadeSamples {
			fade = float64(i) / float64(fadeSamples)
		}
		if tail := len(pcm) - 1 - i; tail < fadeSamples {
			fade = math.Min(fade, float64(tail)/float64(fadeSamples))
		}
		pcm[i] = int16(value * fade * float64(amplitude))
	}
	return pcm
}

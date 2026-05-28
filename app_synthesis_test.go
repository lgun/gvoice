package main

import (
	"encoding/base64"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guvoice/internal/model"
	"guvoice/internal/storage"
	"guvoice/internal/synth"
)

func TestSynthesizeToFileUsesRecordedWAVSamples(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "test voice",
		TargetSamples: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, prompt := range requiredPromptDefinitions(source.TargetSamples) {
		_, err := store.SaveSample(model.SaveSampleRequest{
			SourceID:       source.ID,
			PromptID:       prompt.ID,
			FileName:       prompt.ID + ".wav",
			DataBase64:     testWAVDataURL(t, 220+index*11),
			Transcript:     prompt.Text,
			DurationMillis: 120,
		})
		if err != nil {
			t.Fatalf("save sample %s: %v", prompt.ID, err)
		}
	}

	app := &App{store: store}
	result, err := app.synthesizeToFile(model.SynthesisRequest{
		SourceID:   source.ID,
		Text:       "가 나 아?",
		Format:     "wav",
		OutputName: "sample-based-test",
		SampleRate: 8000,
		Speed:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "샘플 기반 WAV") {
		t.Fatalf("unexpected synthesis message: %q", result.Message)
	}
	outputPath := filepath.Join(store.BaseDir(), filepath.FromSlash(result.AudioPath))
	buffer, err := synth.ReadWAV(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if buffer.SampleRate != 8000 {
		t.Fatalf("expected 8000 Hz, got %d", buffer.SampleRate)
	}
	if len(buffer.Samples) == 0 || result.DurationMillis <= 0 {
		t.Fatalf("empty synthesized audio: samples=%d duration=%d", len(buffer.Samples), result.DurationMillis)
	}
	if !result.MissingReport.ReadyForMVP {
		t.Fatalf("successful synthesis should be ready, got report %#v", result.MissingReport)
	}
	if len(result.MissingReport.MissingPromptIDs) != 0 {
		t.Fatalf("successful synthesis should not include missing prompts: %#v", result.MissingReport.MissingPromptIDs)
	}
	if len(result.MissingReport.MissingJamo) != 0 || len(result.MissingReport.MissingExactSyllables) != 0 {
		t.Fatalf("successful synthesis report should use prompt sequence semantics: %#v", result.MissingReport)
	}
}

func TestSynthesizeToFileBlocksMissingRequiredSamples(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "empty voice",
		TargetSamples: 25,
	})
	if err != nil {
		t.Fatal(err)
	}

	app := &App{store: store}
	_, err = app.synthesizeToFile(model.SynthesisRequest{
		SourceID: source.ID,
		Text:     "가",
		Format:   "wav",
	})
	if err == nil {
		t.Fatal("expected missing required sample error")
	}
	if !strings.Contains(err.Error(), "필수 WAV 샘플이 부족합니다") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAnalyzeTextReportsInputPromptMissing(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "partial voice",
		TargetSamples: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveSample(model.SaveSampleRequest{
		SourceID:       source.ID,
		PromptID:       "vowel-a",
		FileName:       "vowel-a.wav",
		DataBase64:     testWAVDataURL(t, 220),
		Transcript:     "아",
		DurationMillis: 120,
	})
	if err != nil {
		t.Fatal(err)
	}

	app := &App{store: store}
	report, err := app.AnalyzeText(source.ID, "바")
	if err != nil {
		t.Fatal(err)
	}
	if !containsMissingPrompt(report, "rep-ba") {
		t.Fatalf("expected rep-ba missing in analysis, got %#v", report.Missing)
	}
}

func TestAddSampleRejectsNonWAVUpload(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "upload voice",
		TargetSamples: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	app := &App{store: store}
	_, err = app.AddSample(source.ID, UIVoiceSample{
		PromptID:   "vowel-a",
		Label:      "bad upload",
		Text:       "not wav",
		AudioName:  "bad.webm",
		DataBase64: "data:audio/webm;base64," + base64.StdEncoding.EncodeToString([]byte("fake webm")),
	})
	if err == nil {
		t.Fatal("expected non-WAV upload to be rejected")
	}
	if !strings.Contains(err.Error(), "WAV") {
		t.Fatalf("expected WAV guidance, got %v", err)
	}
}

func TestTargetSamplesClampToPromptCatalog(t *testing.T) {
	if normalizeTarget(len(guvoicePromptCatalog)+20) != len(guvoicePromptCatalog) {
		t.Fatalf("expected target clamp to catalog length %d", len(guvoicePromptCatalog))
	}

	source := model.VoiceSource{
		ID:            "voice-over-target",
		Name:          "over target",
		TargetSamples: 80,
	}
	ui := sourceToUI(source, nil)
	if ui.TargetSamples != len(guvoicePromptCatalog) {
		t.Fatalf("expected UI target %d, got %d", len(guvoicePromptCatalog), ui.TargetSamples)
	}
	if got := len(requiredPromptDefinitions(80)); got != len(guvoicePromptCatalog) {
		t.Fatalf("expected required prompts to clamp to catalog length, got %d", got)
	}
}

func testWAVDataURL(t *testing.T, frequency int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.wav")
	const sampleRate = 8000
	pcm := make([]int16, sampleRate/8)
	for i := range pcm {
		value := math.Sin(float64(i) / sampleRate * math.Pi * 2 * float64(frequency))
		pcm[i] = int16(value * 12000)
	}
	if err := synth.WriteWAV(path, sampleRate, pcm); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(data)
}

func containsMissingPrompt(report UIAnalysisResult, promptID string) bool {
	for _, missing := range report.Missing {
		if missing.PromptID == promptID {
			return true
		}
	}
	return false
}

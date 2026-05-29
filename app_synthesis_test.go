package main

import (
	"encoding/base64"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guvoice/internal/hangul"
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

func TestExportMP3WritesMP3File(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "mp3 voice",
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
			DataBase64:     testWAVDataURL(t, 260+index*7),
			Transcript:     prompt.Text,
			DurationMillis: 120,
		})
		if err != nil {
			t.Fatalf("save sample %s: %v", prompt.ID, err)
		}
	}
	exportDir := filepath.Join(t.TempDir(), "chosen-mp3-dir")
	app := &App{store: store}
	settings, err := app.SetOutputDirectory(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Path != exportDir || settings.IsDefault {
		t.Fatalf("expected custom output directory settings, got %#v", settings)
	}

	preview, err := app.Synthesize(UISynthesisRequest{
		SourceID: source.ID,
		Text:     "hello",
		Options: UISynthesisOptions{
			Speed:          1,
			Pitch:          2,
			Clarity:        70,
			NoiseReduction: 40,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Status != "ready" || !strings.HasPrefix(preview.AudioURL, "data:audio/wav;base64,") {
		t.Fatalf("expected ready WAV preview, got %#v", preview)
	}
	exportEntries, err := os.ReadDir(store.ExportsDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(exportEntries) != 0 {
		t.Fatalf("preview should not write to default exports dir, got %d entries", len(exportEntries))
	}
	customExportEntries, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(customExportEntries) != 0 {
		t.Fatalf("preview should not write to configured MP3 export dir, got %d entries", len(customExportEntries))
	}
	tempEntries, err := os.ReadDir(store.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(tempEntries) != 1 || filepath.Ext(tempEntries[0].Name()) != ".wav" {
		t.Fatalf("expected preview WAV in temp dir, got %#v", tempEntries)
	}

	result, err := app.ExportMP3(UISynthesisRequest{
		SourceID: source.ID,
		Text:     "媛",
		Options: UISynthesisOptions{
			Speed:          1,
			Pitch:          2,
			Clarity:        70,
			NoiseReduction: 40,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "saved" {
		t.Fatalf("expected saved export, got %#v", result)
	}
	if filepath.Ext(result.Path) != ".mp3" {
		t.Fatalf("expected .mp3 export path, got %s", result.Path)
	}
	if filepath.Dir(result.Path) != exportDir {
		t.Fatalf("expected MP3 in configured directory %s, got %s", exportDir, result.Path)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !hasMP3FrameHeader(data) {
		t.Fatalf("expected MP3 frame header, got first bytes % x", data[:min(len(data), 8)])
	}
}

func TestOutputDirectorySettingsGetSetAndReset(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}

	defaultSettings, err := app.GetOutputDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if !defaultSettings.IsDefault {
		t.Fatalf("expected default output directory, got %#v", defaultSettings)
	}
	if defaultSettings.Path != "" || defaultSettings.DefaultPath != store.ExportsDir() {
		t.Fatalf("expected blank custom path and default path %s, got %#v", store.ExportsDir(), defaultSettings)
	}
	if defaultSettings.Source != "wails" || strings.TrimSpace(defaultSettings.Message) == "" {
		t.Fatalf("expected Wails source and message, got %#v", defaultSettings)
	}

	customDir := filepath.Join(t.TempDir(), "custom-output")
	customSettings, err := app.SetOutputDirectory(customDir)
	if err != nil {
		t.Fatal(err)
	}
	if customSettings.IsDefault || customSettings.Path != customDir {
		t.Fatalf("expected custom output directory %s, got %#v", customDir, customSettings)
	}

	resetSettings, err := app.SetOutputDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	if !resetSettings.IsDefault || resetSettings.Path != "" || resetSettings.DefaultPath != store.ExportsDir() {
		t.Fatalf("expected reset to default path %s, got %#v", store.ExportsDir(), resetSettings)
	}

	defaultPathSettings, err := app.SetOutputDirectory(store.ExportsDir())
	if err != nil {
		t.Fatal(err)
	}
	if !defaultPathSettings.IsDefault || defaultPathSettings.Path != "" {
		t.Fatalf("expected explicit default path to stay default, got %#v", defaultPathSettings)
	}
}

func TestSpeechLibrarySettingsGetSetAndReset(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}

	defaultSettings, err := app.GetSpeechLibrarySettings()
	if err != nil {
		t.Fatal(err)
	}
	if !defaultSettings.IsDefault {
		t.Fatalf("expected default speech library directory, got %#v", defaultSettings)
	}
	if defaultSettings.Path != "" || defaultSettings.DefaultPath != store.DefaultSpeechLibraryDir() {
		t.Fatalf("expected blank custom path and default path %s, got %#v", store.DefaultSpeechLibraryDir(), defaultSettings)
	}
	if defaultSettings.ResolvedPath != store.DefaultSpeechLibraryDir() {
		t.Fatalf("expected resolved default path %s, got %#v", store.DefaultSpeechLibraryDir(), defaultSettings)
	}

	customDir := filepath.Join(t.TempDir(), "custom-speech-library")
	customSettings, err := app.SetSpeechLibraryDirectory(customDir)
	if err != nil {
		t.Fatal(err)
	}
	if customSettings.IsDefault || customSettings.Path != customDir || customSettings.ResolvedPath != customDir {
		t.Fatalf("expected custom speech library directory %s, got %#v", customDir, customSettings)
	}

	resetSettings, err := app.SetSpeechLibraryDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	if !resetSettings.IsDefault || resetSettings.Path != "" || resetSettings.ResolvedPath != store.DefaultSpeechLibraryDir() {
		t.Fatalf("expected reset to default speech library path %s, got %#v", store.DefaultSpeechLibraryDir(), resetSettings)
	}

	defaultPathSettings, err := app.SetSpeechLibraryDirectory(store.DefaultSpeechLibraryDir())
	if err != nil {
		t.Fatal(err)
	}
	if !defaultPathSettings.IsDefault || defaultPathSettings.Path != "" {
		t.Fatalf("expected explicit default path to stay default, got %#v", defaultPathSettings)
	}
}

func TestOutputAndSpeechLibrarySettingsRejectSameDirectory(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}

	sharedFromLibrary := filepath.Join(t.TempDir(), "shared-from-library")
	if _, err := app.SetSpeechLibraryDirectory(sharedFromLibrary); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetOutputDirectory(sharedFromLibrary); err == nil {
		t.Fatal("expected output directory matching speech library directory to be rejected")
	} else if !strings.Contains(err.Error(), "cannot be the same as the speech library directory") {
		t.Fatalf("expected clear directory conflict error, got %v", err)
	}

	if _, err := app.SetSpeechLibraryDirectory(""); err != nil {
		t.Fatal(err)
	}
	sharedFromExport := filepath.Join(t.TempDir(), "shared-from-export")
	if _, err := app.SetOutputDirectory(sharedFromExport); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetSpeechLibraryDirectory(sharedFromExport); err == nil {
		t.Fatal("expected speech library directory matching output directory to be rejected")
	} else if !strings.Contains(err.Error(), "cannot be the same as the MP3 export directory") {
		t.Fatalf("expected clear directory conflict error, got %v", err)
	}
}

func TestSaveSpeechItemCreatesMP3MetadataAndDeleteRemovesBoth(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "library voice",
		TargetSamples: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	fillRequiredSamplesForTest(t, store, source, 300)
	exportDir := filepath.Join(t.TempDir(), "exports-only")
	libraryDir := filepath.Join(t.TempDir(), "speech-library")
	app := &App{store: store}
	if _, err := app.SetOutputDirectory(exportDir); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetSpeechLibraryDirectory(libraryDir); err != nil {
		t.Fatal(err)
	}

	item, err := app.SaveSpeechItem(UISaveSpeechItemRequest{
		SourceID:   source.ID,
		SourceName: source.Name,
		Title:      "Greeting",
		Text:       "hello",
		OutputName: "library-save",
		Options: UISynthesisOptions{
			Speed:          1,
			Clarity:        60,
			NoiseReduction: 30,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == "" || item.SourceID != source.ID || item.SourceName != source.Name {
		t.Fatalf("unexpected saved speech item: %#v", item)
	}
	if item.Title != "Greeting" {
		t.Fatalf("expected title in saved speech item UI shape, got %#v", item)
	}
	if filepath.Dir(item.Path) != libraryDir {
		t.Fatalf("expected speech item in library dir %s, got %s", libraryDir, item.Path)
	}
	if filepath.Dir(item.Path) == exportDir {
		t.Fatalf("speech item should not be saved in MP3 export dir %s", exportDir)
	}
	if filepath.Ext(item.Path) != ".mp3" || item.Duration <= 0 {
		t.Fatalf("expected MP3 speech item with duration, got %#v", item)
	}
	if !strings.HasPrefix(item.AudioURL, "data:audio/mpeg;base64,") {
		t.Fatalf("expected playable MP3 data URL, got %q", item.AudioURL)
	}
	data, err := os.ReadFile(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !hasMP3FrameHeader(data) {
		t.Fatalf("expected MP3 frame header, got first bytes % x", data[:min(len(data), 8)])
	}

	items, err := app.ListSpeechItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("expected saved speech item to be listed, got %#v", items)
	}
	if items[0].Title != "Greeting" {
		t.Fatalf("expected listed speech item title, got %#v", items[0])
	}
	if items[0].Duration <= 0 || items[0].AudioURL != "" {
		t.Fatalf("expected listed speech item metadata without eager audio URL, got %#v", items[0])
	}
	audioURL, err := app.GetSpeechItemAudio(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(audioURL, "data:audio/mpeg;base64,") {
		t.Fatalf("expected on-demand MP3 data URL, got %q", audioURL)
	}

	fastItem, err := app.SaveSpeechItem(UISaveSpeechItemRequest{
		SourceID:   source.ID,
		SourceName: source.Name,
		Title:      "Fast Greeting",
		Text:       "hello",
		OutputName: "library-save-fast",
		Options: UISynthesisOptions{
			Speed:          5,
			Clarity:        60,
			NoiseReduction: 30,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fastItem.Duration <= 0 || fastItem.Duration >= item.Duration {
		t.Fatalf("expected speed=5 saved item duration to be shorter than speed=1: normal=%f fast=%f", item.Duration, fastItem.Duration)
	}

	if err := app.DeleteSpeechItem(item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(item.Path); !os.IsNotExist(err) {
		t.Fatalf("expected speech item file to be deleted, stat err=%v", err)
	}
	if err := app.DeleteSpeechItem(fastItem.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fastItem.Path); !os.IsNotExist(err) {
		t.Fatalf("expected fast speech item file to be deleted, stat err=%v", err)
	}
	items, err = app.ListSpeechItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected speech item metadata to be deleted, got %#v", items)
	}
}

func TestAnalyzeTextTreatsSilentLegacyWAVAsMissing(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "legacy silent voice",
		TargetSamples: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveSample(model.SaveSampleRequest{
		SourceID:       source.ID,
		PromptID:       "vowel-a",
		FileName:       "vowel-a.wav",
		MimeType:       "audio/wav",
		DataBase64:     silentWAVDataURL(t),
		Transcript:     "silent legacy sample",
		DurationMillis: 120,
	})
	if err != nil {
		t.Fatal(err)
	}

	app := &App{store: store}
	report, err := app.AnalyzeText(source.ID, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if report.Coverage != 0 || !containsMissingPrompt(report, "vowel-a") {
		t.Fatalf("expected silent WAV to be reported missing, got %#v", report)
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

func TestAnalyzeTextUsesTargetAwareExactSyllables(t *testing.T) {
	exactID := hangul.SyllablePromptID('개')

	legacyStore, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	legacySource, err := legacyStore.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "legacy target",
		TargetSamples: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	fillRequiredSamplesForTest(t, legacyStore, legacySource, 220)
	legacyReport, err := (&App{store: legacyStore}).AnalyzeText(legacySource.ID, "개")
	if err != nil {
		t.Fatal(err)
	}
	if containsMissingPrompt(legacyReport, exactID) {
		t.Fatalf("target 25 should not require exact syllable %s, got %#v", exactID, legacyReport.Missing)
	}

	exactStore, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exactSource, err := exactStore.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "exact target",
		TargetSamples: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, prompt := range requiredPromptDefinitions(exactSource.TargetSamples) {
		if prompt.ID == exactID {
			continue
		}
		_, err := exactStore.SaveSample(model.SaveSampleRequest{
			SourceID:       exactSource.ID,
			PromptID:       prompt.ID,
			FileName:       prompt.ID + ".wav",
			DataBase64:     testWAVDataURL(t, 240+index*5),
			Transcript:     prompt.Text,
			DurationMillis: 120,
		})
		if err != nil {
			t.Fatalf("save sample %s: %v", prompt.ID, err)
		}
	}
	exactReport, err := (&App{store: exactStore}).AnalyzeText(exactSource.ID, "개")
	if err != nil {
		t.Fatal(err)
	}
	if !containsMissingPrompt(exactReport, exactID) {
		t.Fatalf("target 100 should report missing exact syllable %s, got %#v", exactID, exactReport.Missing)
	}
}

func TestSynthesizeToFileUsesTargetAwareExactSyllables(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          "exact synthesis",
		TargetSamples: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	fillRequiredSamplesForTest(t, store, source, 260)
	exactID := hangul.SyllablePromptID('개')

	result, err := (&App{store: store}).synthesizeToFile(model.SynthesisRequest{
		SourceID:   source.ID,
		Text:       "개",
		Format:     "wav",
		OutputName: "target-aware-exact",
		SampleRate: 8000,
		Speed:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, exactID) {
		t.Fatalf("expected synthesis to use exact prompt %s, got message %q", exactID, result.Message)
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

func TestRequiredPromptDefinitionsSupportPresetTargets(t *testing.T) {
	for _, target := range []int{25, 100, 200, 300} {
		if got := len(requiredPromptDefinitions(target)); got != target {
			t.Fatalf("expected %d required prompts, got %d", target, got)
		}
	}
}

func TestFirstTwentyFivePromptIDsRemainStable(t *testing.T) {
	expected := []string{
		"vowel-a",
		"vowel-eo",
		"vowel-o",
		"vowel-u",
		"vowel-eu",
		"vowel-i",
		"vowel-ae",
		"vowel-e",
		"vowel-ya",
		"vowel-yeo",
		"vowel-yo",
		"vowel-yu",
		"rep-ga",
		"rep-na",
		"rep-da",
		"rep-ra",
		"rep-ma",
		"rep-ba",
		"rep-sa",
		"rep-ja",
		"rep-cha",
		"rep-ka",
		"rep-ta",
		"rep-pa",
		"rep-ha",
	}
	expectedTexts := []string{
		"\uC544",
		"\uC5B4",
		"\uC624",
		"\uC6B0",
		"\uC73C",
		"\uC774",
		"\uC560",
		"\uC5D0",
		"\uC57C",
		"\uC5EC",
		"\uC694",
		"\uC720",
		"\uAC00",
		"\uB098",
		"\uB2E4",
		"\uB77C",
		"\uB9C8",
		"\uBC14",
		"\uC0AC",
		"\uC790",
		"\uCC28",
		"\uCE74",
		"\uD0C0",
		"\uD30C",
		"\uD558",
	}
	prompts := requiredPromptDefinitions(25)
	if len(prompts) != len(expected) {
		t.Fatalf("expected %d minimal prompts, got %d", len(expected), len(prompts))
	}
	for index, prompt := range prompts {
		if prompt.ID != expected[index] {
			t.Fatalf("prompt %d changed: expected %s, got %s", index, expected[index], prompt.ID)
		}
		if prompt.Text != expectedTexts[index] {
			t.Fatalf("prompt %d text changed: expected %q, got %q", index, expectedTexts[index], prompt.Text)
		}
	}
}

func TestGeneratedExactSyllablePromptsStartAfterMinimalSet(t *testing.T) {
	prompts := requiredPromptDefinitions(300)
	if len(prompts) != 300 {
		t.Fatalf("expected 300 prompts, got %d", len(prompts))
	}
	for index, prompt := range prompts[25:] {
		runes := []rune(prompt.Text)
		if len(runes) != 1 {
			t.Fatalf("prompt %d should be a single exact syllable, got %#v", index+25, prompt)
		}
		if prompt.ID != hangul.SyllablePromptID(runes[0]) {
			t.Fatalf("prompt %d should use syllable id for %q, got %s", index+25, prompt.Text, prompt.ID)
		}
	}
}

func TestGeneratedExactSyllablePromptsDoNotDuplicateMinimalTexts(t *testing.T) {
	prompts := requiredPromptDefinitions(300)
	minimalTexts := map[string]bool{}
	for _, prompt := range prompts[:25] {
		minimalTexts[prompt.Text] = true
	}
	seenExactTexts := map[string]bool{}
	for index, prompt := range prompts[25:] {
		if minimalTexts[prompt.Text] {
			t.Fatalf("exact prompt %d duplicates minimal text %q", index+25, prompt.Text)
		}
		if seenExactTexts[prompt.Text] {
			t.Fatalf("exact prompt %d duplicates earlier exact text %q", index+25, prompt.Text)
		}
		seenExactTexts[prompt.Text] = true
	}
}

func TestGeneratedExactSyllablePromptsAreBalancedAcrossChoseong(t *testing.T) {
	prompts := requiredPromptDefinitions(100)
	choseongSeen := map[int]bool{}
	for _, prompt := range prompts[25:] {
		runes := []rune(prompt.Text)
		if len(runes) != 1 {
			t.Fatalf("exact prompt should be a single syllable, got %#v", prompt)
		}
		parts, ok := hangul.DecomposeRune(runes[0])
		if !ok {
			t.Fatalf("exact prompt should be Hangul, got %#v", prompt)
		}
		choseongSeen[parts.ChoseongIndex] = true
	}
	if len(choseongSeen) < 15 {
		t.Fatalf("expected target 100 exact prompts to cover many choseong, got %d: %#v", len(choseongSeen), choseongSeen)
	}
	for _, requiredIndex := range []int{0, 2, 3, 6, 9, 18} {
		if !choseongSeen[requiredIndex] {
			t.Fatalf("expected target 100 exact prompts to include choseong index %d, got %#v", requiredIndex, choseongSeen)
		}
	}
}

func TestGaeRemainsExactPromptAtTarget100(t *testing.T) {
	exactText := "\uAC1C"
	exactRune := []rune(exactText)[0]
	exactID := hangul.SyllablePromptID(exactRune)
	prompts := requiredPromptDefinitions(100)
	for _, prompt := range prompts {
		if prompt.ID == exactID && prompt.Text == exactText {
			return
		}
	}
	t.Fatalf("expected %s (%s) to be included in target 100 exact prompts", exactText, exactID)
}

func TestSequenceForTextWithTargetPrefersExactSyllableWithinTarget(t *testing.T) {
	legacySteps, _ := sequenceForTextWithTarget("개", 25)
	if len(legacySteps) != 1 || legacySteps[0].PromptID != "rep-ga" {
		t.Fatalf("target 25 should keep fallback prompt for 개, got %#v", legacySteps)
	}

	exactID := hangul.SyllablePromptID('개')
	exactSteps, _ := sequenceForTextWithTarget("개", 100)
	if len(exactSteps) != 1 || exactSteps[0].PromptID != exactID {
		t.Fatalf("target 100 should use exact syllable %s for 개, got %#v", exactID, exactSteps)
	}
}

func TestTargetSamplesClampToMaxRequiredTarget(t *testing.T) {
	cases := []struct {
		value int
		want  int
	}{
		{value: 0, want: 25},
		{value: 1, want: 25},
		{value: 28, want: 100},
		{value: 80, want: 100},
		{value: 150, want: 200},
		{value: 250, want: 300},
		{value: 999, want: 300},
	}
	for _, tc := range cases {
		if got := normalizeTarget(tc.value); got != tc.want {
			t.Fatalf("normalizeTarget(%d) = %d, want %d", tc.value, got, tc.want)
		}
	}

	source := model.VoiceSource{
		ID:            "voice-over-target",
		Name:          "over target",
		TargetSamples: 999,
	}
	ui := sourceToUI(source, nil)
	if ui.TargetSamples != maxTargetSamples {
		t.Fatalf("expected UI target %d, got %d", maxTargetSamples, ui.TargetSamples)
	}
	if got := len(requiredPromptDefinitions(999)); got != maxTargetSamples {
		t.Fatalf("expected required prompts to clamp to %d, got %d", maxTargetSamples, got)
	}
}

func fillRequiredSamplesForTest(t *testing.T, store *storage.Store, source model.VoiceSource, baseFrequency int) {
	t.Helper()
	for index, prompt := range requiredPromptDefinitions(source.TargetSamples) {
		_, err := store.SaveSample(model.SaveSampleRequest{
			SourceID:       source.ID,
			PromptID:       prompt.ID,
			FileName:       prompt.ID + ".wav",
			DataBase64:     testWAVDataURL(t, baseFrequency+index*7),
			Transcript:     prompt.Text,
			DurationMillis: 120,
		})
		if err != nil {
			t.Fatalf("save sample %s: %v", prompt.ID, err)
		}
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

func silentWAVDataURL(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "silent.wav")
	pcm := make([]int16, 800)
	if err := synth.WriteWAV(path, 8000, pcm); err != nil {
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

func hasMP3FrameHeader(data []byte) bool {
	for index := 0; index+1 < len(data); index++ {
		if data[index] == 0xFF && data[index+1]&0xE0 == 0xE0 {
			return true
		}
	}
	return false
}

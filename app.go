package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"guvoice/internal/catalog"
	"guvoice/internal/hangul"
	"guvoice/internal/ids"
	"guvoice/internal/model"
	"guvoice/internal/storage"
	"guvoice/internal/synth"
)

type App struct {
	ctx   context.Context
	store *storage.Store
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	store, err := storage.Open(storage.DefaultBaseDir())
	if err != nil {
		println("guvoice storage startup error:", err.Error())
		return
	}
	a.store = store
}

func (a *App) ensureStore() (*storage.Store, error) {
	if a.store != nil {
		return a.store, nil
	}
	store, err := storage.Open(storage.DefaultBaseDir())
	if err != nil {
		return nil, err
	}
	a.store = store
	return store, nil
}

func (a *App) getAppInfo() (model.AppInfo, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.AppInfo{}, err
	}
	state := store.StateSnapshot()
	return model.AppInfo{
		Name:                   "guvoice",
		DisplayName:            "구보이스",
		DataDir:                store.BaseDir(),
		MP3ExportDirectory:     store.SettingsSnapshot().MP3ExportDirectory,
		SpeechLibraryDirectory: store.SettingsSnapshot().SpeechLibraryDirectory,
		SelectedVoiceSourceID:  state.SelectedVoiceSourceID,
		MinimumSampleSetID:     catalog.MinimumKoreanSampleSetID,
		FallbackPolicy:         catalog.DefaultFallbackPolicy(),
	}, nil
}

func (a *App) createVoiceSource(req model.CreateVoiceSourceRequest) (model.VoiceSource, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.VoiceSource{}, err
	}
	req.TargetSamples = normalizeTarget(req.TargetSamples)
	return store.CreateVoiceSource(req)
}

func (a *App) listVoiceSources() ([]model.VoiceSource, error) {
	store, err := a.ensureStore()
	if err != nil {
		return nil, err
	}
	return store.ListVoiceSources(), nil
}

func (a *App) selectVoiceSource(sourceID string) (model.VoiceSource, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.VoiceSource{}, err
	}
	return store.SelectVoiceSource(sourceID)
}

func (a *App) listSampleSets() ([]model.SampleSet, error) {
	store, err := a.ensureStore()
	if err != nil {
		return nil, err
	}
	sets := []model.SampleSet{catalog.MinimumKoreanSampleSet()}
	sets = append(sets, store.ListCustomSampleSets()...)
	return sets, nil
}

func (a *App) defineSampleSet(req model.DefineSampleSetRequest) (model.SampleSet, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.SampleSet{}, err
	}
	return store.DefineSampleSet(req)
}

func (a *App) saveSample(req model.SaveSampleRequest) (model.Sample, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.Sample{}, err
	}
	if err := validateWAVSampleBlob(req.FileName, req.DataBase64, req.MimeType); err != nil {
		return model.Sample{}, err
	}
	return store.SaveSample(req)
}

func (a *App) registerUpload(req model.RegisterUploadRequest) (model.Upload, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.Upload{}, err
	}
	if strings.TrimSpace(req.PromptID) != "" && strings.TrimSpace(req.DataBase64) != "" {
		if err := validateWAVSampleBlob(req.FileName, req.DataBase64, req.MimeType); err != nil {
			return model.Upload{}, err
		}
	}
	return store.RegisterUpload(req)
}

func (a *App) analyzeKoreanTextDetails(text string) (model.TextAnalysis, error) {
	return analyzeKoreanText(text), nil
}

func (a *App) checkMissingSamples(req model.CheckMissingSamplesRequest) (model.MissingSampleReport, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.MissingSampleReport{}, err
	}
	sourceID, err := store.ResolveSourceID(req.SourceID)
	if err != nil {
		return model.MissingSampleReport{}, err
	}
	source, err := store.GetVoiceSource(sourceID)
	if err != nil {
		return model.MissingSampleReport{}, err
	}
	analysis := analyzeKoreanText(req.Text)
	samples := usableSamplesOnDisk(store, store.ListSamples(sourceID))
	return buildPromptMissingSampleReport(sourceID, req.Text, analysis, samples, source.TargetSamples), nil
}

func (a *App) synthesizeToFile(req model.SynthesisRequest) (model.SynthesisResult, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.SynthesisResult{}, err
	}
	sourceID, err := store.ResolveSourceID(req.SourceID)
	if err != nil {
		return model.SynthesisResult{}, err
	}
	if strings.TrimSpace(req.Text) == "" {
		return model.SynthesisResult{}, errors.New("text is required")
	}

	analysis := analyzeKoreanText(req.Text)
	source, err := store.GetVoiceSource(sourceID)
	if err != nil {
		return model.SynthesisResult{}, err
	}
	samples := usableSamplesOnDisk(store, store.ListSamples(sourceID))
	missingRequired := missingRequiredPromptIDs(source.TargetSamples, samples)
	if len(missingRequired) > 0 {
		return model.SynthesisResult{}, fmt.Errorf("필수 WAV 샘플이 부족합니다: %s", strings.Join(missingRequired, ", "))
	}
	resultID := ids.New("synth")
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "wav"
	}
	if format != "wav" && format != "mp3" {
		return model.SynthesisResult{}, fmt.Errorf("unsupported synthesis format %q; wav and mp3 are available", format)
	}

	baseName := strings.TrimSpace(req.OutputName)
	if baseName == "" {
		baseName = resultID
	}
	baseName = storage.SafeFileBase(baseName)
	manifestRel := filepath.ToSlash(filepath.Join("exports", baseName+".json"))
	audioRef := filepath.ToSlash(filepath.Join("exports", baseName+"."+format))
	audioPath := filepath.Join(store.BaseDir(), filepath.FromSlash(audioRef))
	if outputPath := strings.TrimSpace(req.OutputPath); outputPath != "" {
		if filepath.IsAbs(outputPath) {
			audioPath = filepath.Clean(outputPath)
		} else {
			audioPath = filepath.Join(store.BaseDir(), outputPath)
		}
		audioRef = audioPath
	} else if format == "mp3" {
		mp3Dir, err := store.ResolveMP3ExportDir()
		if err != nil {
			return model.SynthesisResult{}, err
		}
		audioPath = filepath.Join(mp3Dir, baseName+".mp3")
		if filepath.Clean(mp3Dir) != filepath.Clean(store.ExportsDir()) {
			audioRef = audioPath
		}
	}
	manifestPath := filepath.Join(store.BaseDir(), manifestRel)
	if req.SkipManifest {
		manifestRel = ""
		manifestPath = ""
	}

	steps, usedPromptIDs := sequenceForText(req.Text)
	latestSamples := latestUsableSamplesByPrompt(samples)
	missingUsed := []string{}
	for index := range steps {
		if steps[index].PromptID == "" {
			continue
		}
		sample, ok := latestSamples[steps[index].PromptID]
		if !ok {
			missingUsed = append(missingUsed, steps[index].PromptID)
			continue
		}
		steps[index].Path = filepath.Join(store.BaseDir(), filepath.FromSlash(sample.Path))
	}
	if len(missingUsed) > 0 {
		return model.SynthesisResult{}, fmt.Errorf("입력 텍스트에 필요한 WAV 샘플이 없습니다: %s", strings.Join(missingUsed, ", "))
	}

	missing := buildPromptMissingSampleReport(sourceID, req.Text, analysis, samples, source.TargetSamples)

	options := synth.Options{
		SampleRate:     req.SampleRate,
		Speed:          req.Speed,
		Pitch:          req.Pitch,
		Clarity:        req.Clarity,
		NoiseReduction: req.NoiseReduction,
	}
	var duration int
	if format == "mp3" {
		duration, err = synth.WriteSequenceMP3(audioPath, steps, options)
	} else {
		duration, err = synth.WriteSequenceWAV(audioPath, steps, options)
	}
	if err != nil {
		return model.SynthesisResult{}, err
	}

	result := model.SynthesisResult{
		ID:             resultID,
		SourceID:       sourceID,
		Text:           req.Text,
		Format:         format,
		AudioPath:      audioRef,
		ManifestPath:   manifestRel,
		DurationMillis: duration,
		CreatedAt:      time.Now().UTC(),
		MissingReport:  missing,
		Message:        fmt.Sprintf("샘플 기반 %s를 생성했습니다. 사용한 promptId: %s", strings.ToUpper(format), strings.Join(usedPromptIDs, ", ")),
	}

	if !req.SkipManifest {
		manifest, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return model.SynthesisResult{}, err
		}
		if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
			return model.SynthesisResult{}, err
		}
	}
	if !req.SkipRecord {
		if err := store.RecordSynthesis(result); err != nil {
			return model.SynthesisResult{}, err
		}
	}
	return result, nil
}

func (a *App) exportVoiceSource(req model.ExportRequest) (model.ExportResult, error) {
	store, err := a.ensureStore()
	if err != nil {
		return model.ExportResult{}, err
	}
	sourceID, err := store.ResolveSourceID(req.SourceID)
	if err != nil {
		return model.ExportResult{}, err
	}
	source, err := store.GetVoiceSource(sourceID)
	if err != nil {
		return model.ExportResult{}, err
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "zip"
	}
	if format != "zip" {
		return model.ExportResult{}, fmt.Errorf("unsupported export format %q; zip is available in the MVP skeleton", format)
	}

	outputPath := strings.TrimSpace(req.OutputPath)
	if outputPath == "" {
		outputPath = filepath.Join(store.ExportsDir(), storage.SafeFileBase(source.Name)+"-"+ids.New("export")+".zip")
	} else {
		info, statErr := os.Stat(outputPath)
		if statErr == nil && info.IsDir() {
			outputPath = filepath.Join(outputPath, storage.SafeFileBase(source.Name)+"-"+ids.New("export")+".zip")
		}
		if filepath.Ext(outputPath) == "" {
			outputPath += ".zip"
		}
	}

	samples := store.ListSamples(sourceID)
	result, err := store.ExportVoiceSource(source, samples, outputPath)
	if err != nil {
		return model.ExportResult{}, err
	}
	return result, nil
}

func analyzeKoreanText(text string) model.TextAnalysis {
	analysis := model.TextAnalysis{
		Text:              text,
		FallbackPolicy:    catalog.DefaultFallbackPolicy(),
		RequiredPromptIDs: []string{},
	}
	syllableCounts := map[rune]int{}
	jamoCounts := map[string]int{}
	nonHangulCounts := map[rune]int{}
	required := map[string]struct{}{}

	for _, r := range text {
		analysis.RuneCount++
		parts, ok := hangul.DecomposeRune(r)
		if !ok {
			if !isSkippable(r) {
				nonHangulCounts[r]++
			}
			continue
		}
		analysis.HangulSyllableCount++
		syllableCounts[r]++
		required[hangul.SyllablePromptID(r)] = struct{}{}
		for _, promptID := range hangul.JamoPromptIDs(parts) {
			required[promptID] = struct{}{}
		}
		jamoCounts[hangul.ChoseongPromptID(parts.Choseong)]++
		jamoCounts[hangul.JungseongPromptID(parts.Jungseong)]++
		if parts.HasJongseong() {
			jamoCounts[hangul.JongseongPromptID(parts.Jongseong)]++
		}
	}

	syllables := make([]rune, 0, len(syllableCounts))
	for r := range syllableCounts {
		syllables = append(syllables, r)
	}
	sort.Slice(syllables, func(i, j int) bool { return syllables[i] < syllables[j] })
	for _, r := range syllables {
		parts, _ := hangul.DecomposeRune(r)
		analysis.DistinctSyllables = append(analysis.DistinctSyllables, model.SyllableUsage{
			Text:      string(r),
			CodePoint: fmt.Sprintf("U+%04X", r),
			Count:     syllableCounts[r],
			PromptID:  hangul.SyllablePromptID(r),
			Parts: model.HangulParts{
				Choseong:          parts.Choseong,
				Jungseong:         parts.Jungseong,
				Jongseong:         parts.Jongseong,
				ChoseongPromptID:  hangul.ChoseongPromptID(parts.Choseong),
				JungseongPromptID: hangul.JungseongPromptID(parts.Jungseong),
				JongseongPromptID: hangul.JongseongPromptID(parts.Jongseong),
			},
		})
	}

	jamoIDs := make([]string, 0, len(jamoCounts))
	for id := range jamoCounts {
		jamoIDs = append(jamoIDs, id)
	}
	sort.Strings(jamoIDs)
	for _, id := range jamoIDs {
		analysis.DistinctJamo = append(analysis.DistinctJamo, model.JamoUsage{
			PromptID: id,
			Text:     hangul.TokenFromPromptID(id),
			Count:    jamoCounts[id],
		})
	}

	nonHangul := make([]rune, 0, len(nonHangulCounts))
	for r := range nonHangulCounts {
		nonHangul = append(nonHangul, r)
	}
	sort.Slice(nonHangul, func(i, j int) bool { return nonHangul[i] < nonHangul[j] })
	for _, r := range nonHangul {
		analysis.NonHangulRunes = append(analysis.NonHangulRunes, model.RuneUsage{
			Text:      string(r),
			CodePoint: fmt.Sprintf("U+%04X", r),
			Count:     nonHangulCounts[r],
		})
	}

	for id := range required {
		analysis.RequiredPromptIDs = append(analysis.RequiredPromptIDs, id)
	}
	sort.Strings(analysis.RequiredPromptIDs)
	return analysis
}

func buildPromptMissingSampleReport(sourceID string, text string, analysis model.TextAnalysis, samples []model.Sample, target int) model.MissingSampleReport {
	filled := filledUsablePromptIDs(samples)
	latest := latestUsableSamplesByPrompt(samples)
	missingIDs := map[string]bool{}

	for _, prompt := range requiredPromptDefinitions(target) {
		if !filled[prompt.ID] {
			missingIDs[prompt.ID] = true
		}
	}

	_, usedPromptIDs := sequenceForText(text)
	report := model.MissingSampleReport{
		SourceID:                sourceID,
		Text:                    text,
		Analysis:                analysis,
		FallbackPolicy:          catalog.DefaultFallbackPolicy(),
		MissingPromptIDs:        []string{},
		Resolutions:             []model.FallbackResolution{},
		ReadyForJamoComposition: true,
	}
	for _, promptID := range usedPromptIDs {
		prompt := promptDefinitionForID(promptID)
		resolution := model.FallbackResolution{
			Syllable:  prompt.Text,
			Method:    "prompt_sample",
			PromptIDs: []string{promptID},
		}
		if sample, ok := latest[promptID]; ok {
			resolution.SampleIDs = []string{sample.ID}
		} else {
			resolution.Method = "missing_samples"
			resolution.MissingPromptIDs = []string{promptID}
			missingIDs[promptID] = true
		}
		report.Resolutions = append(report.Resolutions, resolution)
	}
	for promptID := range missingIDs {
		report.MissingPromptIDs = append(report.MissingPromptIDs, promptID)
	}
	sort.Strings(report.MissingPromptIDs)
	report.ReadyForMVP = strings.TrimSpace(text) != "" && len(report.MissingPromptIDs) == 0
	return report
}

func usableSamplesOnDisk(store *storage.Store, samples []model.Sample) []model.Sample {
	usable := make([]model.Sample, 0, len(samples))
	for _, sample := range samples {
		if !sampleUsableForSynthesis(sample) || strings.TrimSpace(sample.Path) == "" {
			continue
		}
		buffer, err := synth.ReadWAV(storedAudioPath(store, sample.Path))
		if err != nil {
			continue
		}
		if !synth.HasAudibleContent(buffer.Samples, buffer.SampleRate) {
			continue
		}
		usable = append(usable, sample)
	}
	return usable
}

func buildMissingSampleReport(sourceID string, text string, analysis model.TextAnalysis, samples []model.Sample) model.MissingSampleReport {
	latestByPrompt := map[string]model.Sample{}
	hasAnyUsableSample := false
	for _, sample := range samples {
		if sample.PromptID == "" {
			continue
		}
		hasAnyUsableSample = true
		existing, ok := latestByPrompt[sample.PromptID]
		if !ok || sample.CreatedAt.After(existing.CreatedAt) {
			latestByPrompt[sample.PromptID] = sample
		}
	}

	report := model.MissingSampleReport{
		SourceID:         sourceID,
		Text:             text,
		Analysis:         analysis,
		FallbackPolicy:   catalog.DefaultFallbackPolicy(),
		MissingPromptIDs: []string{},
		Resolutions:      []model.FallbackResolution{},
		ReadyForMVP:      analysis.HangulSyllableCount == 0 || hasAnyUsableSample,
	}
	missingIDs := map[string]struct{}{}
	missingExact := map[string]model.SyllableUsage{}
	missingJamo := map[string]model.JamoUsage{}

	for _, syllable := range analysis.DistinctSyllables {
		exactID := syllable.PromptID
		if sample, ok := latestByPrompt[exactID]; ok {
			report.Resolutions = append(report.Resolutions, model.FallbackResolution{
				Syllable: syllable.Text,
				Method:   "exact_syllable",
				PromptIDs: []string{
					exactID,
				},
				SampleIDs: []string{sample.ID},
			})
			continue
		}

		missingIDs[exactID] = struct{}{}
		missingExact[exactID] = syllable

		jamoIDs := []string{
			syllable.Parts.ChoseongPromptID,
			syllable.Parts.JungseongPromptID,
		}
		if syllable.Parts.Jongseong != "" {
			jamoIDs = append(jamoIDs, syllable.Parts.JongseongPromptID)
		}
		sampleIDs := []string{}
		unavailableJamo := []string{}
		for _, id := range jamoIDs {
			if sample, ok := latestByPrompt[id]; ok {
				sampleIDs = append(sampleIDs, sample.ID)
			} else {
				missingIDs[id] = struct{}{}
				unavailableJamo = append(unavailableJamo, id)
				missingJamo[id] = model.JamoUsage{
					PromptID: id,
					Text:     hangul.TokenFromPromptID(id),
					Count:    1,
				}
			}
		}

		method := "missing_samples"
		if len(unavailableJamo) == 0 {
			method = "jamo_composition"
		} else if hasAnyUsableSample {
			method = "source_fallback"
		}
		report.Resolutions = append(report.Resolutions, model.FallbackResolution{
			Syllable:         syllable.Text,
			Method:           method,
			PromptIDs:        jamoIDs,
			SampleIDs:        sampleIDs,
			MissingPromptIDs: unavailableJamo,
		})
	}

	for _, usage := range missingExact {
		report.MissingExactSyllables = append(report.MissingExactSyllables, usage)
	}
	sort.Slice(report.MissingExactSyllables, func(i, j int) bool {
		return report.MissingExactSyllables[i].PromptID < report.MissingExactSyllables[j].PromptID
	})
	for _, usage := range missingJamo {
		report.MissingJamo = append(report.MissingJamo, usage)
	}
	sort.Slice(report.MissingJamo, func(i, j int) bool {
		return report.MissingJamo[i].PromptID < report.MissingJamo[j].PromptID
	})
	for id := range missingIDs {
		report.MissingPromptIDs = append(report.MissingPromptIDs, id)
	}
	sort.Strings(report.MissingPromptIDs)
	report.ReadyForJamoComposition = len(report.MissingJamo) == 0
	return report
}

func isSkippable(r rune) bool {
	return r == ' ' || r == '\n' || r == '\r' || r == '\t' || isProsodyRune(r)
}

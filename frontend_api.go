package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"guvoice/internal/catalog"
	"guvoice/internal/ids"
	"guvoice/internal/model"
	"guvoice/internal/storage"
)

const defaultTargetSamples = 25

type UIVoiceSample struct {
	ID         string  `json:"id"`
	PromptID   string  `json:"promptId,omitempty"`
	Label      string  `json:"label"`
	Text       string  `json:"text"`
	Duration   float64 `json:"duration"`
	Origin     string  `json:"origin"`
	CreatedAt  string  `json:"createdAt"`
	AudioName  string  `json:"audioName,omitempty"`
	AudioURL   string  `json:"audioUrl,omitempty"`
	DataBase64 string  `json:"dataBase64,omitempty"`
}

type UIVoiceSource struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Speaker       string          `json:"speaker"`
	Note          string          `json:"note"`
	TargetSamples int             `json:"targetSamples"`
	Samples       []UIVoiceSample `json:"samples"`
	CreatedAt     string          `json:"createdAt"`
	UpdatedAt     string          `json:"updatedAt"`
}

type UICreateSourceInput struct {
	Name          string `json:"name"`
	Speaker       string `json:"speaker"`
	Note          string `json:"note"`
	TargetSamples int    `json:"targetSamples"`
}

type UIUpdateSourcePatch struct {
	Name          *string          `json:"name,omitempty"`
	Speaker       *string          `json:"speaker,omitempty"`
	Note          *string          `json:"note,omitempty"`
	TargetSamples *int             `json:"targetSamples,omitempty"`
	Samples       *[]UIVoiceSample `json:"samples,omitempty"`
}

type UIMissingSample struct {
	Token    string `json:"token"`
	PromptID string `json:"promptId,omitempty"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

type UIAnalysisResult struct {
	Coverage int               `json:"coverage"`
	Matched  int               `json:"matched"`
	Required int               `json:"required"`
	Missing  []UIMissingSample `json:"missing"`
}

type UISynthesisOptions struct {
	Speed          float64 `json:"speed"`
	Pitch          float64 `json:"pitch"`
	Clarity        float64 `json:"clarity"`
	NoiseReduction float64 `json:"noiseReduction"`
}

type UISynthesisRequest struct {
	SourceID string             `json:"sourceId"`
	Text     string             `json:"text"`
	Options  UISynthesisOptions `json:"options"`
}

type UIPreviewResult struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	AudioURL string `json:"audioUrl,omitempty"`
}

type UIExportResult struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	Path        string `json:"path,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
}

type UIEngineStatus struct {
	Mode    string `json:"mode"`
	Label   string `json:"label"`
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
}

func (a *App) GetEngineStatus() (UIEngineStatus, error) {
	store, err := a.ensureStore()
	if err != nil {
		return UIEngineStatus{}, err
	}
	return UIEngineStatus{
		Mode:    "wails",
		Label:   "Wails / Go",
		Ready:   true,
		Message: "데이터는 " + store.BaseDir() + " 아래에 저장됩니다.",
	}, nil
}

func (a *App) ListSources() ([]UIVoiceSource, error) {
	store, err := a.ensureStore()
	if err != nil {
		return nil, err
	}
	sources := store.ListVoiceSources()
	result := make([]UIVoiceSource, 0, len(sources))
	for _, source := range sources {
		result = append(result, sourceToUI(source, store.ListSamples(source.ID)))
	}
	return result, nil
}

func (a *App) CreateSource(input UICreateSourceInput) (UIVoiceSource, error) {
	store, err := a.ensureStore()
	if err != nil {
		return UIVoiceSource{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "새 목소리"
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{
		Name:          name,
		Speaker:       input.Speaker,
		Note:          input.Note,
		Description:   input.Note,
		SampleSetID:   catalog.MinimumKoreanSampleSetID,
		TargetSamples: normalizeTarget(input.TargetSamples),
	})
	if err != nil {
		return UIVoiceSource{}, err
	}
	return sourceToUI(source, store.ListSamples(source.ID)), nil
}

func (a *App) UpdateSource(sourceID string, patch UIUpdateSourcePatch) (UIVoiceSource, error) {
	store, err := a.ensureStore()
	if err != nil {
		return UIVoiceSource{}, err
	}
	source, err := store.GetVoiceSource(sourceID)
	if err != nil {
		return UIVoiceSource{}, err
	}
	name := source.Name
	if patch.Name != nil {
		name = *patch.Name
	}
	speaker := source.Speaker
	if patch.Speaker != nil {
		speaker = *patch.Speaker
	}
	note := source.Note
	if patch.Note != nil {
		note = *patch.Note
	}
	targetSamples := normalizeTarget(source.TargetSamples)
	if patch.TargetSamples != nil {
		targetSamples = normalizeTarget(*patch.TargetSamples)
	}
	updated, err := store.UpdateVoiceSource(sourceID, model.UpdateVoiceSourceRequest{
		Name:          name,
		Speaker:       speaker,
		Note:          note,
		Description:   note,
		TargetSamples: targetSamples,
	})
	if err != nil {
		return UIVoiceSource{}, err
	}
	if patch.Samples != nil {
		keep := map[string]bool{}
		for _, sample := range *patch.Samples {
			keep[sample.ID] = true
		}
		if err := store.ReplaceSamples(sourceID, keep); err != nil {
			return UIVoiceSource{}, err
		}
	}
	return sourceToUI(updated, store.ListSamples(sourceID)), nil
}

func (a *App) DeleteSource(sourceID string) error {
	store, err := a.ensureStore()
	if err != nil {
		return err
	}
	return store.DeleteVoiceSource(sourceID)
}

func (a *App) AddSample(sourceID string, input UIVoiceSample) (UIVoiceSource, error) {
	store, err := a.ensureStore()
	if err != nil {
		return UIVoiceSource{}, err
	}
	data := strings.TrimSpace(input.DataBase64)
	if data == "" {
		return UIVoiceSource{}, errors.New("WAV audio data is required")
	}
	promptID := strings.TrimSpace(input.PromptID)
	if promptID == "" {
		promptID = "frontend-" + storage.SafeFileBase(firstNonEmpty(input.Label, input.Text, ids.New("sample")))
	}
	fileName := strings.TrimSpace(input.AudioName)
	if fileName == "" {
		fileName = promptID + ".wav"
	}
	if err := validateWAVSampleBlob(fileName, data, ""); err != nil {
		return UIVoiceSource{}, err
	}
	if _, err := store.SaveSample(model.SaveSampleRequest{
		SourceID:       sourceID,
		PromptID:       promptID,
		FileName:       fileName,
		DataBase64:     data,
		Transcript:     input.Text,
		DurationMillis: int(input.Duration * 1000),
	}); err != nil {
		return UIVoiceSource{}, err
	}
	source, err := store.GetVoiceSource(sourceID)
	if err != nil {
		return UIVoiceSource{}, err
	}
	return sourceToUI(source, store.ListSamples(sourceID)), nil
}

func (a *App) AnalyzeText(sourceID string, text string) (UIAnalysisResult, error) {
	store, err := a.ensureStore()
	if err != nil {
		return UIAnalysisResult{}, err
	}
	source, err := store.GetVoiceSource(sourceID)
	if err != nil {
		return UIAnalysisResult{}, err
	}
	samples := store.ListSamples(sourceID)
	return analyzeSourceCoverage(source, samples, text), nil
}

func (a *App) Synthesize(req UISynthesisRequest) (UIPreviewResult, error) {
	report, err := a.AnalyzeText(req.SourceID, req.Text)
	if err != nil {
		return UIPreviewResult{}, err
	}
	if hasBlockingMissing(report) {
		return UIPreviewResult{
			ID:      ids.New("preview"),
			Status:  "error",
			Message: "소스가 아직 채워지지 않아 미리듣기를 만들 수 없습니다.",
		}, nil
	}
	result, err := a.synthesizeToFile(model.SynthesisRequest{
		SourceID:   req.SourceID,
		Text:       req.Text,
		Format:     "wav",
		OutputName: "preview-" + ids.New("audio"),
		Speed:      req.Options.Speed,
	})
	if err != nil {
		return UIPreviewResult{
			ID:      ids.New("preview"),
			Status:  "error",
			Message: err.Error(),
		}, nil
	}
	audioURL, err := a.audioDataURL(result.AudioPath)
	if err != nil {
		return UIPreviewResult{}, err
	}
	return UIPreviewResult{
		ID:       result.ID,
		Status:   "ready",
		Message:  "미리듣기 WAV를 생성했습니다.",
		AudioURL: audioURL,
	}, nil
}

func (a *App) ExportMP3(req UISynthesisRequest) (UIExportResult, error) {
	report, err := a.AnalyzeText(req.SourceID, req.Text)
	if err != nil {
		return UIExportResult{}, err
	}
	if hasBlockingMissing(report) {
		return UIExportResult{
			Status:  "error",
			Message: "소스가 아직 채워지지 않아 저장할 수 없습니다.",
		}, nil
	}
	result, err := a.synthesizeToFile(model.SynthesisRequest{
		SourceID:   req.SourceID,
		Text:       req.Text,
		Format:     "wav",
		OutputName: "guvoice-" + time.Now().Format("20060102-150405"),
		Speed:      req.Options.Speed,
	})
	if err != nil {
		return UIExportResult{
			Status:  "error",
			Message: err.Error(),
		}, nil
	}
	store, err := a.ensureStore()
	if err != nil {
		return UIExportResult{}, err
	}
	path := filepath.Join(store.BaseDir(), filepath.FromSlash(result.AudioPath))
	return UIExportResult{
		Status:  "saved",
		Message: "WAV 파일로 저장했습니다.",
		Path:    path,
	}, nil
}

func (a *App) audioDataURL(relPath string) (string, error) {
	store, err := a.ensureStore()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(store.BaseDir(), filepath.FromSlash(relPath)))
	if err != nil {
		return "", err
	}
	return "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(data), nil
}

func sourceToUI(source model.VoiceSource, samples []model.Sample) UIVoiceSource {
	uiSamples := make([]UIVoiceSample, 0, len(samples))
	for _, sample := range samples {
		origin := "recording"
		if strings.Contains(sample.Path, "/uploads/") || strings.Contains(sample.Path, "\\uploads\\") {
			origin = "upload"
		}
		uiSamples = append(uiSamples, UIVoiceSample{
			ID:        sample.ID,
			PromptID:  sample.PromptID,
			Label:     sampleLabel(sample),
			Text:      sample.Transcript,
			Duration:  float64(sample.DurationMillis) / 1000,
			Origin:    origin,
			CreatedAt: formatUITime(sample.CreatedAt),
			AudioName: sample.FileName,
		})
	}
	return UIVoiceSource{
		ID:            source.ID,
		Name:          source.Name,
		Speaker:       firstNonEmpty(source.Speaker, "이름 없음"),
		Note:          firstNonEmpty(source.Note, source.Description),
		TargetSamples: normalizeTarget(source.TargetSamples),
		Samples:       uiSamples,
		CreatedAt:     formatUITime(source.CreatedAt),
		UpdatedAt:     formatUITime(source.UpdatedAt),
	}
}

func analyzeSourceCoverage(source model.VoiceSource, samples []model.Sample, text string) UIAnalysisResult {
	requiredPrompts := requiredPromptDefinitions(source.TargetSamples)
	target := len(requiredPrompts)
	filledIDs := filledUsablePromptIDs(samples)
	matched := 0
	missingPrompts := []promptDefinition{}
	for _, prompt := range requiredPrompts {
		if filledIDs[prompt.ID] {
			matched++
		} else {
			missingPrompts = append(missingPrompts, prompt)
		}
	}
	coverage := 0
	if target > 0 {
		coverage = int(float64(matched) / float64(target) * 100)
	}
	missing := []UIMissingSample{}
	if strings.TrimSpace(text) == "" {
		missing = append(missing, UIMissingSample{
			Token:    "텍스트",
			Reason:   "말할 문장을 입력해야 합니다.",
			Severity: "warn",
		})
	}
	reportedMissing := map[string]bool{}
	for _, prompt := range missingPrompts[:min(len(missingPrompts), 10)] {
		reportedMissing[prompt.ID] = true
		missing = append(missing, UIMissingSample{
			Token:    prompt.Text,
			PromptID: prompt.ID,
			Reason:   prompt.Label + " WAV 샘플이 없습니다.",
			Severity: "missing",
		})
	}
	if len(missingPrompts) > 10 {
		missing = append(missing, UIMissingSample{
			Token:    fmt.Sprintf("+%d개", len(missingPrompts)-10),
			Reason:   "추가 필수 WAV 샘플이 더 필요합니다.",
			Severity: "missing",
		})
	}

	_, usedPromptIDs := sequenceForText(text)
	for _, promptID := range usedPromptIDs {
		if filledIDs[promptID] || reportedMissing[promptID] {
			continue
		}
		prompt := promptDefinitionForID(promptID)
		missing = append(missing, UIMissingSample{
			Token:    firstNonEmpty(prompt.Text, promptID),
			PromptID: promptID,
			Reason:   "입력 텍스트 합성에 필요한 WAV 샘플입니다.",
			Severity: "missing",
		})
		reportedMissing[promptID] = true
	}
	return UIAnalysisResult{
		Coverage: coverage,
		Matched:  matched,
		Required: target,
		Missing:  missing,
	}
}

func hasBlockingMissing(report UIAnalysisResult) bool {
	for _, missing := range report.Missing {
		if missing.Severity == "missing" {
			return true
		}
	}
	return false
}

func sampleLabel(sample model.Sample) string {
	if sample.Transcript != "" {
		return sample.Transcript
	}
	if sample.PromptID != "" {
		return sample.PromptID
	}
	return sample.FileName
}

func normalizeTarget(value int) int {
	maxTarget := len(guvoicePromptCatalog)
	if maxTarget <= 0 {
		maxTarget = defaultTargetSamples
	}
	if value <= 0 {
		value = defaultTargetSamples
	}
	if value > maxTarget {
		return maxTarget
	}
	return value
}

func formatUITime(value time.Time) string {
	if value.IsZero() {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return value.Format(time.RFC3339)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

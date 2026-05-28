package model

import "time"

type AppInfo struct {
	Name                  string           `json:"name"`
	DisplayName           string           `json:"displayName"`
	DataDir               string           `json:"dataDir"`
	SelectedVoiceSourceID string           `json:"selectedVoiceSourceId"`
	MinimumSampleSetID    string           `json:"minimumSampleSetId"`
	FallbackPolicy        []FallbackPolicy `json:"fallbackPolicy"`
}

type State struct {
	Version               int               `json:"version"`
	SelectedVoiceSourceID string            `json:"selectedVoiceSourceId"`
	VoiceSources          []VoiceSource     `json:"voiceSources"`
	CustomSampleSets      []SampleSet       `json:"customSampleSets"`
	Samples               []Sample          `json:"samples"`
	Uploads               []Upload          `json:"uploads"`
	Syntheses             []SynthesisResult `json:"syntheses"`
	Exports               []ExportResult    `json:"exports"`
	UpdatedAt             time.Time         `json:"updatedAt"`
}

type VoiceSource struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Speaker       string    `json:"speaker"`
	Kind          string    `json:"kind"`
	Locale        string    `json:"locale"`
	Note          string    `json:"note"`
	Description   string    `json:"description"`
	SampleSetID   string    `json:"sampleSetId"`
	TargetSamples int       `json:"targetSamples"`
	Selected      bool      `json:"selected"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type CreateVoiceSourceRequest struct {
	Name          string `json:"name"`
	Speaker       string `json:"speaker"`
	Kind          string `json:"kind"`
	Locale        string `json:"locale"`
	Note          string `json:"note"`
	Description   string `json:"description"`
	SampleSetID   string `json:"sampleSetId"`
	TargetSamples int    `json:"targetSamples"`
}

type UpdateVoiceSourceRequest struct {
	Name          string `json:"name"`
	Speaker       string `json:"speaker"`
	Note          string `json:"note"`
	Description   string `json:"description"`
	TargetSamples int    `json:"targetSamples"`
}

type SampleSet struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Locale      string         `json:"locale"`
	Description string         `json:"description"`
	Items       []SamplePrompt `json:"items"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type SamplePrompt struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Label    string   `json:"label"`
	Text     string   `json:"text"`
	Tokens   []string `json:"tokens"`
	Required bool     `json:"required"`
	Notes    string   `json:"notes"`
}

type DefineSampleSetRequest struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Locale      string         `json:"locale"`
	Description string         `json:"description"`
	Items       []SamplePrompt `json:"items"`
}

type SaveSampleRequest struct {
	SourceID       string `json:"sourceId"`
	PromptID       string `json:"promptId"`
	FileName       string `json:"fileName"`
	MimeType       string `json:"mimeType"`
	DataBase64     string `json:"dataBase64"`
	Transcript     string `json:"transcript"`
	DurationMillis int    `json:"durationMillis"`
}

type Sample struct {
	ID             string    `json:"id"`
	SourceID       string    `json:"sourceId"`
	PromptID       string    `json:"promptId"`
	FileName       string    `json:"fileName"`
	Path           string    `json:"path"`
	MimeType       string    `json:"mimeType"`
	SHA256         string    `json:"sha256"`
	Bytes          int64     `json:"bytes"`
	Transcript     string    `json:"transcript"`
	DurationMillis int       `json:"durationMillis"`
	CreatedAt      time.Time `json:"createdAt"`
}

type RegisterUploadRequest struct {
	SourceID       string `json:"sourceId"`
	FileName       string `json:"fileName"`
	MimeType       string `json:"mimeType"`
	DataBase64     string `json:"dataBase64"`
	Transcript     string `json:"transcript"`
	Notes          string `json:"notes"`
	PromptID       string `json:"promptId"`
	DurationMillis int    `json:"durationMillis"`
}

type Upload struct {
	ID             string    `json:"id"`
	SourceID       string    `json:"sourceId"`
	FileName       string    `json:"fileName"`
	Path           string    `json:"path"`
	MimeType       string    `json:"mimeType"`
	SHA256         string    `json:"sha256"`
	Bytes          int64     `json:"bytes"`
	Transcript     string    `json:"transcript"`
	Notes          string    `json:"notes"`
	PromptID       string    `json:"promptId"`
	SampleID       string    `json:"sampleId"`
	DurationMillis int       `json:"durationMillis"`
	CreatedAt      time.Time `json:"createdAt"`
}

type TextAnalysis struct {
	Text                string           `json:"text"`
	RuneCount           int              `json:"runeCount"`
	HangulSyllableCount int              `json:"hangulSyllableCount"`
	DistinctSyllables   []SyllableUsage  `json:"distinctSyllables"`
	DistinctJamo        []JamoUsage      `json:"distinctJamo"`
	NonHangulRunes      []RuneUsage      `json:"nonHangulRunes"`
	RequiredPromptIDs   []string         `json:"requiredPromptIds"`
	FallbackPolicy      []FallbackPolicy `json:"fallbackPolicy"`
}

type SyllableUsage struct {
	Text      string      `json:"text"`
	CodePoint string      `json:"codePoint"`
	Count     int         `json:"count"`
	PromptID  string      `json:"promptId"`
	Parts     HangulParts `json:"parts"`
}

type HangulParts struct {
	Choseong          string `json:"choseong"`
	Jungseong         string `json:"jungseong"`
	Jongseong         string `json:"jongseong"`
	ChoseongPromptID  string `json:"choseongPromptId"`
	JungseongPromptID string `json:"jungseongPromptId"`
	JongseongPromptID string `json:"jongseongPromptId"`
}

type JamoUsage struct {
	Text     string `json:"text"`
	PromptID string `json:"promptId"`
	Count    int    `json:"count"`
}

type RuneUsage struct {
	Text      string `json:"text"`
	CodePoint string `json:"codePoint"`
	Count     int    `json:"count"`
}

type FallbackPolicy struct {
	Order       int    `json:"order"`
	Method      string `json:"method"`
	Description string `json:"description"`
}

type FallbackResolution struct {
	Syllable         string   `json:"syllable"`
	Method           string   `json:"method"`
	PromptIDs        []string `json:"promptIds"`
	SampleIDs        []string `json:"sampleIds"`
	MissingPromptIDs []string `json:"missingPromptIds"`
}

type CheckMissingSamplesRequest struct {
	SourceID string `json:"sourceId"`
	Text     string `json:"text"`
}

type MissingSampleReport struct {
	SourceID                string               `json:"sourceId"`
	Text                    string               `json:"text"`
	Analysis                TextAnalysis         `json:"analysis"`
	MissingPromptIDs        []string             `json:"missingPromptIds"`
	MissingExactSyllables   []SyllableUsage      `json:"missingExactSyllables"`
	MissingJamo             []JamoUsage          `json:"missingJamo"`
	Resolutions             []FallbackResolution `json:"resolutions"`
	FallbackPolicy          []FallbackPolicy     `json:"fallbackPolicy"`
	ReadyForJamoComposition bool                 `json:"readyForJamoComposition"`
	ReadyForMVP             bool                 `json:"readyForMvp"`
}

type SynthesisRequest struct {
	SourceID   string  `json:"sourceId"`
	Text       string  `json:"text"`
	Format     string  `json:"format"`
	OutputName string  `json:"outputName"`
	SampleRate int     `json:"sampleRate"`
	Speed      float64 `json:"speed"`
}

type SynthesisResult struct {
	ID             string              `json:"id"`
	SourceID       string              `json:"sourceId"`
	Text           string              `json:"text"`
	Format         string              `json:"format"`
	AudioPath      string              `json:"audioPath"`
	ManifestPath   string              `json:"manifestPath"`
	DurationMillis int                 `json:"durationMillis"`
	MissingReport  MissingSampleReport `json:"missingReport"`
	Message        string              `json:"message"`
	CreatedAt      time.Time           `json:"createdAt"`
}

type ExportRequest struct {
	SourceID   string `json:"sourceId"`
	Format     string `json:"format"`
	OutputPath string `json:"outputPath"`
}

type ExportResult struct {
	ID        string    `json:"id"`
	SourceID  string    `json:"sourceId"`
	Format    string    `json:"format"`
	Path      string    `json:"path"`
	Bytes     int64     `json:"bytes"`
	CreatedAt time.Time `json:"createdAt"`
}

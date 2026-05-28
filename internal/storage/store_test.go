package storage

import (
	"archive/zip"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"guvoice/internal/model"
	"guvoice/internal/synth"
)

func TestStorePersistsVoiceSourceAndSample(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{Name: "test voice"})
	if err != nil {
		t.Fatal(err)
	}
	sample, err := store.SaveSample(model.SaveSampleRequest{
		SourceID:   source.ID,
		PromptID:   "jamo-choseong-3131",
		FileName:   "sample.wav",
		MimeType:   "audio/wav",
		DataBase64: base64.StdEncoding.EncodeToString([]byte("fake wav payload")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if sample.Path == "" {
		t.Fatal("sample path is empty")
	}
	if _, err := os.Stat(filepath.Join(store.BaseDir(), filepath.FromSlash(sample.Path))); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(store.BaseDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.ListSamples(source.ID); len(got) != 1 {
		t.Fatalf("expected 1 sample after reopen, got %d", len(got))
	}
}

func TestRegisterUploadCanCreatePromptSample(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{Name: "upload voice"})
	if err != nil {
		t.Fatal(err)
	}
	upload, err := store.RegisterUpload(model.RegisterUploadRequest{
		SourceID:   source.ID,
		FileName:   "upload.webm",
		MimeType:   "audio/webm",
		DataBase64: "data:audio/webm;base64," + base64.StdEncoding.EncodeToString([]byte("fake webm payload")),
		PromptID:   "sentence-ko-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if upload.SampleID == "" {
		t.Fatal("expected upload to register a sample")
	}
	if got := store.ListSamples(source.ID); len(got) != 1 || got[0].PromptID != "sentence-ko-001" {
		t.Fatalf("unexpected samples: %#v", got)
	}
}

func TestRegisterUploadStoresTrimmedPromptWAVWhenPossible(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{Name: "trim upload voice"})
	if err != nil {
		t.Fatal(err)
	}
	upload, err := store.RegisterUpload(model.RegisterUploadRequest{
		SourceID:       source.ID,
		FileName:       "upload.wav",
		MimeType:       "audio/wav",
		DataBase64:     "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(paddedWAVData(t)),
		PromptID:       "vowel-a",
		DurationMillis: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := synth.ReadWAV(filepath.Join(store.BaseDir(), filepath.FromSlash(upload.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if len(buffer.Samples) >= 1900 {
		t.Fatalf("expected uploaded prompt WAV to be trimmed, got %d samples", len(buffer.Samples))
	}
	if upload.DurationMillis >= 1000 {
		t.Fatalf("expected upload duration metadata to follow trimmed WAV, got %d", upload.DurationMillis)
	}
}

func TestSaveSampleStoresTrimmedWAVWhenPossible(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{Name: "trim voice"})
	if err != nil {
		t.Fatal(err)
	}
	data := paddedWAVData(t)

	sample, err := store.SaveSample(model.SaveSampleRequest{
		SourceID:       source.ID,
		PromptID:       "vowel-a",
		FileName:       "trim-me.wav",
		MimeType:       "audio/wav",
		DataBase64:     base64.StdEncoding.EncodeToString(data),
		DurationMillis: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := synth.ReadWAV(filepath.Join(store.BaseDir(), filepath.FromSlash(sample.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if len(buffer.Samples) >= 1900 {
		t.Fatalf("expected stored WAV to be trimmed, got %d samples", len(buffer.Samples))
	}
	if sample.DurationMillis >= 1000 {
		t.Fatalf("expected duration metadata to follow trimmed WAV, got %d", sample.DurationMillis)
	}
}

func TestUpdateSettingsPersistsMP3ExportDirectory(t *testing.T) {
	baseDir := t.TempDir()
	store, err := Open(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	exportDir := filepath.Join(t.TempDir(), "mp3 exports")
	settings, err := store.UpdateSettings(model.AppSettings{MP3ExportDirectory: exportDir})
	if err != nil {
		t.Fatal(err)
	}
	if settings.MP3ExportDirectory == "" {
		t.Fatal("expected MP3 export directory to be stored")
	}
	if store.MP3ExportDir() != settings.MP3ExportDirectory {
		t.Fatalf("expected configured MP3 export dir, got %s", store.MP3ExportDir())
	}

	reopened, err := Open(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.SettingsSnapshot().MP3ExportDirectory != settings.MP3ExportDirectory {
		t.Fatalf("settings did not persist: %#v", reopened.SettingsSnapshot())
	}
}

func TestUpdateSettingsTreatsDefaultMP3ExportDirectoryAsDefault(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	settings, err := store.UpdateSettings(model.AppSettings{MP3ExportDirectory: store.ExportsDir()})
	if err != nil {
		t.Fatal(err)
	}
	if settings.MP3ExportDirectory != "" {
		t.Fatalf("expected default export directory to be stored as blank, got %q", settings.MP3ExportDirectory)
	}
	if store.MP3ExportDir() != store.ExportsDir() {
		t.Fatalf("expected effective default export dir, got %s", store.MP3ExportDir())
	}
}

func TestUpdateSettingsRejectsFileAsMP3ExportDirectory(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = store.UpdateSettings(model.AppSettings{MP3ExportDirectory: path})
	if err == nil {
		t.Fatal("expected file path to be rejected as export directory")
	}
}

func TestExportVoiceSourceWritesZip(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateVoiceSource(model.CreateVoiceSourceRequest{Name: "export voice"})
	if err != nil {
		t.Fatal(err)
	}
	sample, err := store.SaveSample(model.SaveSampleRequest{
		SourceID:   source.ID,
		PromptID:   "sentence-ko-001",
		FileName:   "sample.wav",
		MimeType:   "audio/wav",
		DataBase64: base64.StdEncoding.EncodeToString([]byte("fake wav payload")),
	})
	if err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(store.ExportsDir(), "voice.zip")
	result, err := store.ExportVoiceSource(source, []model.Sample{sample}, outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.Bytes == 0 {
		t.Fatal("expected non-empty export")
	}
	reader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	if !names["voice-source.json"] || !names["README.txt"] {
		t.Fatalf("missing export metadata files: %#v", names)
	}
}

func paddedWAVData(t *testing.T) []byte {
	t.Helper()
	pcm := make([]int16, 0, 1900)
	pcm = append(pcm, make([]int16, 700)...)
	for i := 0; i < 500; i++ {
		if i%20 < 10 {
			pcm = append(pcm, 7000)
		} else {
			pcm = append(pcm, -7000)
		}
	}
	pcm = append(pcm, make([]int16, 700)...)
	data, err := synth.EncodeWAV(8000, pcm)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

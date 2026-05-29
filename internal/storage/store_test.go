package storage

import (
	"archive/zip"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"guvoice/internal/model"
	"guvoice/internal/synth"
)

func TestStorePersistsVoiceSourceAndSample(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.TempDir()); err != nil {
		t.Fatalf("expected temp dir to be created: %v", err)
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

func TestOpenMigratesLegacyPreviewArtifactsToTemp(t *testing.T) {
	baseDir := t.TempDir()
	exportsDir := filepath.Join(baseDir, "exports")
	tempDir := filepath.Join(baseDir, "temp")
	if err := os.MkdirAll(exportsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(exportsDir, "preview-legacy.wav"):  "wav",
		filepath.Join(exportsDir, "preview-legacy.json"): "json",
		filepath.Join(exportsDir, "saved-export.mp3"):    "mp3",
		filepath.Join(exportsDir, "preview-legacy.mp3"):  "preview mp3",
		filepath.Join(exportsDir, "voice.zip"):           "zip",
	}
	for path, data := range files {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(exportsDir, "preview-directory.wav"), 0700); err != nil {
		t.Fatal(err)
	}

	store, err := Open(baseDir)
	if err != nil {
		t.Fatal(err)
	}

	assertMissing(t, filepath.Join(store.ExportsDir(), "preview-legacy.wav"))
	assertMissing(t, filepath.Join(store.ExportsDir(), "preview-legacy.json"))
	assertFileContent(t, filepath.Join(store.TempDir(), "preview-legacy.wav"), "wav")
	assertFileContent(t, filepath.Join(store.TempDir(), "preview-legacy.json"), "json")
	assertFileContent(t, filepath.Join(store.ExportsDir(), "saved-export.mp3"), "mp3")
	assertFileContent(t, filepath.Join(store.ExportsDir(), "preview-legacy.mp3"), "preview mp3")
	assertFileContent(t, filepath.Join(store.ExportsDir(), "voice.zip"), "zip")
	if info, err := os.Stat(filepath.Join(store.ExportsDir(), "preview-directory.wav")); err != nil || !info.IsDir() {
		t.Fatalf("expected preview-like directory to remain in exports, info=%#v err=%v", info, err)
	}
}

func TestOpenMigratesLegacyPreviewArtifactsWithoutOverwritingTempFiles(t *testing.T) {
	baseDir := t.TempDir()
	exportsDir := filepath.Join(baseDir, "exports")
	tempDir := filepath.Join(baseDir, "temp")
	if err := os.MkdirAll(exportsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exportsDir, "preview-same.wav"), []byte("legacy"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "preview-same.wav"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(baseDir)
	if err != nil {
		t.Fatal(err)
	}

	assertMissing(t, filepath.Join(store.ExportsDir(), "preview-same.wav"))
	assertFileContent(t, filepath.Join(store.TempDir(), "preview-same.wav"), "existing")
	assertFileContent(t, filepath.Join(store.TempDir(), "preview-same-1.wav"), "legacy")
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

func TestUpdateSettingsRejectsCurrentSpeechLibraryDirectory(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sharedDir := filepath.Join(t.TempDir(), "shared-audio-dir")
	if _, err := store.UpdateSpeechLibraryDirectory(sharedDir); err != nil {
		t.Fatal(err)
	}

	_, err = store.UpdateSettings(model.AppSettings{MP3ExportDirectory: sharedDir})
	if err == nil {
		t.Fatal("expected export directory matching speech library directory to be rejected")
	}
	if !strings.Contains(err.Error(), "cannot be the same as the speech library directory") {
		t.Fatalf("expected clear directory conflict error, got %v", err)
	}
	if store.MP3ExportDir() == sharedDir {
		t.Fatalf("conflicting MP3 export directory should not be stored")
	}
}

func TestUpdateSettingsRejectsTempDirectory(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.UpdateSettings(model.AppSettings{MP3ExportDirectory: store.TempDir()})
	if err == nil {
		t.Fatal("expected temp directory to be rejected as export directory")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "temporary directory") {
		t.Fatalf("expected clear temporary directory error, got %v", err)
	}
	if samePath(store.MP3ExportDir(), store.TempDir()) {
		t.Fatalf("temp directory should not be stored as MP3 export directory")
	}
}

func TestSpeechLibraryDirectoryDefaultsToAppDataFolder(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if store.SettingsSnapshot().SpeechLibraryDirectory != "" {
		t.Fatalf("expected default speech library setting to be blank, got %#v", store.SettingsSnapshot())
	}
	if store.SpeechLibraryDir() != store.DefaultSpeechLibraryDir() {
		t.Fatalf("expected default speech library dir %s, got %s", store.DefaultSpeechLibraryDir(), store.SpeechLibraryDir())
	}
	if _, err := os.Stat(store.DefaultSpeechLibraryDir()); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSpeechLibraryDirectoryPersistsCustomAndNormalizesDefault(t *testing.T) {
	baseDir := t.TempDir()
	store, err := Open(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	customDir := filepath.Join(t.TempDir(), "speech library")
	settings, err := store.UpdateSpeechLibraryDirectory(customDir)
	if err != nil {
		t.Fatal(err)
	}
	if settings.SpeechLibraryDirectory == "" {
		t.Fatal("expected custom speech library directory to be stored")
	}
	if store.SpeechLibraryDir() != settings.SpeechLibraryDirectory {
		t.Fatalf("expected configured speech library dir, got %s", store.SpeechLibraryDir())
	}

	reopened, err := Open(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.SettingsSnapshot().SpeechLibraryDirectory != settings.SpeechLibraryDirectory {
		t.Fatalf("settings did not persist: %#v", reopened.SettingsSnapshot())
	}

	reset, err := reopened.UpdateSpeechLibraryDirectory(reopened.DefaultSpeechLibraryDir())
	if err != nil {
		t.Fatal(err)
	}
	if reset.SpeechLibraryDirectory != "" {
		t.Fatalf("expected default speech library directory to be stored as blank, got %q", reset.SpeechLibraryDirectory)
	}
}

func TestUpdateSpeechLibraryDirectoryRejectsCurrentMP3ExportDirectory(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sharedDir := filepath.Join(t.TempDir(), "shared-audio-dir")
	if _, err := store.UpdateSettings(model.AppSettings{MP3ExportDirectory: sharedDir}); err != nil {
		t.Fatal(err)
	}

	_, err = store.UpdateSpeechLibraryDirectory(sharedDir)
	if err == nil {
		t.Fatal("expected speech library directory matching export directory to be rejected")
	}
	if !strings.Contains(err.Error(), "cannot be the same as the MP3 export directory") {
		t.Fatalf("expected clear directory conflict error, got %v", err)
	}
	if store.SpeechLibraryDir() == sharedDir {
		t.Fatalf("conflicting speech library directory should not be stored")
	}
}

func TestUpdateSpeechLibraryDirectoryRejectsTempDirectory(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.UpdateSpeechLibraryDirectory(store.TempDir())
	if err == nil {
		t.Fatal("expected temp directory to be rejected as speech library directory")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "temporary directory") {
		t.Fatalf("expected clear temporary directory error, got %v", err)
	}
	if samePath(store.SpeechLibraryDir(), store.TempDir()) {
		t.Fatalf("temp directory should not be stored as speech library directory")
	}
}

func TestUpdateSpeechLibraryDirectoryRejectsFilePath(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = store.UpdateSpeechLibraryDirectory(path)
	if err == nil {
		t.Fatal("expected file path to be rejected as speech library directory")
	}
}

func TestDeleteSpeechItemSavesMetadataBeforeDeletingFile(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	itemDir := filepath.Join(store.BaseDir(), "speech-library", "non-empty-item-dir")
	if err := os.MkdirAll(itemDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "keeps-delete-from-succeeding.txt"), []byte("still here"), 0600); err != nil {
		t.Fatal(err)
	}
	item, err := store.AddSpeechItem(model.SpeechItem{
		ID:        "speech-delete-order",
		SourceID:  "voice-delete-order",
		Title:     "Delete order",
		Text:      "hello",
		FileName:  "delete-order.mp3",
		Path:      itemDir,
		Bytes:     1,
		CreatedAt: nowForTest(),
	})
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := store.DeleteSpeechItem(item.ID)
	if err == nil {
		t.Fatal("expected file deletion error for non-empty directory")
	}
	if deleted.ID != "" {
		t.Fatalf("expected empty item on delete error, got %#v", deleted)
	}
	if !strings.Contains(err.Error(), "could not delete speech item file") {
		t.Fatalf("expected clear file deletion error, got %v", err)
	}
	if items := store.ListSpeechItems(); len(items) != 0 {
		t.Fatalf("metadata should be removed after state save even when file deletion fails, got %#v", items)
	}
	if _, statErr := os.Stat(itemDir); statErr != nil {
		t.Fatalf("delete failure should leave file path in place, stat err=%v", statErr)
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

func nowForTest() time.Time {
	return time.Now().UTC()
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, err=%v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("unexpected content for %s: got %q want %q", path, string(data), want)
	}
}

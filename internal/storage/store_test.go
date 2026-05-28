package storage

import (
	"archive/zip"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"guvoice/internal/model"
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

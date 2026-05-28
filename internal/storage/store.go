package storage

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"guvoice/internal/catalog"
	"guvoice/internal/ids"
	"guvoice/internal/model"
	"guvoice/internal/synth"
)

const stateVersion = 1

type Store struct {
	mu         sync.Mutex
	baseDir    string
	samplesDir string
	exportsDir string
	statePath  string
	state      model.State
}

func DefaultBaseDir() string {
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "guvoice")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".guvoice")
	}
	return filepath.Join(".", "guvoice-data")
}

func Open(baseDir string) (*Store, error) {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = DefaultBaseDir()
	}
	store := &Store{
		baseDir:    baseDir,
		samplesDir: filepath.Join(baseDir, "samples"),
		exportsDir: filepath.Join(baseDir, "exports"),
		statePath:  filepath.Join(baseDir, "state.json"),
	}
	if err := os.MkdirAll(store.samplesDir, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(store.exportsDir, 0700); err != nil {
		return nil, err
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) BaseDir() string {
	return s.baseDir
}

func (s *Store) ExportsDir() string {
	return s.exportsDir
}

func (s *Store) MP3ExportDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if dir := strings.TrimSpace(s.state.Settings.MP3ExportDirectory); dir != "" {
		if samePath(dir, s.exportsDir) {
			return s.exportsDir
		}
		return dir
	}
	return s.exportsDir
}

func (s *Store) ResolveMP3ExportDir() (string, error) {
	dir := s.MP3ExportDir()
	if samePath(dir, s.exportsDir) {
		return s.exportsDir, nil
	}
	return prepareWritableDirectory(dir)
}

func (s *Store) IsDefaultMP3ExportDir(dir string) bool {
	dir = strings.TrimSpace(dir)
	return dir == "" || samePath(dir, s.exportsDir)
}

func (s *Store) SettingsSnapshot() model.AppSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Settings
}

func (s *Store) UpdateSettings(settings model.AppSettings) (model.AppSettings, error) {
	dir := strings.TrimSpace(settings.MP3ExportDirectory)
	if dir != "" {
		if s.IsDefaultMP3ExportDir(dir) {
			dir = ""
		}
	}
	if dir != "" {
		prepared, err := prepareWritableDirectory(dir)
		if err != nil {
			return model.AppSettings{}, err
		}
		if samePath(prepared, s.exportsDir) {
			dir = ""
		} else {
			dir = prepared
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Settings.MP3ExportDirectory = dir
	if err := s.saveLocked(); err != nil {
		return model.AppSettings{}, err
	}
	return s.state.Settings, nil
}

func (s *Store) StateSnapshot() model.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneState(s.state)
}

func (s *Store) CreateVoiceSource(req model.CreateVoiceSourceRequest) (model.VoiceSource, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.VoiceSource{}, errors.New("voice source name is required")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "recorded"
	}
	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = "ko-KR"
	}
	sampleSetID := strings.TrimSpace(req.SampleSetID)
	if sampleSetID == "" {
		sampleSetID = catalog.MinimumKoreanSampleSetID
	}
	targetSamples := req.TargetSamples
	if targetSamples <= 0 {
		targetSamples = 25
	}

	now := time.Now().UTC()
	source := model.VoiceSource{
		ID:            ids.New("voice"),
		Name:          name,
		Speaker:       strings.TrimSpace(req.Speaker),
		Kind:          kind,
		Locale:        locale,
		Note:          strings.TrimSpace(req.Note),
		Description:   strings.TrimSpace(req.Description),
		SampleSetID:   sampleSetID,
		TargetSamples: targetSamples,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state.VoiceSources) == 0 || s.state.SelectedVoiceSourceID == "" {
		s.state.SelectedVoiceSourceID = source.ID
		source.Selected = true
	}
	s.state.VoiceSources = append(s.state.VoiceSources, source)
	if err := s.saveLocked(); err != nil {
		return model.VoiceSource{}, err
	}
	return source, nil
}

func (s *Store) UpdateVoiceSource(sourceID string, req model.UpdateVoiceSourceRequest) (model.VoiceSource, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return model.VoiceSource{}, errors.New("sourceId is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.VoiceSource{}, errors.New("voice source name is required")
	}
	targetSamples := req.TargetSamples
	if targetSamples <= 0 {
		targetSamples = 25
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.VoiceSources {
		if s.state.VoiceSources[i].ID == sourceID {
			s.state.VoiceSources[i].Name = name
			s.state.VoiceSources[i].Speaker = strings.TrimSpace(req.Speaker)
			s.state.VoiceSources[i].Note = strings.TrimSpace(req.Note)
			s.state.VoiceSources[i].Description = strings.TrimSpace(req.Description)
			s.state.VoiceSources[i].TargetSamples = targetSamples
			s.state.VoiceSources[i].UpdatedAt = time.Now().UTC()
			if err := s.saveLocked(); err != nil {
				return model.VoiceSource{}, err
			}
			source := s.state.VoiceSources[i]
			source.Selected = source.ID == s.state.SelectedVoiceSourceID
			return source, nil
		}
	}
	return model.VoiceSource{}, fmt.Errorf("voice source %q not found", sourceID)
}

func (s *Store) DeleteVoiceSource(sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return errors.New("sourceId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	nextSources := s.state.VoiceSources[:0]
	for _, source := range s.state.VoiceSources {
		if source.ID == sourceID {
			found = true
			continue
		}
		nextSources = append(nextSources, source)
	}
	if !found {
		return fmt.Errorf("voice source %q not found", sourceID)
	}
	s.state.VoiceSources = nextSources
	s.state.Samples = filterSamplesBySource(s.state.Samples, sourceID)
	s.state.Uploads = filterUploadsBySource(s.state.Uploads, sourceID)
	if s.state.SelectedVoiceSourceID == sourceID {
		s.state.SelectedVoiceSourceID = ""
		if len(s.state.VoiceSources) > 0 {
			s.state.SelectedVoiceSourceID = s.state.VoiceSources[0].ID
		}
	}
	return s.saveLocked()
}

func (s *Store) ListVoiceSources() []model.VoiceSource {
	s.mu.Lock()
	defer s.mu.Unlock()
	sources := append([]model.VoiceSource(nil), s.state.VoiceSources...)
	for i := range sources {
		sources[i].Selected = sources[i].ID == s.state.SelectedVoiceSourceID
	}
	return sources
}

func (s *Store) SelectVoiceSource(sourceID string) (model.VoiceSource, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return model.VoiceSource{}, errors.New("sourceId is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.VoiceSources {
		if s.state.VoiceSources[i].ID == sourceID {
			s.state.SelectedVoiceSourceID = sourceID
			s.state.VoiceSources[i].UpdatedAt = time.Now().UTC()
			if err := s.saveLocked(); err != nil {
				return model.VoiceSource{}, err
			}
			source := s.state.VoiceSources[i]
			source.Selected = true
			return source, nil
		}
	}
	return model.VoiceSource{}, fmt.Errorf("voice source %q not found", sourceID)
}

func (s *Store) ResolveSourceID(sourceID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		sourceID = s.state.SelectedVoiceSourceID
	}
	if sourceID == "" {
		return "", errors.New("no voice source selected")
	}
	for _, source := range s.state.VoiceSources {
		if source.ID == sourceID {
			return sourceID, nil
		}
	}
	return "", fmt.Errorf("voice source %q not found", sourceID)
}

func (s *Store) GetVoiceSource(sourceID string) (model.VoiceSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, source := range s.state.VoiceSources {
		if source.ID == sourceID {
			source.Selected = source.ID == s.state.SelectedVoiceSourceID
			return source, nil
		}
	}
	return model.VoiceSource{}, fmt.Errorf("voice source %q not found", sourceID)
}

func (s *Store) DefineSampleSet(req model.DefineSampleSetRequest) (model.SampleSet, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.SampleSet{}, errors.New("sample set name is required")
	}
	if len(req.Items) == 0 {
		return model.SampleSet{}, errors.New("sample set requires at least one prompt")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = ids.New("sampleset")
	}
	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = "ko-KR"
	}
	set := model.SampleSet{
		ID:          SafeFileBase(id),
		Name:        name,
		Locale:      locale,
		Description: strings.TrimSpace(req.Description),
		Items:       append([]model.SamplePrompt(nil), req.Items...),
		CreatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if set.ID == catalog.MinimumKoreanSampleSetID {
		return model.SampleSet{}, fmt.Errorf("sample set %q already exists", set.ID)
	}
	for _, existing := range s.state.CustomSampleSets {
		if existing.ID == set.ID {
			return model.SampleSet{}, fmt.Errorf("sample set %q already exists", set.ID)
		}
	}
	s.state.CustomSampleSets = append(s.state.CustomSampleSets, set)
	if err := s.saveLocked(); err != nil {
		return model.SampleSet{}, err
	}
	return set, nil
}

func (s *Store) ListCustomSampleSets() []model.SampleSet {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.SampleSet(nil), s.state.CustomSampleSets...)
}

func (s *Store) SaveSample(req model.SaveSampleRequest) (model.Sample, error) {
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		return model.Sample{}, errors.New("sourceId is required")
	}
	promptID := strings.TrimSpace(req.PromptID)
	if promptID == "" {
		return model.Sample{}, errors.New("promptId is required")
	}
	data, mimeType, err := decodeBase64Blob(req.DataBase64, req.MimeType)
	if err != nil {
		return model.Sample{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.sourceExistsLocked(sourceID) {
		return model.Sample{}, fmt.Errorf("voice source %q not found", sourceID)
	}
	sample, err := s.writeSampleLocked(sourceID, promptID, req.FileName, mimeType, data, req.Transcript, req.DurationMillis)
	if err != nil {
		return model.Sample{}, err
	}
	s.state.Samples = append(s.state.Samples, sample)
	if err := s.touchSourceLocked(sourceID); err != nil {
		return model.Sample{}, err
	}
	if err := s.saveLocked(); err != nil {
		return model.Sample{}, err
	}
	return sample, nil
}

func (s *Store) RegisterUpload(req model.RegisterUploadRequest) (model.Upload, error) {
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		return model.Upload{}, errors.New("sourceId is required")
	}
	var data []byte
	mimeType := strings.TrimSpace(req.MimeType)
	var err error
	if strings.TrimSpace(req.DataBase64) != "" {
		data, mimeType, err = decodeBase64Blob(req.DataBase64, req.MimeType)
		if err != nil {
			return model.Upload{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.sourceExistsLocked(sourceID) {
		return model.Upload{}, fmt.Errorf("voice source %q not found", sourceID)
	}

	now := time.Now().UTC()
	uploadID := ids.New("upload")
	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = uploadID + extensionFor(mimeType, "")
	}
	durationMillis := req.DurationMillis
	if strings.TrimSpace(req.PromptID) != "" && len(data) > 0 {
		data, mimeType, durationMillis = normalizeWAVBlobIfPossible(fileName, mimeType, data, durationMillis)
	}
	upload := model.Upload{
		ID:             uploadID,
		SourceID:       sourceID,
		FileName:       filepath.Base(fileName),
		MimeType:       mimeType,
		Transcript:     strings.TrimSpace(req.Transcript),
		Notes:          strings.TrimSpace(req.Notes),
		PromptID:       strings.TrimSpace(req.PromptID),
		DurationMillis: durationMillis,
		CreatedAt:      now,
	}
	if len(data) > 0 {
		relPath, sha, err := s.writeBlobLocked(sourceID, "uploads", uploadID, fileName, mimeType, data)
		if err != nil {
			return model.Upload{}, err
		}
		upload.Path = relPath
		upload.SHA256 = sha
		upload.Bytes = int64(len(data))
	}
	if upload.PromptID != "" && len(data) > 0 {
		sample := model.Sample{
			ID:             ids.New("sample"),
			SourceID:       sourceID,
			PromptID:       upload.PromptID,
			FileName:       upload.FileName,
			Path:           upload.Path,
			MimeType:       upload.MimeType,
			SHA256:         upload.SHA256,
			Bytes:          upload.Bytes,
			Transcript:     upload.Transcript,
			DurationMillis: upload.DurationMillis,
			CreatedAt:      now,
		}
		upload.SampleID = sample.ID
		s.state.Samples = append(s.state.Samples, sample)
	}
	s.state.Uploads = append(s.state.Uploads, upload)
	if err := s.touchSourceLocked(sourceID); err != nil {
		return model.Upload{}, err
	}
	if err := s.saveLocked(); err != nil {
		return model.Upload{}, err
	}
	return upload, nil
}

func (s *Store) ListSamples(sourceID string) []model.Sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	samples := []model.Sample{}
	for _, sample := range s.state.Samples {
		if sample.SourceID == sourceID {
			samples = append(samples, sample)
		}
	}
	return samples
}

func (s *Store) ReplaceSamples(sourceID string, keepSampleIDs map[string]bool) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return errors.New("sourceId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.sourceExistsLocked(sourceID) {
		return fmt.Errorf("voice source %q not found", sourceID)
	}
	next := s.state.Samples[:0]
	for _, sample := range s.state.Samples {
		if sample.SourceID == sourceID && !keepSampleIDs[sample.ID] {
			continue
		}
		next = append(next, sample)
	}
	s.state.Samples = next
	if err := s.touchSourceLocked(sourceID); err != nil {
		return err
	}
	return s.saveLocked()
}

func (s *Store) RecordSynthesis(result model.SynthesisResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Syntheses = append(s.state.Syntheses, result)
	return s.saveLocked()
}

func (s *Store) ExportVoiceSource(source model.VoiceSource, samples []model.Sample, outputPath string) (model.ExportResult, error) {
	if strings.TrimSpace(outputPath) == "" {
		return model.ExportResult{}, errors.New("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return model.ExportResult{}, err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return model.ExportResult{}, err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	manifest := struct {
		Source  model.VoiceSource `json:"source"`
		Samples []model.Sample    `json:"samples"`
	}{
		Source:  source,
		Samples: samples,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		zipWriter.Close()
		return model.ExportResult{}, err
	}
	if err := addZipBytes(zipWriter, "voice-source.json", manifestBytes); err != nil {
		zipWriter.Close()
		return model.ExportResult{}, err
	}
	if err := addZipBytes(zipWriter, "README.txt", []byte("guvoice voice source export\n")); err != nil {
		zipWriter.Close()
		return model.ExportResult{}, err
	}
	for _, sample := range samples {
		if sample.Path == "" {
			continue
		}
		absPath := filepath.Join(s.baseDir, filepath.FromSlash(sample.Path))
		if err := addZipFile(zipWriter, filepath.ToSlash(filepath.Join("samples", filepath.Base(sample.Path))), absPath); err != nil {
			zipWriter.Close()
			return model.ExportResult{}, err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return model.ExportResult{}, err
	}
	info, err := file.Stat()
	if err != nil {
		return model.ExportResult{}, err
	}
	result := model.ExportResult{
		ID:        ids.New("export"),
		SourceID:  source.ID,
		Format:    "zip",
		Path:      outputPath,
		Bytes:     info.Size(),
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Exports = append(s.state.Exports, result)
	if err := s.saveLocked(); err != nil {
		return model.ExportResult{}, err
	}
	return result, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := model.State{
		Version:   stateVersion,
		UpdatedAt: time.Now().UTC(),
	}
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.state = state
			return s.saveLocked()
		}
		return err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.Version == 0 {
		state.Version = stateVersion
	}
	s.state = state
	return nil
}

func (s *Store) saveLocked() error {
	s.state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.baseDir, 0700); err != nil {
		return err
	}
	return os.WriteFile(s.statePath, data, 0600)
}

func (s *Store) writeSampleLocked(sourceID string, promptID string, fileName string, mimeType string, data []byte, transcript string, durationMillis int) (model.Sample, error) {
	data, mimeType, durationMillis = normalizeWAVBlobIfPossible(fileName, mimeType, data, durationMillis)
	sampleID := ids.New("sample")
	relPath, sha, err := s.writeBlobLocked(sourceID, "prompts", sampleID, fileName, mimeType, data)
	if err != nil {
		return model.Sample{}, err
	}
	displayName := filepath.Base(strings.TrimSpace(fileName))
	if displayName == "." || displayName == "" {
		displayName = filepath.Base(relPath)
	}
	return model.Sample{
		ID:             sampleID,
		SourceID:       sourceID,
		PromptID:       promptID,
		FileName:       displayName,
		Path:           relPath,
		MimeType:       mimeType,
		SHA256:         sha,
		Bytes:          int64(len(data)),
		Transcript:     strings.TrimSpace(transcript),
		DurationMillis: durationMillis,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

func (s *Store) writeBlobLocked(sourceID string, section string, id string, fileName string, mimeType string, data []byte) (string, string, error) {
	if len(data) == 0 {
		return "", "", errors.New("audio blob is empty")
	}
	ext := extensionFor(mimeType, fileName)
	name := SafeFileBase(id) + ext
	relPath := filepath.ToSlash(filepath.Join("samples", SafeFileBase(sourceID), SafeFileBase(section), name))
	absPath := filepath.Join(s.baseDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0700); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(absPath, data, 0600); err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	return relPath, hex.EncodeToString(sum[:]), nil
}

func normalizeWAVBlobIfPossible(fileName string, mimeType string, data []byte, durationMillis int) ([]byte, string, int) {
	ext := strings.ToLower(filepath.Ext(fileName))
	if !isWAVBlob(mimeType, ext) {
		return data, mimeType, durationMillis
	}
	normalized, trimmedDuration, err := synth.NormalizeWAVSample(data, filepath.Base(fileName))
	if err != nil {
		return data, mimeType, durationMillis
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "audio/wav"
	}
	if durationMillis <= 0 || trimmedDuration < durationMillis {
		durationMillis = trimmedDuration
	}
	return normalized, mimeType, durationMillis
}

func isWAVBlob(mimeType string, ext string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "audio/wav", "audio/wave", "audio/x-wav", "audio/vnd.wave":
		return true
	}
	return ext == ".wav"
}

func prepareWritableDirectory(dir string) (string, error) {
	cleaned, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return "", fmt.Errorf("invalid MP3 export directory %q: %w", dir, err)
	}
	if err := os.MkdirAll(cleaned, 0700); err != nil {
		return "", fmt.Errorf("could not create MP3 export directory %q: %w", cleaned, err)
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", fmt.Errorf("could not access MP3 export directory %q: %w", cleaned, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("MP3 export path %q is not a directory", cleaned)
	}
	testFile, err := os.CreateTemp(cleaned, ".guvoice-write-test-*")
	if err != nil {
		return "", fmt.Errorf("MP3 export directory %q is not writable: %w", cleaned, err)
	}
	testPath := testFile.Name()
	if err := testFile.Close(); err != nil {
		_ = os.Remove(testPath)
		return "", fmt.Errorf("MP3 export directory %q is not writable: %w", cleaned, err)
	}
	if err := os.Remove(testPath); err != nil {
		return "", fmt.Errorf("MP3 export directory %q is not writable: %w", cleaned, err)
	}
	return cleaned, nil
}

func samePath(left string, right string) bool {
	leftAbs, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbs, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr != nil || rightErr != nil {
		leftClean := filepath.Clean(left)
		rightClean := filepath.Clean(right)
		return leftClean == rightClean || strings.EqualFold(leftClean, rightClean)
	}
	leftClean := filepath.Clean(leftAbs)
	rightClean := filepath.Clean(rightAbs)
	return leftClean == rightClean || strings.EqualFold(leftClean, rightClean)
}

func (s *Store) sourceExistsLocked(sourceID string) bool {
	for _, source := range s.state.VoiceSources {
		if source.ID == sourceID {
			return true
		}
	}
	return false
}

func (s *Store) touchSourceLocked(sourceID string) error {
	for i := range s.state.VoiceSources {
		if s.state.VoiceSources[i].ID == sourceID {
			s.state.VoiceSources[i].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return fmt.Errorf("voice source %q not found", sourceID)
}

func decodeBase64Blob(blob string, fallbackMime string) ([]byte, string, error) {
	blob = strings.TrimSpace(blob)
	if blob == "" {
		return nil, "", errors.New("dataBase64 is required")
	}
	mimeType := strings.TrimSpace(fallbackMime)
	payload := blob
	if strings.HasPrefix(blob, "data:") {
		header, encoded, ok := strings.Cut(blob, ",")
		if !ok {
			return nil, "", errors.New("invalid data URL")
		}
		payload = encoded
		if strings.Contains(header, ";base64") {
			mimeType = strings.TrimPrefix(strings.TrimSuffix(header, ";base64"), "data:")
		}
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		return nil, "", err
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return data, mimeType, nil
}

func extensionFor(mimeType string, fileName string) string {
	if ext := strings.ToLower(filepath.Ext(fileName)); ext != "" {
		return ext
	}
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "audio/wav", "audio/wave", "audio/x-wav":
		return ".wav"
	case "audio/webm":
		return ".webm"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/mp4", "audio/x-m4a":
		return ".m4a"
	case "audio/ogg", "application/ogg":
		return ".ogg"
	default:
		return ".bin"
	}
}

var unsafeFileChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func SafeFileBase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "untitled"
	}
	value = unsafeFileChars.ReplaceAllString(value, "-")
	value = strings.Trim(value, ".-_")
	if value == "" {
		return "untitled"
	}
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}

func addZipBytes(zipWriter *zip.Writer, name string, data []byte) error {
	writer, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func addZipFile(zipWriter *zip.Writer, name string, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, file)
	return err
}

func cloneState(state model.State) model.State {
	state.VoiceSources = append([]model.VoiceSource(nil), state.VoiceSources...)
	state.CustomSampleSets = append([]model.SampleSet(nil), state.CustomSampleSets...)
	state.Samples = append([]model.Sample(nil), state.Samples...)
	state.Uploads = append([]model.Upload(nil), state.Uploads...)
	state.Syntheses = append([]model.SynthesisResult(nil), state.Syntheses...)
	state.Exports = append([]model.ExportResult(nil), state.Exports...)
	return state
}

func filterSamplesBySource(samples []model.Sample, sourceID string) []model.Sample {
	next := samples[:0]
	for _, sample := range samples {
		if sample.SourceID != sourceID {
			next = append(next, sample)
		}
	}
	return next
}

func filterUploadsBySource(uploads []model.Upload, sourceID string) []model.Upload {
	next := uploads[:0]
	for _, upload := range uploads {
		if upload.SourceID != sourceID {
			next = append(next, upload)
		}
	}
	return next
}

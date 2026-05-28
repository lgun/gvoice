package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"guvoice/internal/synth"
)

func validateWAVSampleBlob(fileName string, dataBase64 string, fallbackMime string) error {
	data, mimeType, err := decodeSampleBlob(dataBase64, fallbackMime)
	if err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if mimeType != "" && !isWAVMime(mimeType) && ext != ".wav" {
		return fmt.Errorf("WAV 샘플만 합성에 사용할 수 있습니다. %s 파일은 WAV로 변환해서 업로드하세요", mimeType)
	}
	if _, err := synth.DecodeWAV(data, filepath.Base(fileName)); err != nil {
		return fmt.Errorf("합성 가능한 WAV/PCM 샘플이 아닙니다: %w", err)
	}
	return nil
}

func decodeSampleBlob(blob string, fallbackMime string) ([]byte, string, error) {
	blob = strings.TrimSpace(blob)
	if blob == "" {
		return nil, "", errors.New("WAV audio data is required")
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
	return data, strings.ToLower(strings.TrimSpace(mimeType)), nil
}

func isWAVMime(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "", "audio/wav", "audio/wave", "audio/x-wav", "audio/vnd.wave":
		return true
	default:
		return false
	}
}

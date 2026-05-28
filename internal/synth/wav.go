package synth

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"unicode/utf8"
)

const maxInt16Amplitude = 32767

type Options struct {
	SampleRate int
	Speed      float64
}

func WritePlaceholderWAV(path string, text string, opts Options) (int, error) {
	sampleRate := opts.SampleRate
	if sampleRate <= 0 {
		sampleRate = 22050
	}
	speed := opts.Speed
	if speed <= 0 {
		speed = 1
	}
	runeCount := utf8.RuneCountInString(text)
	durationMillis := int(math.Max(350, float64(runeCount)*85/speed))
	sampleCount := sampleRate * durationMillis / 1000
	pcm := make([]int16, sampleCount)
	for i := range pcm {
		t := float64(i) / float64(sampleRate)
		envelope := 0.18
		if i < sampleRate/100 {
			envelope *= float64(i) / float64(sampleRate/100)
		}
		if remaining := len(pcm) - i; remaining < sampleRate/80 {
			envelope *= float64(remaining) / float64(sampleRate/80)
		}
		carrier := math.Sin(2 * math.Pi * 220 * t)
		pulse := math.Sin(2 * math.Pi * 3.5 * t)
		pcm[i] = int16(carrier * pulse * envelope * maxInt16Amplitude)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return 0, err
	}
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	byteRate := sampleRate * 2
	dataSize := uint32(len(pcm) * 2)
	riffSize := uint32(36) + dataSize
	if _, err := file.Write([]byte("RIFF")); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, riffSize); err != nil {
		return 0, err
	}
	if _, err := file.Write([]byte("WAVEfmt ")); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(16)); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(byteRate)); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(2)); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(16)); err != nil {
		return 0, err
	}
	if _, err := file.Write([]byte("data")); err != nil {
		return 0, err
	}
	if err := binary.Write(file, binary.LittleEndian, dataSize); err != nil {
		return 0, err
	}
	for _, sample := range pcm {
		if err := binary.Write(file, binary.LittleEndian, sample); err != nil {
			return 0, err
		}
	}
	return durationMillis, nil
}

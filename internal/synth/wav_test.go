package synth

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeWAVAcceptsExtensiblePCM16(t *testing.T) {
	pcm := []int16{1000, -1000, 2000, -2000}
	data := extensibleWAVBytes(16000, pcm)

	buffer, err := DecodeWAV(data, "extensible.wav")
	if err != nil {
		t.Fatal(err)
	}
	if buffer.SampleRate != 16000 {
		t.Fatalf("expected 16000 Hz, got %d", buffer.SampleRate)
	}
	if len(buffer.Samples) != len(pcm) {
		t.Fatalf("expected %d samples, got %d", len(pcm), len(buffer.Samples))
	}
	for index, sample := range pcm {
		if buffer.Samples[index] != sample {
			t.Fatalf("sample %d: expected %d, got %d", index, sample, buffer.Samples[index])
		}
	}
}

func TestWriteMP3CreatesFrameHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tone.mp3")
	const sampleRate = 44100
	pcm := make([]int16, sampleRate/2)
	for i := range pcm {
		if i%80 < 40 {
			pcm[i] = 10000
		} else {
			pcm[i] = -10000
		}
	}

	if err := WriteMP3(path, sampleRate, pcm); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".mp3" {
		t.Fatalf("expected .mp3 path, got %s", path)
	}
	if !hasMP3FrameHeader(data) {
		t.Fatalf("expected MP3 frame header, got first bytes % x", data[:min(len(data), 8)])
	}
}

func TestWriteMP3LowSampleRateCreatesFrameHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tone-8000.mp3")
	const sampleRate = 8000
	pcm := make([]int16, sampleRate/2)
	for i := range pcm {
		if i%48 < 24 {
			pcm[i] = 9000
		} else {
			pcm[i] = -9000
		}
	}

	if err := WriteMP3(path, sampleRate, pcm); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".mp3" {
		t.Fatalf("expected .mp3 path, got %s", path)
	}
	if !hasMP3FrameHeader(data) {
		t.Fatalf("expected MP3 frame header, got first bytes % x", data[:min(len(data), 8)])
	}
}

func TestNormalizeWAVSampleTrimsLeadingAndTrailingSilence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "padded.wav")
	pcm := make([]int16, 0, 1900)
	pcm = append(pcm, make([]int16, 700)...)
	for i := 0; i < 500; i++ {
		if i%24 < 12 {
			pcm = append(pcm, 9000)
		} else {
			pcm = append(pcm, -9000)
		}
	}
	pcm = append(pcm, make([]int16, 700)...)
	if err := WriteWAV(path, 8000, pcm); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	normalized, duration, err := NormalizeWAVSample(data, "padded.wav")
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := DecodeWAV(normalized, "normalized.wav")
	if err != nil {
		t.Fatal(err)
	}
	if len(buffer.Samples) >= len(pcm) {
		t.Fatalf("expected trim to shorten sample: before=%d after=%d", len(pcm), len(buffer.Samples))
	}
	if len(buffer.Samples) <= 500 {
		t.Fatalf("expected trim to keep voiced content plus small padding, got %d samples", len(buffer.Samples))
	}
	if duration != len(buffer.Samples)*1000/buffer.SampleRate {
		t.Fatalf("duration mismatch: got %d for %d samples", duration, len(buffer.Samples))
	}
}

func TestShineMonoInputPreservesBlocksForMPEGVersions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sampleRate int
		pass       int
	}{
		{name: "mpeg1", sampleRate: 44100, pass: 1152},
		{name: "mpeg2", sampleRate: 16000, pass: 576},
		{name: "mpeg25", sampleRate: 8000, pass: 576},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pcm := numberedPCM(tc.pass*2 + 37)
			input := shineMonoInput(pcm, tc.sampleRate)
			blocks := (len(pcm) + tc.pass - 1) / tc.pass
			if len(input) != blocks*tc.pass*2 {
				t.Fatalf("expected prepared length %d, got %d", blocks*tc.pass*2, len(input))
			}

			consumed := consumeShineMonoInput(input, tc.pass, len(pcm))
			if !equalSamples(consumed, pcm) {
				t.Fatalf("shine Write-visible samples changed for %s: got %d samples", tc.name, len(consumed))
			}
			for block := 0; block < blocks; block++ {
				first := input[block*tc.pass*2 : block*tc.pass*2+tc.pass]
				skippedDuplicate := input[block*tc.pass*2+tc.pass : block*tc.pass*2+tc.pass*2]
				if !equalSamples(first, skippedDuplicate) {
					t.Fatalf("expected skipped duplicate block %d to match encoder-visible block", block)
				}
			}
		})
	}
}

func TestClarityLowMufflesAndHighSharpensTransientEnergy(t *testing.T) {
	pcm := make([]int16, 1000)
	for i := range pcm {
		if i%32 < 16 {
			pcm[i] = 9000
		} else {
			pcm[i] = -9000
		}
	}

	muffled := applyClarity(pcm, 0)
	clear := applyClarity(pcm, 100)
	if transientEnergy(muffled) >= transientEnergy(clear) {
		t.Fatalf("expected high clarity to preserve more edge energy: low=%d high=%d", transientEnergy(muffled), transientEnergy(clear))
	}
	if equalSamples(muffled, pcm) {
		t.Fatal("expected clarity=0 to alter and muffle PCM")
	}
}

func TestOptionsAffectRenderedPCM(t *testing.T) {
	dir := t.TempDir()
	samplePath := filepath.Join(dir, "sample.wav")
	pcm := []int16{0, 120, 380, 1600, -2200, 2600, -1800, 480, 140, 0}
	if err := WriteWAV(samplePath, 8000, pcm); err != nil {
		t.Fatal(err)
	}
	steps := []SequenceStep{{PromptID: "test", Path: samplePath}}

	plain, _, err := RenderSequence(steps, Options{SampleRate: 8000, Speed: 1})
	if err != nil {
		t.Fatal(err)
	}
	processed, _, err := RenderSequence(steps, Options{
		SampleRate:     8000,
		Speed:          1,
		Pitch:          5,
		Clarity:        100,
		NoiseReduction: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if equalSamples(plain.Samples, processed.Samples) {
		t.Fatal("expected pitch/clarity/noiseReduction options to alter rendered PCM")
	}
	if maxAbs(processed.Samples) <= maxAbs(plain.Samples) {
		t.Fatalf("expected clarity normalization to raise peak: plain=%d processed=%d", maxAbs(plain.Samples), maxAbs(processed.Samples))
	}
}

func TestRenderSequenceAllowsFiveXSpeed(t *testing.T) {
	dir := t.TempDir()
	samplePath := filepath.Join(dir, "sample.wav")
	const sampleRate = 8000
	pcm := make([]int16, sampleRate)
	for i := range pcm {
		if i%40 < 20 {
			pcm[i] = 9000
		} else {
			pcm[i] = -9000
		}
	}
	if err := WriteWAV(samplePath, sampleRate, pcm); err != nil {
		t.Fatal(err)
	}
	steps := []SequenceStep{{PromptID: "test", Path: samplePath}}

	normal, _, err := RenderSequence(steps, Options{SampleRate: sampleRate, Speed: 1})
	if err != nil {
		t.Fatal(err)
	}
	double, _, err := RenderSequence(steps, Options{SampleRate: sampleRate, Speed: 2})
	if err != nil {
		t.Fatal(err)
	}
	fiveX, _, err := RenderSequence(steps, Options{SampleRate: sampleRate, Speed: 5})
	if err != nil {
		t.Fatal(err)
	}

	if len(fiveX.Samples) >= len(double.Samples) {
		t.Fatalf("expected 5x speed not to clamp to 2x length: 2x=%d 5x=%d", len(double.Samples), len(fiveX.Samples))
	}
	if len(fiveX.Samples) >= len(normal.Samples)/3 {
		t.Fatalf("expected 5x speed to produce much shorter output: 1x=%d 5x=%d", len(normal.Samples), len(fiveX.Samples))
	}
}

func numberedPCM(length int) []int16 {
	pcm := make([]int16, length)
	for i := range pcm {
		pcm[i] = int16((i % 30000) - 15000)
	}
	return pcm
}

func consumeShineMonoInput(input []int16, pass int, originalLength int) []int16 {
	consumed := make([]int16, 0, originalLength)
	for offset := 0; offset < len(input); offset += pass * 2 {
		end := offset + pass
		if end > len(input) {
			end = len(input)
		}
		consumed = append(consumed, input[offset:end]...)
	}
	return consumed[:originalLength]
}

func extensibleWAVBytes(sampleRate int, pcm []int16) []byte {
	dataSize := len(pcm) * 2
	totalSize := 12 + 8 + 40 + 8 + dataSize
	data := make([]byte, totalSize)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(totalSize-8))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 40)
	binary.LittleEndian.PutUint16(data[20:22], 0xFFFE)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(data[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	binary.LittleEndian.PutUint16(data[36:38], 22)
	binary.LittleEndian.PutUint16(data[38:40], 16)
	binary.LittleEndian.PutUint32(data[40:44], 0)
	copy(data[44:60], []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71})
	copy(data[60:64], "data")
	binary.LittleEndian.PutUint32(data[64:68], uint32(dataSize))
	offset := 68
	for _, sample := range pcm {
		binary.LittleEndian.PutUint16(data[offset:offset+2], uint16(sample))
		offset += 2
	}
	return data
}

func hasMP3FrameHeader(data []byte) bool {
	for i := 0; i+1 < len(data); i++ {
		if data[i] == 0xFF && data[i+1]&0xE0 == 0xE0 {
			return true
		}
	}
	return false
}

func equalSamples(left []int16, right []int16) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func maxAbs(samples []int16) int {
	peak := 0
	for _, sample := range samples {
		value := abs16(sample)
		if value > peak {
			peak = value
		}
	}
	return peak
}

func transientEnergy(samples []int16) int64 {
	var total int64
	for i := 1; i < len(samples); i++ {
		diff := int(samples[i]) - int(samples[i-1])
		if diff < 0 {
			diff = -diff
		}
		total += int64(diff)
	}
	return total
}

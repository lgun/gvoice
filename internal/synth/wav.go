package synth

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/braheezy/shine-mp3/pkg/mp3"
)

type Options struct {
	SampleRate     int
	Speed          float64
	Pitch          float64
	Clarity        float64
	NoiseReduction float64
	GapMillis      int
}

type Buffer struct {
	SampleRate int
	Samples    []int16
}

type SequenceStep struct {
	PromptID      string
	Path          string
	SilenceMillis int
	Speed         float64
	Gain          float64
	GapMillis     int
	Repeat        int
	Stretch       float64
}

func ReadWAV(path string) (Buffer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Buffer{}, err
	}
	return DecodeWAV(data, filepath.Base(path))
}

func DecodeWAV(data []byte, name string) (Buffer, error) {
	if strings.TrimSpace(name) == "" {
		name = "audio"
	}
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return Buffer{}, fmt.Errorf("%s is not a RIFF/WAVE file", name)
	}

	var (
		audioFormat uint16
		channels    uint16
		sampleRate  uint32
		bitsPer     uint16
		dataChunk   []byte
		seenFmt     bool
	)

	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if chunkSize < 0 || offset+chunkSize > len(data) {
			return Buffer{}, fmt.Errorf("invalid wav chunk size in %s", name)
		}
		chunk := data[offset : offset+chunkSize]
		switch chunkID {
		case "fmt ":
			if len(chunk) < 16 {
				return Buffer{}, fmt.Errorf("invalid wav fmt chunk in %s", name)
			}
			audioFormat = binary.LittleEndian.Uint16(chunk[0:2])
			channels = binary.LittleEndian.Uint16(chunk[2:4])
			sampleRate = binary.LittleEndian.Uint32(chunk[4:8])
			bitsPer = binary.LittleEndian.Uint16(chunk[14:16])
			seenFmt = true
		case "data":
			dataChunk = chunk
		}
		offset += chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}

	if !seenFmt || len(dataChunk) == 0 {
		return Buffer{}, fmt.Errorf("wav fmt/data chunk is missing in %s", name)
	}
	if audioFormat == 0xFFFE {
		var err error
		audioFormat, err = extensibleAudioFormat(data, name)
		if err != nil {
			return Buffer{}, err
		}
	}
	if audioFormat != 1 {
		return Buffer{}, fmt.Errorf("%s uses unsupported wav format %d; PCM is required", name, audioFormat)
	}
	if channels == 0 || channels > 8 {
		return Buffer{}, fmt.Errorf("%s has unsupported channel count %d", name, channels)
	}
	if bitsPer != 16 {
		return Buffer{}, fmt.Errorf("%s is %d-bit wav; 16-bit PCM is required", name, bitsPer)
	}
	if sampleRate == 0 {
		return Buffer{}, fmt.Errorf("%s has an invalid sample rate", name)
	}

	frameSize := int(channels) * 2
	if frameSize == 0 || len(dataChunk) < frameSize {
		return Buffer{}, fmt.Errorf("%s has no pcm frames", name)
	}
	frameCount := len(dataChunk) / frameSize
	pcm := make([]int16, frameCount)
	for frame := 0; frame < frameCount; frame++ {
		base := frame * frameSize
		sum := 0
		for channel := 0; channel < int(channels); channel++ {
			sum += int(int16(binary.LittleEndian.Uint16(dataChunk[base+channel*2 : base+channel*2+2])))
		}
		pcm[frame] = int16(sum / int(channels))
	}

	return Buffer{SampleRate: int(sampleRate), Samples: pcm}, nil
}

func WriteSequenceWAV(path string, steps []SequenceStep, opts Options) (int, error) {
	buffer, duration, err := RenderSequence(steps, opts)
	if err != nil {
		return 0, err
	}
	if err := WriteWAV(path, buffer.SampleRate, buffer.Samples); err != nil {
		return 0, err
	}
	return duration, nil
}

func WriteSequenceMP3(path string, steps []SequenceStep, opts Options) (int, error) {
	opts.SampleRate = normalizedMP3SampleRate(opts.SampleRate)
	buffer, duration, err := RenderSequence(steps, opts)
	if err != nil {
		return 0, err
	}
	if err := WriteMP3(path, buffer.SampleRate, buffer.Samples); err != nil {
		return 0, err
	}
	return duration, nil
}

func RenderSequence(steps []SequenceStep, opts Options) (Buffer, int, error) {
	sampleRate := normalizedSampleRate(opts.SampleRate)
	speed := normalizedSpeed(opts.Speed)
	gapMillis := opts.GapMillis
	if gapMillis <= 0 {
		gapMillis = 18
	}

	output := make([]int16, 0)
	for _, step := range steps {
		if step.SilenceMillis > 0 {
			output = appendSilence(output, sampleRate, int(float64(step.SilenceMillis)/speed))
			continue
		}
		if step.Path == "" {
			return Buffer{}, 0, fmt.Errorf("sample path is empty for %s", step.PromptID)
		}
		buffer, err := ReadWAV(step.Path)
		if err != nil {
			return Buffer{}, 0, err
		}
		pcm := resample(buffer.Samples, buffer.SampleRate, sampleRate)
		pcm = applyNoiseReduction(pcm, opts.NoiseReduction)
		pcm = trimSilenceWithThreshold(pcm, sampleRate, silenceThreshold(opts.NoiseReduction))
		if !HasAudibleContent(pcm, sampleRate) {
			return Buffer{}, 0, fmt.Errorf("sample %s has no audible content", step.PromptID)
		}
		pcm = applyPitchShift(pcm, opts.Pitch)
		stepSpeed := speed * normalizedStepSpeed(step.Speed) / normalizedStretch(step.Stretch)
		pcm = resampleForSpeedFactor(pcm, stepSpeed)
		pcm = applyClarity(pcm, opts.Clarity)
		applyGain(pcm, normalizedGain(step.Gain))
		applyFade(pcm, sampleRate)

		repeat := normalizedRepeat(step.Repeat)
		for count := 0; count < repeat; count++ {
			output = append(output, pcm...)
			if count < repeat-1 {
				output = appendSilence(output, sampleRate, int(float64(max(8, gapMillis/2))/speed))
			}
		}
		stepGapMillis := gapMillis + max(0, step.GapMillis)
		output = appendSilence(output, sampleRate, int(float64(stepGapMillis)/speed))
	}

	if len(output) == 0 {
		return Buffer{}, 0, errors.New("synthesis produced no audio")
	}
	return Buffer{SampleRate: sampleRate, Samples: output}, len(output) * 1000 / sampleRate, nil
}

func WriteWAV(path string, sampleRate int, pcm []int16) error {
	if len(pcm) == 0 {
		return errors.New("pcm is empty")
	}
	sampleRate = normalizedSampleRate(sampleRate)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return writeWAV(file, sampleRate, pcm)
}

func EncodeWAV(sampleRate int, pcm []int16) ([]byte, error) {
	if len(pcm) == 0 {
		return nil, errors.New("pcm is empty")
	}
	var buffer bytes.Buffer
	if err := writeWAV(&buffer, normalizedSampleRate(sampleRate), pcm); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeWAV(file interface {
	Write([]byte) (int, error)
}, sampleRate int, pcm []int16) error {
	byteRate := sampleRate * 2
	dataSize := uint32(len(pcm) * 2)
	riffSize := uint32(36) + dataSize
	if _, err := file.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, riffSize); err != nil {
		return err
	}
	if _, err := file.Write([]byte("WAVEfmt ")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(2)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(16)); err != nil {
		return err
	}
	if _, err := file.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, dataSize); err != nil {
		return err
	}
	for _, sample := range pcm {
		if err := binary.Write(file, binary.LittleEndian, sample); err != nil {
			return err
		}
	}
	return nil
}

func NormalizeWAVSample(data []byte, name string) ([]byte, int, error) {
	buffer, err := DecodeWAV(data, name)
	if err != nil {
		return nil, 0, err
	}
	pcm := TrimSampleSilence(buffer.Samples, buffer.SampleRate)
	if len(pcm) == 0 || !HasAudibleContent(pcm, buffer.SampleRate) {
		return nil, 0, errors.New("wav sample has no audio")
	}
	encoded, err := EncodeWAV(buffer.SampleRate, pcm)
	if err != nil {
		return nil, 0, err
	}
	return encoded, len(pcm) * 1000 / buffer.SampleRate, nil
}

func WriteMP3(path string, sampleRate int, pcm []int16) error {
	if len(pcm) == 0 {
		return errors.New("pcm is empty")
	}
	sourceRate := normalizedSampleRate(sampleRate)
	sampleRate = normalizedMP3SampleRate(sourceRate)
	pcm = resample(pcm, sourceRate, sampleRate)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoderInput := shineMonoInput(pcm, sampleRate)
	encoder := mp3.NewEncoder(sampleRate, 1)
	return encoder.Write(file, encoderInput)
}

func normalizedSampleRate(value int) int {
	if value <= 0 {
		return 44100
	}
	return value
}

func normalizedMP3SampleRate(value int) int {
	supported := []int{44100, 48000, 32000, 22050, 24000, 16000, 11025, 12000, 8000}
	if value <= 0 {
		return 44100
	}
	for _, sampleRate := range supported {
		if value == sampleRate {
			return value
		}
	}
	best := supported[0]
	bestDistance := absInt(value - best)
	for _, sampleRate := range supported[1:] {
		if distance := absInt(value - sampleRate); distance < bestDistance {
			best = sampleRate
			bestDistance = distance
		}
	}
	return best
}

func normalizedSpeed(value float64) float64 {
	if value <= 0 {
		return 1
	}
	if value < 0.5 {
		return 0.5
	}
	if value > 5 {
		return 5
	}
	return value
}

func normalizedStepSpeed(value float64) float64 {
	if value <= 0 {
		return 1
	}
	if value < 0.5 {
		return 0.5
	}
	if value > 2 {
		return 2
	}
	return value
}

func normalizedStretch(value float64) float64 {
	if value <= 0 {
		return 1
	}
	if value < 0.5 {
		return 0.5
	}
	if value > 2.5 {
		return 2.5
	}
	return value
}

func normalizedGain(value float64) float64 {
	if value <= 0 {
		return 1
	}
	if value > 2 {
		return 2
	}
	return value
}

func normalizedRepeat(value int) int {
	if value <= 1 {
		return 1
	}
	if value > 4 {
		return 4
	}
	return value
}

func appendSilence(output []int16, sampleRate int, millis int) []int16 {
	if millis <= 0 {
		return output
	}
	return append(output, make([]int16, sampleRate*millis/1000)...)
}

func trimSilence(pcm []int16, sampleRate int) []int16 {
	return trimSilenceWithThreshold(pcm, sampleRate, 420)
}

func TrimSampleSilence(pcm []int16, sampleRate int) []int16 {
	if len(pcm) == 0 {
		return nil
	}
	peak := peakAbs(pcm)
	if peak < 260 {
		return append([]int16(nil), pcm...)
	}
	threshold := max(260, min(1400, peak/35))
	return trimSilenceWithThreshold(pcm, sampleRate, threshold)
}

func HasAudibleContent(pcm []int16, sampleRate int) bool {
	if len(pcm) == 0 {
		return false
	}
	for _, sample := range pcm {
		if abs16(sample) >= 300 {
			return true
		}
	}
	return false
}

func trimSilenceWithThreshold(pcm []int16, sampleRate int, threshold int) []int16 {
	if len(pcm) == 0 {
		return pcm
	}
	start := 0
	for start < len(pcm) && abs16(pcm[start]) < threshold {
		start++
	}
	if start == len(pcm) {
		return pcm
	}
	end := len(pcm) - 1
	for end > start && abs16(pcm[end]) < threshold {
		end--
	}
	pad := sampleRate / 125
	if start > pad {
		start -= pad
	} else {
		start = 0
	}
	if end+pad < len(pcm) {
		end += pad
	} else {
		end = len(pcm) - 1
	}
	return append([]int16(nil), pcm[start:end+1]...)
}

func silenceThreshold(noiseReduction float64) int {
	level := clampFloat64(noiseReduction, 0, 100) / 100
	return int(math.Round(260 + level*720))
}

func resample(pcm []int16, srcRate int, dstRate int) []int16 {
	if len(pcm) == 0 {
		return nil
	}
	if srcRate <= 0 || dstRate <= 0 || srcRate == dstRate {
		return append([]int16(nil), pcm...)
	}
	outLen := int(math.Max(1, math.Round(float64(len(pcm))*float64(dstRate)/float64(srcRate))))
	return interpolate(pcm, outLen, float64(len(pcm)-1)/float64(max(1, outLen-1)))
}

func resampleForSpeed(pcm []int16, speed float64) []int16 {
	return resampleForSpeedFactor(pcm, normalizedSpeed(speed))
}

func resampleForSpeedFactor(pcm []int16, speed float64) []int16 {
	if len(pcm) == 0 {
		return nil
	}
	if speed <= 0 {
		speed = 1
	}
	if speed < 0.25 {
		speed = 0.25
	}
	if speed > 5 {
		speed = 5
	}
	if math.Abs(speed-1) < 0.001 {
		return append([]int16(nil), pcm...)
	}
	outLen := int(math.Max(1, math.Round(float64(len(pcm))/speed)))
	return interpolate(pcm, outLen, float64(len(pcm)-1)/float64(max(1, outLen-1)))
}

func interpolate(pcm []int16, outLen int, step float64) []int16 {
	output := make([]int16, outLen)
	for i := range output {
		position := float64(i) * step
		left := int(position)
		if left >= len(pcm)-1 {
			output[i] = pcm[len(pcm)-1]
			continue
		}
		frac := position - float64(left)
		value := float64(pcm[left])*(1-frac) + float64(pcm[left+1])*frac
		output[i] = int16(value)
	}
	return output
}

func applyGain(pcm []int16, gain float64) {
	if len(pcm) == 0 || math.Abs(gain-1) < 0.001 {
		return
	}
	for i, sample := range pcm {
		value := int(math.Round(float64(sample) * gain))
		if value > 32767 {
			value = 32767
		} else if value < -32768 {
			value = -32768
		}
		pcm[i] = int16(value)
	}
}

func applyPitchShift(pcm []int16, semitones float64) []int16 {
	if len(pcm) == 0 {
		return nil
	}
	semitones = clampFloat64(semitones, -12, 12)
	if math.Abs(semitones) < 0.001 {
		return append([]int16(nil), pcm...)
	}
	factor := math.Pow(2, semitones/12)
	shifted := resampleForSpeedFactor(pcm, factor)
	if len(shifted) == 0 {
		return append([]int16(nil), pcm...)
	}
	return interpolate(shifted, len(pcm), float64(len(shifted)-1)/float64(max(1, len(pcm)-1)))
}

func applyClarity(pcm []int16, clarity float64) []int16 {
	if len(pcm) == 0 {
		return nil
	}
	level := clampFloat64(clarity, 0, 100) / 100
	output := append([]int16(nil), pcm...)

	if level < 0.5 {
		amount := (0.5 - level) * 2
		alpha := 0.62 - amount*0.43
		var low float64
		for i, sample := range output {
			current := float64(sample)
			low += alpha * (current - low)
			mixed := current*(1-amount*0.7) + low*(amount*0.9)
			if i > 0 && i+1 < len(output) {
				neighbors := (float64(output[i-1]) + float64(pcm[i+1])) * 0.5
				mixed = mixed*(1-amount*0.28) + neighbors*(amount*0.28)
			}
			output[i] = clampInt16(mixed * (1 - amount*0.18))
		}
		normalizePeak(output, int(10500+level*9000))
		return output
	}

	amount := (level - 0.5) * 2
	var low float64
	var previous float64
	alpha := 0.32
	for i, sample := range output {
		current := float64(sample)
		low += alpha * (current - low)
		high := current - low
		transient := current - previous
		mixed := current*(1+amount*0.06) + high*(amount*0.72) + transient*(amount*0.18)
		output[i] = clampInt16(mixed)
		previous = current
	}

	targetPeak := int(16000 + amount*8000)
	normalizePeak(output, targetPeak)
	return output
}

func applyNoiseReduction(pcm []int16, noiseReduction float64) []int16 {
	if len(pcm) == 0 {
		return nil
	}
	level := clampFloat64(noiseReduction, 0, 100) / 100
	if level <= 0 {
		return append([]int16(nil), pcm...)
	}
	threshold := int(math.Round(140 + level*980))
	output := make([]int16, len(pcm))
	for i, sample := range pcm {
		peak := abs16(sample)
		switch {
		case peak < threshold:
			output[i] = 0
		case peak < threshold*2:
			output[i] = int16(float64(sample) * (0.35 + level*0.25))
		default:
			output[i] = sample
		}
	}
	return output
}

func normalizePeak(pcm []int16, targetPeak int) {
	if len(pcm) == 0 || targetPeak <= 0 {
		return
	}
	peak := 0
	for _, sample := range pcm {
		if value := abs16(sample); value > peak {
			peak = value
		}
	}
	if peak == 0 || peak >= targetPeak {
		return
	}
	applyGain(pcm, float64(targetPeak)/float64(peak))
}

func applyFade(pcm []int16, sampleRate int) {
	if len(pcm) == 0 {
		return
	}
	fadeSamples := min(len(pcm)/2, max(1, sampleRate/200))
	for i := 0; i < fadeSamples; i++ {
		gain := float64(i) / float64(fadeSamples)
		pcm[i] = int16(float64(pcm[i]) * gain)
		end := len(pcm) - 1 - i
		pcm[end] = int16(float64(pcm[end]) * gain)
	}
}

func abs16(value int16) int {
	if value < 0 {
		return -int(value)
	}
	return int(value)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func peakAbs(samples []int16) int {
	peak := 0
	for _, sample := range samples {
		if value := abs16(sample); value > peak {
			peak = value
		}
	}
	return peak
}

func clampFloat64(value float64, minValue float64, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampInt16(value float64) int16 {
	rounded := int(math.Round(value))
	if rounded > 32767 {
		return 32767
	}
	if rounded < -32768 {
		return -32768
	}
	return int16(rounded)
}

func shineMonoInput(pcm []int16, sampleRate int) []int16 {
	frameSamples := shineSamplesPerPass(sampleRate)
	blocks := (len(pcm) + frameSamples - 1) / frameSamples
	output := make([]int16, 0, blocks*frameSamples*2)
	for offset := 0; offset < len(pcm); offset += frameSamples {
		end := offset + frameSamples
		block := make([]int16, frameSamples)
		if end > len(pcm) {
			copy(block, pcm[offset:])
		} else {
			copy(block, pcm[offset:end])
		}
		output = append(output, block...)
		output = append(output, block...)
	}
	return output
}

func shineSamplesPerPass(sampleRate int) int {
	switch normalizedMP3SampleRate(sampleRate) {
	case 32000, 44100, 48000:
		return mp3.SHINE_MAX_SAMPLES
	default:
		return mp3.SHINE_MAX_SAMPLES / 2
	}
}

func extensibleAudioFormat(data []byte, name string) (uint16, error) {
	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if chunkSize < 0 || offset+chunkSize > len(data) {
			return 0, fmt.Errorf("invalid wav chunk size in %s", name)
		}
		if chunkID == "fmt " {
			chunk := data[offset : offset+chunkSize]
			if len(chunk) < 40 {
				return 0, fmt.Errorf("invalid WAVE_FORMAT_EXTENSIBLE fmt chunk in %s", name)
			}
			validBits := binary.LittleEndian.Uint16(chunk[18:20])
			if validBits != 0 && validBits != 16 {
				return 0, fmt.Errorf("%s uses %d valid bits; 16-bit PCM is required", name, validBits)
			}
			if !isPCMSubformat(chunk[24:40]) {
				return 0, fmt.Errorf("%s uses unsupported WAVE_FORMAT_EXTENSIBLE subtype; PCM is required", name)
			}
			return 1, nil
		}
		offset += chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	return 0, fmt.Errorf("wav fmt chunk is missing in %s", name)
}

func isPCMSubformat(guid []byte) bool {
	pcmGUID := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71}
	if len(guid) != len(pcmGUID) {
		return false
	}
	for i := range pcmGUID {
		if guid[i] != pcmGUID[i] {
			return false
		}
	}
	return true
}

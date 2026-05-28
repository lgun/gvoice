package synth

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	SampleRate int
	Speed      float64
	GapMillis  int
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
			return 0, fmt.Errorf("sample path is empty for %s", step.PromptID)
		}
		buffer, err := ReadWAV(step.Path)
		if err != nil {
			return 0, err
		}
		pcm := resample(buffer.Samples, buffer.SampleRate, sampleRate)
		pcm = trimSilence(pcm, sampleRate)
		stepSpeed := speed * normalizedStepSpeed(step.Speed) / normalizedStretch(step.Stretch)
		pcm = resampleForSpeedFactor(pcm, stepSpeed)
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
		return 0, errors.New("synthesis produced no audio")
	}
	if err := WriteWAV(path, sampleRate, output); err != nil {
		return 0, err
	}
	return len(output) * 1000 / sampleRate, nil
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

func normalizedSampleRate(value int) int {
	if value <= 0 {
		return 44100
	}
	return value
}

func normalizedSpeed(value float64) float64 {
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
	if len(pcm) == 0 {
		return pcm
	}
	threshold := 420
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
	if speed > 4 {
		speed = 4
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

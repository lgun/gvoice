export type EncodedWav = {
  dataUrl: string;
  previewUrl: string;
  duration: number;
  fileName: string;
  trimmedSamples: number;
};

type WindowWithWebkitAudio = Window & {
  webkitAudioContext?: typeof AudioContext;
};

const getAudioContextConstructor = () =>
  window.AudioContext ?? (window as WindowWithWebkitAudio).webkitAudioContext;

export const mergePcmChunks = (chunks: Float32Array[]) => {
  const totalLength = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
  const merged = new Float32Array(totalLength);
  let offset = 0;
  for (const chunk of chunks) {
    merged.set(chunk, offset);
    offset += chunk.length;
  }
  return merged;
};

export const arrayBufferToBase64 = (buffer: ArrayBuffer) => {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    const chunk = bytes.subarray(offset, offset + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
};

export const encodeMono16Wav = (samples: Float32Array, sampleRate: number) => {
  const dataSize = samples.length * 2;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);

  const writeString = (offset: number, value: string) => {
    for (let i = 0; i < value.length; i += 1) {
      view.setUint8(offset + i, value.charCodeAt(i));
    }
  };

  writeString(0, "RIFF");
  view.setUint32(4, 36 + dataSize, true);
  writeString(8, "WAVE");
  writeString(12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeString(36, "data");
  view.setUint32(40, dataSize, true);

  let offset = 44;
  for (const sample of samples) {
    const clipped = Math.max(-1, Math.min(1, sample));
    view.setInt16(offset, clipped < 0 ? clipped * 0x8000 : clipped * 0x7fff, true);
    offset += 2;
  }

  return buffer;
};

const trimSilence = (samples: Float32Array, sampleRate: number) => {
  const threshold = 0.012;
  const padding = Math.floor(sampleRate * 0.035);
  let first = 0;
  let last = samples.length - 1;

  while (first < samples.length && Math.abs(samples[first]) < threshold) {
    first += 1;
  }
  while (last > first && Math.abs(samples[last]) < threshold) {
    last -= 1;
  }

  if (first >= samples.length) {
    return samples;
  }

  const start = Math.max(0, first - padding);
  const end = Math.min(samples.length, last + padding + 1);
  return samples.slice(start, end);
};

export const encodePcmToTrimmedMonoWav = (
  samples: Float32Array,
  sampleRate: number,
  fileName = "sample.wav"
): EncodedWav => {
  const trimmed = trimSilence(samples, sampleRate);
  const buffer = encodeMono16Wav(trimmed, sampleRate);
  return {
    dataUrl: `data:audio/wav;base64,${arrayBufferToBase64(buffer)}`,
    previewUrl: URL.createObjectURL(new Blob([buffer], { type: "audio/wav" })),
    duration: trimmed.length / sampleRate,
    fileName,
    trimmedSamples: samples.length - trimmed.length
  };
};

export const wavDataUrlFromPcm = (chunks: Float32Array[], sampleRate: number, fileName = "recording.wav") => {
  const samples = mergePcmChunks(chunks);
  return encodePcmToTrimmedMonoWav(samples, sampleRate, fileName);
};

export const decodeAudioFileToMonoWav = async (file: File): Promise<EncodedWav> => {
  const AudioContextConstructor = getAudioContextConstructor();
  if (!AudioContextConstructor) {
    throw new Error("이 환경은 오디오 디코딩을 지원하지 않습니다. WAV 녹음을 사용해 주세요.");
  }

  const context = new AudioContextConstructor();
  try {
    const buffer = await file.arrayBuffer();
    const decoded = await context.decodeAudioData(buffer.slice(0));
    const mono = downmixToMono(decoded);
    const baseName = file.name.replace(/\.[^.]+$/, "") || "upload";
    return encodePcmToTrimmedMonoWav(mono, decoded.sampleRate, `${baseName}.wav`);
  } catch (error) {
    throw new Error(
      error instanceof Error
        ? `WebView2가 이 오디오 파일을 디코딩하지 못했습니다: ${error.message}`
        : "WebView2가 이 오디오 파일을 디코딩하지 못했습니다."
    );
  } finally {
    void context.close();
  }
};

const downmixToMono = (buffer: AudioBuffer) => {
  const samples = new Float32Array(buffer.length);
  for (let channel = 0; channel < buffer.numberOfChannels; channel += 1) {
    const data = buffer.getChannelData(channel);
    for (let i = 0; i < data.length; i += 1) {
      samples[i] += data[i] / buffer.numberOfChannels;
    }
  }
  return samples;
};

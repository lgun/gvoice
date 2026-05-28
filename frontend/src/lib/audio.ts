export type EncodedWav = {
  dataUrl: string;
  previewUrl: string;
  duration: number;
  fileName: string;
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

export const wavDataUrlFromPcm = (chunks: Float32Array[], sampleRate: number) => {
  const samples = mergePcmChunks(chunks);
  const buffer = encodeMono16Wav(samples, sampleRate);
  return {
    dataUrl: `data:audio/wav;base64,${arrayBufferToBase64(buffer)}`,
    previewUrl: URL.createObjectURL(new Blob([buffer], { type: "audio/wav" }))
  };
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
    const wavBuffer = encodeMono16Wav(mono, decoded.sampleRate);
    const baseName = file.name.replace(/\.[^.]+$/, "") || "upload";
    return {
      dataUrl: `data:audio/wav;base64,${arrayBufferToBase64(wavBuffer)}`,
      previewUrl: URL.createObjectURL(new Blob([wavBuffer], { type: "audio/wav" })),
      duration: decoded.duration,
      fileName: `${baseName}.wav`
    };
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

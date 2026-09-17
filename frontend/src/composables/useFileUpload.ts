import { ref } from "vue";
import CryptoJS from "crypto-js";

const HASH_CHUNK_SIZE = 4 * 1024 * 1024;
const MAX_CHUNK_RETRIES = 3;

interface UploadInitResponse {
  instant_upload: boolean;
  code?: string;
  url?: string;
  upload_id: string;
  chunk_size: number;
  total_chunks: number;
  uploaded_chunks: number[];
  workers: number;
}

export interface UploadResult {
  code: string;
  instant_upload: boolean;
  url: string;
}

interface UploadOptions {
  count?: number;
  expire?: number;
}

class RequestError extends Error {
  constructor(
    message: string,
    public readonly status: number,
  ) {
    super(message);
  }
}

async function requestJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, options);
  const data = (await response.json().catch(() => ({}))) as Record<string, unknown>;
  if (!response.ok) {
    throw new RequestError(
      typeof data.error === "string"
        ? data.error
        : `Request failed (${response.status})`,
      response.status,
    );
  }
  return data as T;
}

export function useFileUpload() {
  const uploadStage = ref("");
  const uploadProgress = ref(0);
  const resumedChunks = ref(0);

  function resetUploadProgress() {
    uploadStage.value = "";
    uploadProgress.value = 0;
    resumedChunks.value = 0;
  }

  async function calculateSHA1(file: File): Promise<string> {
    uploadStage.value = "hashing";
    uploadProgress.value = 0;
    const hasher = CryptoJS.algo.SHA1.create();
    const totalChunks = Math.max(1, Math.ceil(file.size / HASH_CHUNK_SIZE));
    for (let index = 0; index < totalChunks; index += 1) {
      const start = index * HASH_CHUNK_SIZE;
      const buffer = await file
        .slice(start, Math.min(start + HASH_CHUNK_SIZE, file.size))
        .arrayBuffer();
      hasher.update(CryptoJS.lib.WordArray.create(buffer));
      uploadProgress.value = Math.round(((index + 1) / totalChunks) * 100);
      await new Promise((resolve) => setTimeout(resolve, 0));
    }
    return hasher.finalize().toString(CryptoJS.enc.Hex);
  }

  async function uploadChunkWithRetry(
    uploadID: string,
    index: number,
    blob: Blob,
  ): Promise<void> {
    let lastError: unknown;
    for (let attempt = 1; attempt <= MAX_CHUNK_RETRIES; attempt += 1) {
      try {
        await requestJSON(`/clip/upload/${uploadID}/${index}`, {
          method: "PUT",
          headers: { "Content-Type": "application/octet-stream" },
          body: blob,
        });
        return;
      } catch (error) {
        lastError = error;
        if (attempt < MAX_CHUNK_RETRIES) {
          await new Promise((resolve) => setTimeout(resolve, attempt * 400));
        }
      }
    }
    throw lastError instanceof Error ? lastError : new Error("Upload failed");
  }

  async function uploadFile(
    file: File,
    { count = 1000, expire = 86400 }: UploadOptions = {},
  ): Promise<UploadResult> {
    const sha1 = await calculateSHA1(file);
    const init = await requestJSON<UploadInitResponse>("/clip/upload/init", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        filename: file.name,
        size: file.size,
        sha1,
        count,
        expire,
      }),
    });
    if (init.instant_upload) {
      if (!init.code || !init.url) throw new Error("Invalid instant upload response");
      return { code: init.code, instant_upload: true, url: init.url };
    }

    uploadStage.value = "uploading";
    resumedChunks.value = init.uploaded_chunks.length;
    const uploaded = new Set(init.uploaded_chunks);
    const remaining = Array.from(
      { length: init.total_chunks },
      (_, index) => index,
    ).filter((index) => !uploaded.has(index));
    let completed = uploaded.size;
    uploadProgress.value =
      init.total_chunks === 0
        ? 100
        : Math.round((completed / init.total_chunks) * 100);
    let cursor = 0;
    const worker = async () => {
      while (cursor < remaining.length) {
        const index = remaining[cursor++];
        if (index === undefined) return;
        const start = index * init.chunk_size;
        await uploadChunkWithRetry(
          init.upload_id,
          index,
          file.slice(start, Math.min(start + init.chunk_size, file.size)),
        );
        completed += 1;
        uploadProgress.value = Math.round(
          (completed / init.total_chunks) * 100,
        );
      }
    };
    const workerCount = Math.max(
      1,
      Math.min(init.workers || 4, remaining.length || 1),
    );
    await Promise.all(Array.from({ length: workerCount }, worker));
    return requestJSON<UploadResult>(`/clip/upload/${init.upload_id}/complete`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ filename: file.name, count, expire }),
    });
  }

  return {
    uploadStage,
    uploadProgress,
    resumedChunks,
    uploadFile,
    resetUploadProgress,
  };
}

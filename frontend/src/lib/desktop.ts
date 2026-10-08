import type { UnlistenFn } from "@tauri-apps/api/event";

type ClipboardCopyPayload = { text: string };

export interface FloatingUploadPayload {
  stage: "hashing" | "uploading" | "complete" | "error" | string;
  filename: string;
  progress: number;
  code?: string | null;
  to_room?: boolean;
  room_id?: string | null;
  error?: string | null;
}

export function isDesktopClient(): boolean {
  return "__TAURI_INTERNALS__" in window;
}

export async function listenForDesktopCopies(handler: (text: string) => void): Promise<UnlistenFn | undefined> {
  if (!isDesktopClient()) return undefined;
  const { listen } = await import("@tauri-apps/api/event");
  return listen<ClipboardCopyPayload>("clipboard://copy", ({ payload }) => handler(payload.text));
}

export async function listenForFloatingUploads(
  handler: (payload: FloatingUploadPayload) => void,
): Promise<UnlistenFn | undefined> {
  if (!isDesktopClient()) return undefined;
  const { listen } = await import("@tauri-apps/api/event");
  return listen<FloatingUploadPayload>("floating://upload", ({ payload }) => handler(payload));
}

export async function setDesktopSharedText(text: string | null): Promise<void> {
  if (!isDesktopClient()) return;
  const { invoke } = await import("@tauri-apps/api/core");
  await invoke("set_shared_text", { text });
}

export async function setDesktopBackendOrigin(origin: string): Promise<void> {
  if (!isDesktopClient()) return;
  const { invoke } = await import("@tauri-apps/api/core");
  await invoke("set_backend_origin", { origin });
}

export async function setDesktopUploadTarget(roomId: string | null): Promise<void> {
  if (!isDesktopClient()) return;
  const { invoke } = await import("@tauri-apps/api/core");
  await invoke("set_upload_target", { roomId });
}

export async function saveDesktopRoomFile(url: string, token: string, filename: string): Promise<string> {
  const { invoke } = await import("@tauri-apps/api/core");
  return invoke<string>("save_room_file", { url, token, filename });
}

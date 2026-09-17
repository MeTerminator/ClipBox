import type { UnlistenFn } from "@tauri-apps/api/event";

type ClipboardCopyPayload = { text: string };

export function isDesktopClient(): boolean {
  return "__TAURI_INTERNALS__" in window;
}

export async function listenForDesktopCopies(handler: (text: string) => void): Promise<UnlistenFn | undefined> {
  if (!isDesktopClient()) return undefined;
  const { listen } = await import("@tauri-apps/api/event");
  return listen<ClipboardCopyPayload>("clipboard://copy", ({ payload }) => handler(payload.text));
}

export async function setDesktopSharedText(text: string | null): Promise<void> {
  if (!isDesktopClient()) return;
  const { invoke } = await import("@tauri-apps/api/core");
  await invoke("set_shared_text", { text });
}

const configuredOrigin = (import.meta.env.VITE_API_ORIGIN as string | undefined)?.trim().replace(/\/$/, "");
const desktopDefaultOrigin = "__TAURI_INTERNALS__" in window ? "http://127.0.0.1:5328" : "";

export const backendOrigin = configuredOrigin || desktopDefaultOrigin || window.location.origin;

export function backendURL(path: string): string {
  if (/^https?:\/\//i.test(path)) return path;
  return new URL(path, `${backendOrigin}/`).toString();
}

export function backendWebSocketURL(path: string): string {
  const url = new URL(path, `${backendOrigin}/`);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}

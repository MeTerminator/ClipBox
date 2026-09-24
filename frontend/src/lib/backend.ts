const configuredOrigin = (import.meta.env.VITE_API_ORIGIN as string | undefined)?.trim().replace(/\/$/, "");
const desktopDefaultOrigin = "__TAURI_INTERNALS__" in window ? "http://127.0.0.1:5328" : "";

export const backendOrigin = configuredOrigin || desktopDefaultOrigin || window.location.origin;

export function backendURL(path: string, origin = backendOrigin): string {
  if (/^https?:\/\//i.test(path)) return path;
  return new URL(path, `${origin}/`).toString();
}

interface PublicConfig {
  site_url?: string;
  upload_direct_first?: boolean;
}

let uploadOriginPromise: Promise<string> | undefined;

async function detectUploadOrigin(): Promise<string> {
  try {
    const configResponse = await fetch(backendURL("/api/config"), { cache: "no-store" });
    if (!configResponse.ok) return backendOrigin;
    const config = (await configResponse.json()) as PublicConfig;
    const siteURL = config.site_url?.trim().replace(/\/$/, "");
    if (!config.upload_direct_first || !siteURL) return backendOrigin;

    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 3000);
    try {
      // Upload routes always allow CORS, so a readable response (including a
      // 404 for this probe ID) proves the browser can reach the direct API.
      await fetch(`${siteURL}/api/clip/upload/__direct_probe__`, {
        cache: "no-store",
        signal: controller.signal,
      });
      return siteURL;
    } finally {
      window.clearTimeout(timeout);
    }
  } catch {
    return backendOrigin;
  }
}

export function uploadBackendURL(path: string): Promise<string> {
  uploadOriginPromise ??= detectUploadOrigin();
  return uploadOriginPromise.then((origin) => backendURL(path, origin));
}

export function backendWebSocketURL(path: string): string {
  const url = new URL(path, `${backendOrigin}/`);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}

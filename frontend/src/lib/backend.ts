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

interface StoredUploadRoute {
  site_url: string;
  direct: boolean;
}

const uploadRouteStorageKey = `clipbox.upload-route:${backendOrigin}`;
let uploadOriginPromise: Promise<string> | undefined;

async function detectUploadOrigin(): Promise<string> {
  let siteURL = "";
  let directFirst = false;
  try {
    const configResponse = await fetch(backendURL("/api/config"), { cache: "no-store" });
    if (!configResponse.ok) return backendOrigin;
    const config = (await configResponse.json()) as PublicConfig;
    siteURL = config.site_url?.trim().replace(/\/$/, "") || "";
    directFirst = config.upload_direct_first === true;
  } catch {
    return backendOrigin;
  }

  try {
    const stored = localStorage.getItem(uploadRouteStorageKey);
    if (stored) {
      const route = JSON.parse(stored) as StoredUploadRoute;
      if (route.site_url === siteURL && route.direct === (directFirst && !!siteURL)) {
        return route.direct ? siteURL : backendOrigin;
      }
    }
  } catch {
    // Storage can be unavailable in private browsing; probe for this visit.
  }

  if (!directFirst || !siteURL) {
    persistUploadRoute(siteURL, false);
    return backendOrigin;
  }

  let direct = false;
  try {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 3000);
    try {
      const response = await fetch(`${siteURL}/api/clip/upload/__direct_probe__`, {
        cache: "no-store",
        signal: controller.signal,
      });
      if (response.ok) {
        const result = (await response.json()) as { direct_upload?: boolean };
        direct = result.direct_upload === true;
      }
    } finally {
      window.clearTimeout(timeout);
    }
  } catch {
    direct = false;
  }

  persistUploadRoute(siteURL, direct);
  return direct ? siteURL : backendOrigin;
}

function persistUploadRoute(siteURL: string, direct: boolean) {
  try {
    localStorage.setItem(uploadRouteStorageKey, JSON.stringify({ site_url: siteURL, direct } satisfies StoredUploadRoute));
  } catch {
    // The route selection remains available in memory for this page session.
  }
}

export function initializeUploadRoute(): Promise<string> {
  uploadOriginPromise ??= detectUploadOrigin();
  return uploadOriginPromise;
}

export function uploadBackendURL(path: string): Promise<string> {
  return initializeUploadRoute().then((origin) => backendURL(path, origin));
}

export function backendWebSocketURL(path: string): string {
  const url = new URL(path, `${backendOrigin}/`);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}

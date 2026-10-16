// Plugin-owned handoff state. The host owns all authentication storage.
export const pendingAssistanceKey = "idc-core:pending-assistance";
export class AssistanceLaunchError extends Error {}

export interface PendingAssistance {
  token: string;
  sessionNo: string;
  expiresAt: number;
}

export function readPendingAssistance(): PendingAssistance | null {
  try {
    const value = JSON.parse(sessionStorage.getItem(pendingAssistanceKey) || "null");
    if (
      value && typeof value.token === "string" && /^[A-Za-z0-9_-]{32,128}$/u.test(value.token) &&
      typeof value.sessionNo === "string" && value.sessionNo.length > 0 &&
      typeof value.expiresAt === "number" && value.expiresAt > Date.now()
    ) return value;
  } catch { /* Unavailable storage must not affect ordinary login. */ }
  clearPendingAssistance();
  return null;
}

export function clearPendingAssistance() {
  try { sessionStorage.removeItem(pendingAssistanceKey); } catch { /* No handoff to resume. */ }
}

export function assistanceLaunchURL(launchUrl: string, sessionNo: string, version: string) {
  if (!/^v?\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.-]+)?$/u.test(version)) {
    throw new AssistanceLaunchError("Customer assistance plugin version is unavailable");
  }
  const url = new URL(launchUrl);
  const token = url.searchParams.get("token") || "";
  if (!/^https?:$/u.test(url.protocol) || !/^[A-Za-z0-9_-]{32,128}$/u.test(token) || url.pathname !== "/account/assistance") {
    throw new AssistanceLaunchError("Customer assistance launch URL is invalid");
  }
  url.pathname = `/x-assets/idc-core/${encodeURIComponent(version)}/assistance-launch.html`;
  url.search = "";
  url.hash = new URLSearchParams({ token, sessionNo }).toString();
  return url.href;
}

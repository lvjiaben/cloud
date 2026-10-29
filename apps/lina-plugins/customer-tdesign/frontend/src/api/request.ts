// Same-origin public API client. Session state is private to this template.
export type RequestOptions = {
  params?: object;
  data?: unknown;
  method?: string;
  headers?: Record<string, string>;
  timeout?: number;
  silentErrorMessage?: boolean;
  responseType?: string;
  signal?: AbortSignal;
};
const sessionKey = "customer-tdesign.session";
export type Session = { accessToken: string; refreshToken?: string };
export function getSession(): Session | null {
  try {
    const value = JSON.parse(sessionStorage.getItem(sessionKey) || "null");
    return typeof value?.accessToken === "string" ? value : null;
  } catch {
    return null;
  }
}
let sessionGeneration = 0;
// Read-only fence for multi-request page flows; token refresh preserves this scope.
export function getSessionGeneration() {
  return sessionGeneration;
}
export function saveSession(value: Session | null) {
  sessionGeneration++;
  persistSession(value);
}
function persistSession(value: Session | null) {
  if (value) sessionStorage.setItem(sessionKey, JSON.stringify(value));
  else {
    sessionStorage.removeItem(sessionKey);
    for (const key of Object.keys(sessionStorage))
      if (
        key.startsWith("customer-tdesign.") ||
        key.startsWith("lina-customer-tdesign")
      )
        sessionStorage.removeItem(key);
  }
  window.dispatchEvent(new Event("customer-session"));
}
export class ApiError extends Error {
  response: { status: number; data: unknown };
  constructor(message: string, status: number, data: unknown) {
    super(message);
    this.response = { status, data };
  }
}
export function safePath(path: string) {
  if (
    !path.startsWith("/") ||
    path.startsWith("//") ||
    /[\\\u0000-\u001f]/u.test(path)
  )
    return null;
  return path;
}
export function externalURL(value: string) {
  try {
    const u = new URL(value);
    return ["http:", "https:"].includes(u.protocol) &&
      !u.username &&
      !u.password
      ? u.href
      : null;
  } catch {
    return null;
  }
}
export function pluginApiPath(pluginId: string, path: string) {
  return `/x/${encodeURIComponent(pluginId)}/api/v1/${path.replace(/^\/+/, "")}`;
}
let refreshTask: {
  generation: number;
  token: string;
  promise: Promise<void>;
} | null = null;
async function refreshSession() {
  const current = getSession(),
    generation = sessionGeneration;
  if (!current?.refreshToken) throw new Error("请重新登录");
  if (
    refreshTask?.generation === generation &&
    refreshTask.token === current.accessToken
  )
    return refreshTask.promise;
  const task = {
    generation,
    token: current.accessToken,
    promise: (async () => {
      const response = await fetch("/api/v1/auth/refresh", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Accept-Language": "zh-CN",
        },
        body: JSON.stringify({ refreshToken: current.refreshToken }),
        signal: AbortSignal.timeout(30000),
      });
      const envelope = await response.json();
      if (!response.ok || envelope.code !== 0 || !envelope.data?.accessToken)
        throw new Error("登录已过期，请重新登录");
      // Refresh work belongs to the exact login generation that started it.
      if (
        sessionGeneration !== generation ||
        getSession()?.accessToken !== current.accessToken
      )
        throw new Error("登录状态已变更");
      persistSession({
        accessToken: envelope.data.accessToken,
        refreshToken: envelope.data.refreshToken || current.refreshToken,
      });
    })(),
  };
  refreshTask = task;
  try {
    await task.promise;
  } finally {
    if (refreshTask === task) refreshTask = null;
  }
}
async function request<T>(
  path: string,
  options: RequestOptions = {},
  replayed = false,
): Promise<T> {
  if (!safePath(path)) throw new Error("请求地址不可用");
  const url = new URL(
    path.startsWith("/x/") || path.startsWith("/__") ? path : `/api/v1${path}`,
    location.origin,
  );
  for (const [key, value] of Object.entries(options.params || {})) {
    if (value === undefined || value === null || value === "") continue;
    if (Array.isArray(value))
      value.forEach((item) =>
        url.searchParams.append(`${key}[]`, String(item)),
      );
    else url.searchParams.set(key, String(value));
  }
  const session = getSession(),
    generation = sessionGeneration;
  const headers: Record<string, string> = {
    "Accept-Language": "zh-CN",
    ...options.headers,
  };
  if (session) headers.Authorization = `Bearer ${session.accessToken}`;
  let body: BodyInit | undefined;
  if (options.data !== undefined) {
    if (options.data instanceof Blob || options.data instanceof FormData)
      body = options.data;
    else {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(options.data);
    }
  }
  const response = await fetch(url, {
    method: options.method || "GET",
    body,
    headers,
    credentials: "same-origin",
    cache: "no-store",
    signal: options.signal || AbortSignal.timeout(options.timeout || 30000),
  });
  const assertCurrentSession = () => {
    if (session && generation !== sessionGeneration)
      throw new ApiError("登录状态已变更，请重新打开页面", 401, {});
  };
  assertCurrentSession();
  if (response.status === 401 && session && !path.startsWith("/auth/")) {
    if (generation !== sessionGeneration)
      throw new ApiError("登录状态已变更，请重新打开页面", 401, {});
    if (
      !replayed &&
      session.refreshToken &&
      !path.endsWith("/reveal") &&
      !path.endsWith("/data-exports/proof") &&
      !path.endsWith("/personal-verifications")
    ) {
      try {
        if (getSession()?.accessToken === session.accessToken)
          await refreshSession();
        if (generation !== sessionGeneration) throw new Error("登录状态已变更");
      } catch {
        if (
          generation === sessionGeneration &&
          getSession()?.accessToken === session.accessToken
        )
          saveSession(null);
        throw new ApiError("登录已过期，请重新登录", 401, {});
      }
      return request<T>(path, options, true);
    }
    if (getSession()?.accessToken === session.accessToken) saveSession(null);
    throw new ApiError("登录已过期，请重新登录", 401, {});
  }
  if (options.responseType === "blob" && response.ok) {
    const data = await response.blob();
    assertCurrentSession();
    return data as T;
  }
  const envelope = await response.json().catch(() => null);
  assertCurrentSession();
  if (
    !response.ok ||
    (envelope === null && response.status !== 204) ||
    (envelope?.code !== undefined && envelope.code !== 0)
  ) {
    const explanation =
      response.status === 403
        ? "当前账号无法办理这项业务"
        : response.status === 404
          ? "该服务暂时不可用"
          : response.status === 409
            ? "内容已更新，请刷新后重试"
            : response.status === 429
              ? "操作较频繁，请稍后重试"
              : "操作未完成，请稍后重试";
    const message =
      typeof envelope?.message === "string" &&
      /[\u4e00-\u9fff]/u.test(envelope.message) &&
      !/[{}<>]/u.test(envelope.message)
        ? envelope.message
        : explanation;
    throw new ApiError(message, response.status, envelope);
  }
  return (envelope?.data ?? envelope) as T;
}
export const requestClient = {
  request,
  get: <T>(url: string, c: RequestOptions = {}) => request<T>(url, c),
  post: <T = unknown>(url: string, data?: unknown, c: RequestOptions = {}) =>
    request<T>(url, { ...c, method: "POST", data }),
  put: <T = unknown>(url: string, data?: unknown, c: RequestOptions = {}) =>
    request<T>(url, { ...c, method: "PUT", data }),
  delete: <T = unknown>(url: string, c: RequestOptions = {}) =>
    request<T>(url, { ...c, method: "DELETE" }),
  download: <T = Blob>(url: string, c: RequestOptions = {}) =>
    request<T>(url, { ...c, responseType: "blob" }),
};
export function actionKey(scope: string) {
  const key = `customer-tdesign.action.${scope}`;
  let value = sessionStorage.getItem(key);
  if (!value) {
    value = crypto.randomUUID();
    sessionStorage.setItem(key, value);
  }
  return value;
}
export function clearActionKey(scope: string) {
  sessionStorage.removeItem(`customer-tdesign.action.${scope}`);
}

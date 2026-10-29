import { t, locale } from "./i18n.js";
// Only published anonymous contracts are consumed; no tokens or private state.
export const catalogBase = "/x/idc-core/api/v1/public/catalog/products";
export const contentBase = "/x/idc-core/api/v1/public/content";

export async function request(path, options = {}) {
  const response = await fetch(path, {
    credentials: "omit",
    headers: { Accept: "application/json", "Accept-Language": locale },
    signal: AbortSignal.timeout(8000),
    ...options,
  });
  const payload = await response.json();
  if (!response.ok || String(payload.code ?? 0) !== "0" || !payload.data) {
    throw new Error(
      response.status === 404 || payload.errorCode === "CONTENT_NOTICE_NOT_FOUND"
        ? t("This content is unavailable or has been removed.")
        : t("Unable to load. Please retry."),
    );
  }
  return payload.data;
}

// Keep transfer errors on the article page and never navigate to an API response.
export async function requestDownload(token) {
  if (typeof token !== "string" || !/^[A-Za-z0-9_-]{1,128}$/u.test(token))
    throw new Error("Invalid download ticket");
  const controller = new AbortController();
  let timer;
  const resetIdle = () => {
    clearTimeout(timer);
    timer = setTimeout(
      () =>
        controller.abort(new DOMException("Download stalled", "TimeoutError")),
      30000,
    );
  };
  resetIdle();
  try {
    const response = await fetch(`${contentBase}/downloads/${token}`, {
      credentials: "omit",
      redirect: "error",
      signal: controller.signal,
    });
    resetIdle();
    const disposition = response.headers.get("content-disposition") || "";
    if (!response.ok || !/^attachment(?:;|$)/iu.test(disposition)) {
      await response.body?.cancel();
      throw new Error("Download unavailable");
    }
    let filename =
      disposition.match(/(?:^|;)\s*filename="([^"]*)"/iu)?.[1] ||
      disposition.match(/(?:^|;)\s*filename=([^;]+)/iu)?.[1] ||
      "download";
    const encoded = disposition.match(
      /(?:^|;)\s*filename\*=UTF-8''([^;]+)/iu,
    )?.[1];
    if (encoded) {
      try {
        filename = decodeURIComponent(encoded);
      } catch {
        /* Use the plain filename. */
      }
    }
    filename = filename
      .replace(/[<>:"/\\|?*\u0000-\u001f]/gu, "_")
      .trim()
      .slice(0, 255);
    const stream = response.body?.pipeThrough(
      new TransformStream({
        transform(chunk, output) {
          if (chunk.byteLength) resetIdle();
          output.enqueue(chunk);
        },
      }),
    );
    const blob = await new Response(stream || null, {
      headers: {
        "content-type":
          response.headers.get("content-type") || "application/octet-stream",
      },
    }).blob();
    return { blob, filename: filename || "download" };
  } finally {
    clearTimeout(timer);
    controller.abort();
  }
}

export function localized(value) {
  return typeof value === "string"
    ? value
    : (locale === "en-US"
        ? value?.enUS || value?.zhCN
        : value?.zhCN || value?.enUS) || "";
}

// Reject executable schemes, encoded slashes and cross-origin relative URLs.
export function safeURL(value, { external = false } = {}) {
  if (typeof value !== "string" || !value || /[\\\s\u0000-\u001f]/u.test(value))
    return "";
  if (/^\/(?!\/)/u.test(value) && !/%(?:2f|5c|0[0-9a-f])/iu.test(value))
    return value;
  if (!external) return "";
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) &&
      !url.username &&
      !url.password
      ? url.href
      : "";
  } catch {
    return "";
  }
}

export function customerURL(path) {
  if (!safeURL(path) || !path.startsWith("/")) return "";
  const configured = window.__LINA_TEMPLATE__?.customerUrl;
  const origin = safeURL(configured, { external: true });
  if (!origin || !/^https?:\/\//u.test(origin)) return "";
  return new URL(path, origin).href;
}

export function escapeHTML(value) {
  return String(value ?? "").replace(
    /[&<>"']/gu,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
}

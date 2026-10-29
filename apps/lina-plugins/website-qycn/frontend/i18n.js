// English source messages and a persisted, URL-addressable website locale.
import messages from "./locales/zh-CN.json" with { type: "json" };
export let locale = "zh-CN";
if (typeof window !== "undefined") {
  const selected = new URLSearchParams(location.search).get("lang");
  try {
    locale =
      selected || localStorage.getItem("website-qycn-language") || "zh-CN";
  } catch {
    locale = selected || "zh-CN";
  }
  if (locale !== "en-US") locale = "zh-CN";
}
export function t(source, language = locale) {
  return language === "en-US" ? source : messages[source] || source;
}
export function setLocale(value) {
  locale = value === "en-US" ? value : "zh-CN";
}
export function localizeData(value) {
  if (typeof value === "string") return t(value);
  if (Array.isArray(value)) return value.map(localizeData);
  if (value && typeof value === "object")
    return Object.fromEntries(
      Object.entries(value).map(([k, v]) => [k, localizeData(v)]),
    );
  return value;
}
// Used only for this template's static markup, before inserting remote data.
export function translateMarkup(html) {
  const translate = (value) =>
    value.replace(/^(\s*)(.*?)(\s*)$/su, (_, a, text, b) => a + t(text) + b);
  return html
    .split(/(<[^>]*>)/gu)
    .map((part) =>
      part.startsWith("<")
        ? part.replace(
            /(aria-label|placeholder|title)="([^"]*)"/gu,
            (_, key, value) => `${key}="${translate(value)}"`,
          )
        : translate(part),
    )
    .join("");
}

// This file resolves public API requests for browser and Nuxt SSR execution.
export function resolvePublicApiURL(path: string) {
  if (import.meta.client) return path;
  const config = useRuntimeConfig();
  return new URL(path, `${config.apiOrigin}/`).toString();
}

export function publicRequestHeaders() {
  if (import.meta.client) return {};
  return useRequestHeaders(['accept-language', 'cookie']);
}

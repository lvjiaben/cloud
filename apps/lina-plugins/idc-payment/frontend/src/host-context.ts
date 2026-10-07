export type PluginHostMountContext = {
  accessToken: string;
  assetURL: string;
  baseURL: string;
  container: HTMLElement;
  locale: string;
  messages: Record<string, unknown>;
  query: Record<string, string>;
  route: { fullPath?: string; name?: unknown; path?: string };
  routePath: string;
  t: (key: string, fallback?: string) => string;
  title: string;
};
export type PluginHostMountInstance = { unmount: () => void; update: (context: PluginHostMountContext) => void };

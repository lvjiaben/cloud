import type { PluginHostMountContext } from '../host-context';

export type LocalMessages = Record<string, { en: string; zh: string }>;

function render(template: string, values: Record<string, number | string>) {
  return template.replaceAll(/\{([A-Za-z0-9_]+)\}/gu, (_, key: string) =>
    key in values ? String(values[key]) : `{${key}}`,
  );
}

export function createTranslator(
  currentContext: () => PluginHostMountContext,
  messages: LocalMessages,
) {
  return (key: string, values: Record<string, number | string> = {}) => {
    const context = currentContext();
    const local = messages[key];
    const fallback = local
      ? context.locale.toLowerCase().startsWith('zh')
        ? local.zh
        : local.en
      : key;
    return render(context.t(key, fallback), values);
  };
}

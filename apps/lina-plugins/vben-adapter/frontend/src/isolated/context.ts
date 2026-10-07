import type { App } from 'vue';
import { i18n } from '@vben/locales';
import { useSimpleLocale } from '@vben-core/composables';
import { updatePreferences } from '@vben/preferences';
import messages from 'virtual:vben-isolated-messages';

/** Only this bundle's Vue, preferences and locale state are changed. */
export function setIsolatedVbenContext(locale: string) {
  const normalized = locale.toLowerCase().startsWith('en') ? 'en-US' : 'zh-CN';
  updatePreferences({ app: { locale: normalized } });
  i18n.global.setLocaleMessage(normalized, messages[normalized]);
  i18n.global.locale.value = normalized;
  const simple = useSimpleLocale();
  simple.setMessages(normalized, messages[normalized].common);
  simple.setSimpleLocale(normalized);
}

/** No document-wide preferences, host storage, token or locale initialization. */
export async function initializeIsolatedVben(app: App, locale: string) {
  app.use(i18n);
  setIsolatedVbenContext(locale);
}

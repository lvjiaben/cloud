import { preferences } from "@vben/preferences";
import { $t, $te } from "@vben/locales";
import enMessages from "../../manifest/i18n/en-US/plugin.json";
import zhMessages from "../../manifest/i18n/zh-CN/plugin.json";

// The plugin-owned bundle also covers the interval before runtime catalogs refresh.
export function channelText(key: string) {
  const messageKey = "plugin.idc-channel.workspace." + key;
  if ($te(messageKey)) return $t(messageKey);
  const messages = preferences.app.locale.toLowerCase().startsWith("zh")
    ? zhMessages
    : enMessages;
  return messages[messageKey as keyof typeof messages] ?? messageKey;
}

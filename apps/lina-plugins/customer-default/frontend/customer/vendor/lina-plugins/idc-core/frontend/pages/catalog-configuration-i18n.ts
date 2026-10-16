import { preferences } from "@vben/preferences";

const messages = {
  configured: { enUs: "Configured", zhCn: "已配置" },
  empty: { enUs: "Not configured", zhCn: "未配置" },
  filePending: {
    enUs: "The governed upload service is not available yet.",
    zhCn: "受治理的上传服务暂不可用。",
  },
  configuredFile: { enUs: "Configured file", zhCn: "已配置文件" },
  fileUpload: {
    enUs: "Click or drag a file here to upload",
    zhCn: "点击或拖拽文件到这里上传",
  },
  filePolicy: {
    enUs: "Up to {count} files; accepted: {extensions}",
    zhCn: "最多 {count} 个文件，支持 {extensions}",
  },
  fileDetach: {
    enUs: "Remove from this configuration",
    zhCn: "从当前配置移除",
  },
  fileRemoveFailed: {
    enUs: "The file could not be removed. Please try again.",
    zhCn: "文件暂时无法移除，请重试。",
  },
  fileRemoved: { enUs: "File removed", zhCn: "文件已移除" },
  fileReferenceRemoved: {
    enUs: "Removed from this configuration. Historical records are unchanged.",
    zhCn: "已从当前配置移除，历史记录不受影响。",
  },
  fileRevoke: { enUs: "Cancel upload", zhCn: "撤销上传" },
  fileUploadFailed: {
    enUs: "The file could not be uploaded. Please check it and try again.",
    zhCn: "文件上传失败，请检查后重试。",
  },
  fileUploaded: { enUs: "File passed inspection", zhCn: "文件检查通过" },
  dynamicCandidatesPending: {
    enUs: "Options cannot be loaded because the governed candidate service is unavailable.",
    zhCn: "受治理的候选项服务暂不可用，当前无法加载选项。",
  },
  evaluationFailed: {
    enUs: "The latest configuration could not be checked. Saving is disabled until the check succeeds.",
    zhCn: "暂时无法核对最新配置，核对成功前不能保存。",
  },
  excludes: {
    enUs: "This selection conflicts with another selection.",
    zhCn: "当前选择与其他选择冲突。",
  },
  integer: {
    enUs: "Enter a 64-bit integer.",
    zhCn: "请输入 64 位整数。",
  },
  decimal: { enUs: "Enter a valid decimal.", zhCn: "请输入有效的小数。" },
  maximum: { enUs: "The value is too large.", zhCn: "填写的数值过大。" },
  maximumLength: {
    enUs: "The content is too long.",
    zhCn: "填写的内容过长。",
  },
  minimum: { enUs: "The value is too small.", zhCn: "填写的数值过小。" },
  minimumLength: {
    enUs: "The content is too short.",
    zhCn: "填写的内容过短。",
  },
  invalid: { enUs: "Please check this value.", zhCn: "请检查此项内容。" },
  required: { enUs: "This field is required.", zhCn: "此项为必填项。" },
  requires: {
    enUs: "Complete the required related selection.",
    zhCn: "请同时完成所需的关联选择。",
  },
  secretHint: {
    enUs: "Leave blank to keep an existing credential. The server still checks whether a credential is required.",
    zhCn: "留空表示不修改已有凭据；服务器仍会检查此凭据是否必须配置。",
  },
} as const;

export type CatalogConfigurationMessageKey = keyof typeof messages;

export function catalogConfigurationMessage(
  key: CatalogConfigurationMessageKey,
  variables?: Record<string, number | string>,
) {
  const message = messages[key];
  let result: string =
    preferences.app.locale === "zh-CN" ? message.zhCn : message.enUs;
  for (const [name, value] of Object.entries(variables ?? {}))
    result = result.replaceAll(`{${name}}`, String(value));
  return result;
}

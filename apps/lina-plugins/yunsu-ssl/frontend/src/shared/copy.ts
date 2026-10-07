import type { CertificateAction, Locale } from "./model.js";

const statuses: Record<string, Record<Locale, string>> = {
  awaiting_validation: { "en-US": "Awaiting validation", "zh-CN": "等待验证" },
  cancelled: { "en-US": "Cancelled", "zh-CN": "已取消" },
  expired: { "en-US": "Expired", "zh-CN": "已过期" },
  issued: { "en-US": "Issued", "zh-CN": "已签发" },
  pending: { "en-US": "Processing", "zh-CN": "处理中" },
  rejected: { "en-US": "Not issued", "zh-CN": "未签发" },
  validating: { "en-US": "Validating", "zh-CN": "验证中" },
};

export const copy = {
  "en-US": {
    actionFailed: "The request could not be completed. Existing certificate data is unchanged.", actions: "Actions", adminTitle: "SSL certificate management",
    cancel: "Cancel application", cancelConfirm: "Cancel this certificate application before issuance?", close: "Close", customer: "Customer", customerTitle: "SSL certificates",
    detail: "Certificate details", domain: "Domain", download: "Download certificate", empty: "No certificates", emptyHint: "Delivered SSL certificates will appear here.", error: "Certificates could not be loaded", errorHint: "Please try again. Existing certificate data is unchanged.",
    expiresAt: "Expires", fileName: "File name", host: "Host / record", issuedAt: "Issued", loadMore: "Load more", method: "Validation method",
    methodDns: "DNS validation", methodFile: "File validation", path: "File path", product: "Product", refresh: "Refresh", retry: "Try again",
    revalidate: "Revalidate", save: "Save", status: "Status", sync: "Refresh status", updatedAt: "Updated", updateValidation: "Change validation method",
    value: "Value / file content", validation: "Validation instructions", view: "View details", working: "Processing…",
  },
  "zh-CN": {
    actionFailed: "操作暂时没有完成，已有证书数据不会受到影响。", actions: "操作", adminTitle: "SSL 证书管理",
    cancel: "取消申请", cancelConfirm: "确认在签发前取消这张证书的申请吗？", close: "关闭", customer: "客户", customerTitle: "SSL 证书",
    detail: "证书详情", domain: "域名", download: "下载证书", empty: "暂无证书", emptyHint: "已交付的 SSL 证书会显示在这里。", error: "暂时无法读取证书", errorHint: "请稍后重试，已有证书数据不会受到影响。",
    expiresAt: "到期时间", fileName: "文件名", host: "主机记录", issuedAt: "签发时间", loadMore: "加载更多", method: "验证方式",
    methodDns: "DNS 验证", methodFile: "文件验证", path: "文件路径", product: "商品", refresh: "刷新", retry: "重试",
    revalidate: "重新验证", save: "保存", status: "状态", sync: "同步状态", updatedAt: "更新时间", updateValidation: "切换验证方式",
    value: "记录值或文件内容", validation: "验证说明", view: "查看详情", working: "处理中…",
  },
} as const;

export function statusLabel(status: string, locale: Locale) {
  return statuses[status]?.[locale] ?? (locale === "en-US" ? "Status unavailable" : "状态暂不可用");
}

export function actionLabel(action: CertificateAction, locale: Locale) {
  const words = copy[locale];
  return {
    cancel: words.cancel,
    download: words.download,
    revalidate: words.revalidate,
    sync: words.sync,
    "update-validation": words.updateValidation,
  }[action];
}

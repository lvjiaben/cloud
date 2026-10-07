export type Locale = 'en-US' | 'zh-CN';

export interface ServerItem {
  serverNo: string;
  boundAt: number;
  channelName: string;
  kind: string;
  latestOperation: string;
  poolName: string;
  productNameEnUs: string;
  productNameZhCn: string;
  state: string;
  updatedAt: number;
}

export interface ServerPage { items: ServerItem[]; nextCursor: string; }

export const copy = {
  'en-US': {
    close: 'Close', detailTitle: 'Server details', emptyDescription: 'Servers delivered to this account will appear here.', emptyTitle: 'No servers yet',
    errorDescription: 'The server directory could not be loaded. Your existing data is unchanged.', errorTitle: 'Unable to load servers', loadMore: 'Load more',
    location: 'Channel / resource pool', operation: 'Latest activity', product: 'Product', refresh: 'Refresh', retry: 'Try again', state: 'Status',
    summary: (count: number) => `${count} server${count === 1 ? '' : 's'}`, updatedAt: 'Updated', view: 'View details', boundAt: 'Delivered', kind: 'Type',
  },
  'zh-CN': {
    close: '关闭', detailTitle: '服务器详情', emptyDescription: '当前账号交付的服务器会显示在这里。', emptyTitle: '暂无服务器',
    errorDescription: '暂时无法读取服务器目录，已有数据不会受到影响。', errorTitle: '服务器加载失败', loadMore: '加载更多',
    location: '渠道 / 资源池', operation: '最近活动', product: '商品', refresh: '刷新', retry: '重试', state: '状态',
    summary: (count: number) => `共 ${count} 台服务器`, updatedAt: '更新时间', view: '查看详情', boundAt: '交付时间', kind: '类型',
  },
} as const;

const labels: Record<string, Record<Locale, string>> = {
  attention_required: { 'en-US': 'Needs attention', 'zh-CN': '需要处理' }, bare_metal_server: { 'en-US': 'Bare metal server', 'zh-CN': '裸金属服务器' },
  cloud_server: { 'en-US': 'Cloud server', 'zh-CN': '云服务器' }, completed: { 'en-US': 'Completed', 'zh-CN': '已完成' },
  idle: { 'en-US': 'No pending activity', 'zh-CN': '暂无待处理活动' }, other_server: { 'en-US': 'Server', 'zh-CN': '服务器' },
  preparing: { 'en-US': 'Preparing', 'zh-CN': '准备中' }, ready: { 'en-US': 'Available', 'zh-CN': '可用' },
  released: { 'en-US': 'Released', 'zh-CN': '已释放' }, releasing: { 'en-US': 'Releasing', 'zh-CN': '释放中' },
  unsuccessful: { 'en-US': 'Unsuccessful', 'zh-CN': '未成功' }, unknown: { 'en-US': 'Status unavailable', 'zh-CN': '状态暂不可用' },
  working: { 'en-US': 'In progress', 'zh-CN': '处理中' },
};

export function normalizeLocale(value: unknown): Locale { return value === 'en-US' ? 'en-US' : 'zh-CN'; }
export function displayName(item: ServerItem, locale: Locale): string { return locale === 'en-US' ? item.productNameEnUs : item.productNameZhCn; }
export function businessLabel(value: string, locale: Locale): string { return labels[value]?.[locale] ?? labels.unknown[locale]; }
export function formatTime(value: number, locale: Locale): string {
  if (!Number.isFinite(value) || value <= 0) return '—';
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}
export function validServerItem(value: unknown): value is ServerItem {
  if (!value || typeof value !== 'object') return false;
  const item = value as Partial<ServerItem>;
  return /^SVR-[0-9A-F]{32}$/.test(String(item.serverNo ?? '')) &&
    [item.productNameEnUs, item.productNameZhCn, item.channelName, item.poolName, item.kind, item.state, item.latestOperation].every((field) => typeof field === 'string' && field.trim().length > 0) &&
    Number.isFinite(item.boundAt) && Number.isFinite(item.updatedAt);
}
export function parseServerPage(value: unknown): ServerPage | null {
  if (!value || typeof value !== 'object') return null;
  const page = value as Partial<ServerPage>;
  if (!Array.isArray(page.items) || page.items.length > 50 || !page.items.every(validServerItem)) return null;
  if (typeof page.nextCursor !== 'string' || page.nextCursor.length > 128) return null;
  return { items: page.items, nextCursor: page.nextCursor };
}
export function parseServerDetail(value: unknown): ServerItem | null {
  if (!value || typeof value !== 'object') return null;
  return validServerItem((value as { item?: unknown }).item) ? (value as { item: ServerItem }).item : null;
}

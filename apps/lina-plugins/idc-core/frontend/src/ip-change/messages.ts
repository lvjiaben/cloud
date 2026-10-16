import type { LocalMessages } from '../runtime/i18n';

export const ipChangeMessages: LocalMessages = {
  'ipChange.action.publish': { en: 'Save settings', zh: '保存设置' },
  'ipChange.action.refresh': { en: 'Refresh', zh: '刷新' },
  'ipChange.amount': { en: 'Price after free allowance (minor units)', zh: '超额含税费用（最小单位，如分）' },
  'ipChange.currency': { en: 'Currency', zh: '币种' },
  'ipChange.description': { en: 'Set one local, versioned rule per currency. Customers never choose an address or resource pool.', zh: '按币种设置本地规则并保留历史版本。客户不能自行选择 IP、资源池或服务商参数。' },
  'ipChange.enabled': { en: 'Allow customer IP changes', zh: '允许客户申请更换 IP' },
  'ipChange.error.load': { en: 'The IP-change policies could not be loaded.', zh: '无法读取换 IP 设置，请检查权限后重试。' },
  'ipChange.error.save': { en: 'The policy changed or could not be published. Refresh and try again.', zh: '设置已变化或暂时无法发布，请刷新后重试。' },
  'ipChange.free': { en: 'Free changes in rolling window', zh: '这段时间内免费次数' },
  'ipChange.loading': { en: 'Loading policies…', zh: '正在读取设置…' },
  'ipChange.maxDaily': { en: 'Maximum changes per day', zh: '每天最多更换次数' },
  'ipChange.snapshotAge': { en: 'Maximum inventory age (seconds)', zh: '资源库存最长有效时间（秒）' },
  'ipChange.status.disabled': { en: 'Disabled', zh: '已关闭' },
  'ipChange.status.enabled': { en: 'Enabled', zh: '已开启' },
  'ipChange.saved': { en: 'Settings saved.', zh: '设置已保存。' },
  'ipChange.title': { en: 'Server IP-change policy', zh: '服务器换 IP 设置' },
  'ipChange.version': { en: 'Current version', zh: '当前版本' },
  'ipChange.window': { en: 'Rolling window (days)', zh: '按最近多少天计算' },
};

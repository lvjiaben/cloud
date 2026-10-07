import type { LocalMessages } from '../runtime/i18n';

export const serverOperationMessages: LocalMessages = {
  'serverOperation.action.cancel': { en: 'Cancel', zh: '取消' },
  'serverOperation.action.confirmDisable': {
    en: 'Disable protection',
    zh: '确认关闭保护',
  },
  'serverOperation.action.refresh': { en: 'Refresh', zh: '刷新' },
  'serverOperation.action.save': { en: 'Save policy', zh: '保存设置' },
  'serverOperation.description': {
    en: 'Control whether customers must confirm their current login again before dangerous server actions.',
    zh: '控制客户执行危险服务器操作前，是否需要再次确认当前登录身份。',
  },
  'serverOperation.disableDescription': {
    en: 'Customers will still see the normal second confirmation, but they will no longer be asked to confirm the current login again. This affects password reset, reinstall, resize, IP change, and forced stop or reboot.',
    zh: '关闭后，客户仍要进行普通二次确认，但不再需要再次确认当前登录身份。此变化影响重置密码、重装、变配、换 IP、强制关机和强制重启。',
  },
  'serverOperation.disableTitle': {
    en: 'Confirm reduced login protection',
    zh: '确认降低登录保护',
  },
  'serverOperation.enabledDescription': {
    en: 'Recommended. Dangerous actions require an exact, short-lived confirmation bound to that action.',
    zh: '推荐开启。危险操作必须取得与本次操作完全对应、短时间有效的身份确认。',
  },
  'serverOperation.error.load': {
    en: 'The security policy could not be loaded. Check your permission and try again.',
    zh: '无法读取安全设置，请检查权限后重试。',
  },
  'serverOperation.error.save': {
    en: 'The policy changed or could not be saved. Reload it before trying again.',
    zh: '设置已被他人修改或暂时无法保存，请刷新后重试。',
  },
  'serverOperation.eyebrow': { en: 'Server safety', zh: '服务器安全' },
  'serverOperation.lastUpdated': { en: 'Last updated', zh: '最后更新' },
  'serverOperation.loading': { en: 'Loading policy…', zh: '正在读取设置…' },
  'serverOperation.policyLabel': {
    en: 'Require login confirmation for dangerous actions',
    zh: '危险操作需要再次确认登录身份',
  },
  'serverOperation.saved': { en: 'Policy saved', zh: '设置已保存' },
  'serverOperation.status.enabled': { en: 'Protected', zh: '已开启保护' },
  'serverOperation.status.disabled': { en: 'Protection reduced', zh: '已降低保护' },
  'serverOperation.title': { en: 'Server operation security', zh: '服务器操作安全' },
  'serverOperation.transfer.description': {
    en: 'When enabled, the current owner enters the recipient registered email and the recipient must confirm in the customer center within 24 hours. No separate staff approval is required.',
    zh: '开启后，当前客户填写接收方注册邮箱，接收方须在 24 小时内到客户中心确认，不再逐单等待后台审批。',
  },
  'serverOperation.transfer.disabled': { en: 'Transfers closed', zh: '已关闭转移' },
  'serverOperation.transfer.enabled': { en: 'Transfers open', zh: '已开放转移' },
  'serverOperation.transfer.label': { en: 'Allow customer service transfers', zh: '允许客户转移服务器' },
};

import { defineOverridesPreferences } from '@vben/preferences';

/**
 * @description 项目配置文件
 * 只需要覆盖项目中的一部分配置，不需要的配置不用覆盖，会自动使用默认配置
 * !!! 更改配置后请清空缓存，否则可能不生效
 */
export const overridesPreferences = defineOverridesPreferences({
  // overrides
  app: {
    accessMode: 'frontend',
    authPageLayout: 'panel-center',
    contentPadding: 16,
    defaultHomePath: '/account',
    enableRefreshToken: true,
    name: import.meta.env.VITE_APP_TITLE,
    preferencesButtonPosition: 'header',
  },
  copyright: {
    companyName: 'LinaPro.AI',
    companySiteLink: 'https://linapro.ai',
    date: String(new Date().getFullYear()),
    enable: true,
    settingShow: false,
  },
  footer: {
    enable: false,
    settingShow: false,
  },
  header: {
    height: 52,
  },
  navigation: {
    accordion: false,
    split: false,
    styleType: 'plain',
  },
  sidebar: {
    collapsed: false,
    collapsedButton: false,
    draggable: false,
    // Vben uses this flag as the sidebar's pinned state. Keep it enabled for
    // the customer desktop shell so pointer enter/leave never shrinks the
    // layout spacer back to the collapsed width.
    expandOnHover: true,
    fixedButton: false,
    width: 204,
  },
  tabbar: {
    height: 36,
    styleType: 'plain',
  },
  theme: {
    colorPrimary: '#1677ff',
    fontSize: 14,
    mode: 'light',
    radius: '0',
    semiDarkHeader: false,
    semiDarkSidebar: false,
    semiDarkSidebarSub: false,
  },
  widget: {
    sidebarToggle: false,
  },
});

const { mode: _defaultThemeMode, ...enforcedThemePreferences } =
  overridesPreferences.theme ?? {};

/**
 * 运行时只强制产品视觉规范，保留用户主动选择的明暗主题。
 */
export const enforcedVisualPreferences = {
  ...overridesPreferences,
  theme: enforcedThemePreferences,
};

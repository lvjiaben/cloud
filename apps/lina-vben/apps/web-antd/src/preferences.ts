import { defineOverridesPreferences } from '@vben/preferences';

/**
 * @description 项目配置文件
 * 只需要覆盖项目中的一部分配置，不需要的配置不用覆盖，会自动使用默认配置
 * !!! 更改配置后请清空缓存，否则可能不生效
 */
export const overridesPreferences = defineOverridesPreferences({
  // overrides
  app: {
    accessMode: 'backend',
    authPageLayout: 'panel-center',
    enableRefreshToken: true,
    name: import.meta.env.VITE_APP_TITLE,
  },
  copyright: {
    companyName: 'LinaPro.AI',
    companySiteLink: 'https://linapro.ai',
    date: String(new Date().getFullYear()),
    enable: true,
    settingShow: false,
  },
  footer: {
    enable: true,
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
    draggable: false,
    width: 232,
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

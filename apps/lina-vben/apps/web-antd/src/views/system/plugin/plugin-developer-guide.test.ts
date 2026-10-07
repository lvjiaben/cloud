import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { cwd } from 'node:process';

import { describe, expect, it } from 'vitest';

import enUSPages from '#/locales/langs/en-US/pages.json';
import zhCNPages from '#/locales/langs/zh-CN/pages.json';

import {
  pluginDeveloperGuideExampleSections,
  pluginDeveloperGuideSectionKeys,
} from './plugin-developer-guide';

type GuideLocale = {
  system: {
    plugin: {
      guide: {
        sections: Record<
          string,
          { body: string; example?: string; title: string }
        >;
      };
    };
  };
};

describe('plugin developer guide', () => {
  it('keeps every required topic in both locales', () => {
    const requiredSections = [
      'dimensions',
      'extensions',
      'surfaces',
      'pageBridge',
      'interfaces',
      'fulfillment',
      'secrets',
      'testing',
      'package',
      'lifecycle',
    ];
    expect(pluginDeveloperGuideSectionKeys).toEqual(
      expect.arrayContaining(requiredSections),
    );

    for (const locale of [enUSPages, zhCNPages] as GuideLocale[]) {
      const sections = locale.system.plugin.guide.sections;
      for (const key of pluginDeveloperGuideSectionKeys) {
        expect(sections[key]?.title.trim()).toBeTruthy();
        expect(sections[key]?.body.trim()).toBeTruthy();
        if (pluginDeveloperGuideExampleSections.has(key)) {
          expect(sections[key]?.example?.trim()).toBeTruthy();
        }
      }
    }
  });

  it('is opened from the existing plugin-management page without a new route', () => {
    const source = readFileSync(
      resolve(cwd(), 'apps/web-antd/src/views/system/plugin/index.vue'),
      'utf8',
    );
    expect(source).toContain('plugin-developer-guide-drawer.vue');
    expect(source).toContain('plugin-developer-guide-trigger');
    expect(source).not.toContain("path: '/plugin-developer-guide'");
  });
});

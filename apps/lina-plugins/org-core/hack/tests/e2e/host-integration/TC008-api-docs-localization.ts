import type { APIRequestContext } from '@host-tests/support/playwright';

import { test, expect } from '@host-tests/fixtures/auth';
import {
  createAdminApiContext,
  enablePlugin,
  getPlugin,
  installPlugin,
  syncPlugins,
} from '@host-tests/support/api/job';

const pluginID = 'org-core';
const localizedPathPrefix = `/x/${pluginID}/`;
const openApiMethods = new Set([
  'connect',
  'delete',
  'get',
  'head',
  'options',
  'patch',
  'post',
  'put',
  'trace',
]);

type OpenApiDocument = {
  paths?: Record<string, Record<string, OpenApiOperation | unknown>>;
};

type OpenApiOperation = {
  summary?: unknown;
  tags?: unknown;
};

type OpenApiTitle = {
  method: string;
  path: string;
  kind: 'summary' | 'tag';
  value: string;
};

async function ensureOrgPluginEnabled(api: APIRequestContext) {
  await syncPlugins(api);
  let plugin = await getPlugin(api, pluginID);
  if (plugin.installed !== 1) {
    await installPlugin(api, pluginID);
    plugin = await getPlugin(api, pluginID);
  }
  if (plugin.enabled !== 1) {
    await enablePlugin(api, pluginID);
  }
}

test.describe('TC008 组织插件接口文档本地化', () => {
  let adminApi: APIRequestContext;

  test.beforeAll(async () => {
    adminApi = await createAdminApiContext();
    await ensureOrgPluginEnabled(adminApi);
  });

  test.afterAll(async () => {
    await adminApi.dispose();
  });

  test('TC-8a: 中文 API 文档中组织插件接口标题不回退英文', async ({
    adminPage,
  }) => {
    const response = await adminPage.request.get('/api.json?lang=zh-CN', {
      headers: { 'Accept-Language': 'zh-CN' },
    });
    expect(response.ok()).toBeTruthy();
    const apiDocument = (await response.json()) as OpenApiDocument;

    const englishFallbacks = collectUnlocalizedChineseOperationTitles(
      apiDocument,
    );
    expect(
      englishFallbacks,
      `组织插件中文接口文档仍存在英文标题：\n${formatOpenApiTitles(englishFallbacks)}`,
    ).toHaveLength(0);
    expectOperationTitle(apiDocument, 'GET', '/x/org-core/api/v1/dept', {
      tag: '部门管理',
      summary: '获取部门列表',
    });
    expectOperationTitle(apiDocument, 'GET', '/x/org-core/api/v1/post', {
      tag: '岗位管理',
      summary: '获取岗位列表',
    });
  });
});

function collectUnlocalizedChineseOperationTitles(apiDocument: OpenApiDocument) {
  const titles: OpenApiTitle[] = [];
  for (const [path, pathItem] of Object.entries(apiDocument.paths ?? {})) {
    if (!path.startsWith(localizedPathPrefix)) {
      continue;
    }
    for (const [method, operation] of Object.entries(pathItem)) {
      if (!openApiMethods.has(method.toLowerCase())) {
        continue;
      }
      const typedOperation = operation as OpenApiOperation;
      const summary = stringValue(typedOperation.summary);
      if (summary !== '' && !containsCjk(summary)) {
        titles.push({
          method: method.toUpperCase(),
          path,
          kind: 'summary',
          value: summary,
        });
      }
      for (const tag of arrayStringValues(typedOperation.tags)) {
        if (tag !== '' && !containsCjk(tag)) {
          titles.push({
            method: method.toUpperCase(),
            path,
            kind: 'tag',
            value: tag,
          });
        }
      }
    }
  }
  return titles;
}

function expectOperationTitle(
  apiDocument: OpenApiDocument,
  method: string,
  path: string,
  expected: { tag: string; summary: string },
) {
  const operation = apiDocument.paths?.[path]?.[
    method.toLowerCase()
  ] as OpenApiOperation | undefined;
  expect(operation, `缺少接口定义：${method} ${path}`).toBeTruthy();
  expect(arrayStringValues(operation?.tags)[0]).toBe(expected.tag);
  expect(stringValue(operation?.summary)).toBe(expected.summary);
}

function stringValue(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}

function arrayStringValues(value: unknown) {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string')
    : [];
}

function containsCjk(value: string) {
  return /[\u3400-\u9fff]/u.test(value);
}

function formatOpenApiTitles(titles: OpenApiTitle[]) {
  return titles
    .slice(0, 50)
    .map((item) => `${item.method} ${item.path} ${item.kind}: ${item.value}`)
    .join('\n');
}


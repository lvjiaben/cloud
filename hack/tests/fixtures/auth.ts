import { test as base, type BrowserContext, type Page } from '@playwright/test';

import { adminStorageStatePath } from './auth-state';
import { LoginPage } from '../pages/LoginPage';
import { MainLayout } from '../pages/MainLayout';
import { config, isWorkspaceManagedPath, workspacePath } from './config';
import { waitForRouteReady } from '../support/ui';

export type AuthFixtures = {
  adminContext: BrowserContext;
  adminPage: Page;
  authenticatedPage: Page;
  loginPage: LoginPage;
  mainLayout: MainLayout;
};

const workspaceGotoInstalled = Symbol('workspaceGotoInstalled');

type WorkspaceAwarePage = Page & {
  [workspaceGotoInstalled]?: boolean;
};

function shouldPrefixWorkspacePath(url: Parameters<Page['goto']>[0]) {
  if (typeof url !== 'string' || !url.startsWith('/')) {
    return false;
  }
  return isWorkspaceManagedPath(url);
}

async function installWorkspaceGoto(page: Page) {
  const workspaceAwarePage = page as WorkspaceAwarePage;
  if (workspaceAwarePage[workspaceGotoInstalled]) {
    return;
  }
  const originalGoto = workspaceAwarePage.goto.bind(page);
  workspaceAwarePage.goto = ((url, options) => {
    const nextURL = shouldPrefixWorkspacePath(url)
      ? workspacePath(url as string)
      : url;
    return originalGoto(nextURL, options);
  }) as Page['goto'];
  workspaceAwarePage[workspaceGotoInstalled] = true;
}

async function installSourcePluginRoutes(page: Page) {
  const paths = (process.env.E2E_SOURCE_PLUGIN_ROUTES ?? '')
    .split(',')
    .map((path) => path.trim())
    .filter((path) => path.startsWith('/'));
  if (paths.length === 0) {
    return;
  }

  await page.route('**/api/v1/menus/all', async (route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      code?: number;
      data?: { list?: Array<Record<string, unknown>> };
      message?: string;
    };
    const list = Array.isArray(payload.data?.list) ? payload.data.list : [];
    const sourcePluginRoutes = paths.map((path, index) => ({
      component: '#/views/system/plugin/dynamic-page',
      id: -10_000 - index,
      meta: {
        order: 10_000 + index,
        title: `Source plugin E2E ${index + 1}`,
      },
      name: `SourcePluginE2E${index + 1}`,
      parentId: 0,
      path,
    }));
    await route.fulfill({
      response,
      json: {
        ...payload,
        data: { ...payload.data, list: [...list, ...sourcePluginRoutes] },
      },
    });
  });
}

export const test = base.extend<AuthFixtures>({
  page: async ({ page }, use) => {
    await installWorkspaceGoto(page);
    await use(page);
  },
  adminContext: async ({ browser }, use) => {
    const context = await browser.newContext({
      baseURL: config.baseURL,
      storageState: adminStorageStatePath,
    });
    await use(context);
    await context.close();
  },
  authenticatedPage: async ({ adminContext }, use) => {
    const page = await adminContext.newPage();
    await installWorkspaceGoto(page);
    await use(page);
    await page.close();
  },
  adminPage: async ({ adminContext }, use) => {
    const page = await adminContext.newPage();
    await installWorkspaceGoto(page);
    await installSourcePluginRoutes(page);
    await page.goto(workspacePath('/dashboard/analytics'), { waitUntil: 'domcontentloaded' });
    await waitForRouteReady(page, 15000);
    await use(page);
    await page.close();
  },
  loginPage: async ({ page }, use) => {
    await installWorkspaceGoto(page);
    await use(new LoginPage(page));
  },
  mainLayout: async ({ adminPage }, use) => {
    await use(new MainLayout(adminPage));
  },
});

export { expect } from '@playwright/test';
export type { Locator, Page, Route } from '@playwright/test';

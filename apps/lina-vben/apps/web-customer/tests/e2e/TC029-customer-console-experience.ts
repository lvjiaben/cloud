import { expect, test } from '../fixtures/customer';
import { mockCustomerAPIs } from '../support/customer-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

async function loginCustomer(page: import('@playwright/test').Page) {
  await page.goto('/auth/login');
  await page.locator('input').nth(0).fill('customer');
  await page.locator('input').nth(1).fill('customer-password');
  await page.getByRole('button', { name: 'login' }).click();
  await page.waitForURL(/\/account(?:$|\/)/u);
}

test.describe('TC029 客户中心企业云控制台体验', () => {
  test('TC029a: 中文桌面端使用方角控制台风格并提供真实常用入口', async ({
    page,
  }) => {
    await mockCustomerAPIs(page);
    await loginCustomer(page);

    await expect(page.getByText('常用入口', { exact: true })).toBeVisible();
    await expect(page.getByText('账户余额', { exact: true })).toBeVisible();
    await expect(page.getByText('¥128.50', { exact: true })).toBeVisible();
    await expect(page.getByText('产品选购', { exact: true })).toBeVisible();
    await expect(
      page.getByText('技术支持', { exact: true }).first(),
    ).toBeVisible();
    await expect(page.locator('body')).not.toContainText(
      '业务模块将按阶段接入',
    );

    const visualContract = await page.evaluate(() => {
      const card = document.querySelector<HTMLElement>('.ant-card');
      const cardStyle = card ? getComputedStyle(card) : null;
      const submenuItems = [
        ...document.querySelectorAll<HTMLElement>('li.vben-sub-menu'),
      ];
      return {
        bodyClass: document.body.classList.contains('customer-console-app'),
        cardRadius: cardStyle?.borderRadius,
        cardShadow: cardStyle?.boxShadow,
        dataset: document.documentElement.dataset.customerConsole,
        headerToken: getComputedStyle(document.documentElement)
          .getPropertyValue('--header')
          .trim(),
        menuToken: getComputedStyle(document.documentElement)
          .getPropertyValue('--menu')
          .trim(),
        openSubmenus: submenuItems.filter((item) =>
          item.classList.contains('is-opened'),
        ).length,
        rootRadius: getComputedStyle(document.documentElement)
          .getPropertyValue('--radius')
          .trim(),
        sidebarToken: getComputedStyle(document.documentElement)
          .getPropertyValue('--sidebar')
          .trim(),
        submenus: submenuItems.length,
      };
    });
    expect(visualContract).toEqual({
      bodyClass: true,
      cardRadius: '0px',
      cardShadow: 'none',
      dataset: 'true',
      headerToken: '0 0% 100%',
      menuToken: '0 0% 100%',
      openSubmenus: visualContract.submenus,
      rootRadius: '0rem',
      sidebarToken: '0 0% 100%',
      submenus: visualContract.submenus,
    });
    expect(visualContract.submenus).toBeGreaterThan(3);
    const firstMenuGroup = page.locator('li.vben-sub-menu').first();
    await expect(firstMenuGroup).toHaveClass(/is-opened/u);
    await expect(page.locator('aside')).not.toContainText('账户资料');

    const sidebar = page.locator('aside').first();
    const sidebarSpacer = sidebar.locator('xpath=preceding-sibling::div[1]');
    const main = page.locator('main').first();
    const initialMainBox = await main.boundingBox();
    expect(initialMainBox).not.toBeNull();
    for (let round = 0; round < 3; round += 1) {
      await sidebar.hover();
      await page.waitForTimeout(180);
      await expect(sidebar).toHaveCSS('width', '204px');
      await expect(sidebarSpacer).toHaveCSS('width', '204px');
      await page.mouse.move(
        Math.max(260, (initialMainBox?.x ?? 260) + 20),
        Math.max(100, (initialMainBox?.y ?? 100) + 20),
      );
      await page.waitForTimeout(180);
      await expect(sidebar).toHaveCSS('width', '204px');
      await expect(sidebarSpacer).toHaveCSS('width', '204px');
      const currentMainBox = await main.boundingBox();
      expect(currentMainBox?.x).toBe(initialMainBox?.x);
      expect(currentMainBox?.width).toBe(initialMainBox?.width);
    }

    await captureCustomerScreenshot(page, 'customer-console-overview-zh');
    await page.getByRole('button', { name: /订单管理/u }).click();
    await page.waitForURL(/\/account\/orders$/u);
  });

  test('TC029b: 英文移动端无溢出、无中文和未翻译键', async ({ page }) => {
    await page.setViewportSize({ height: 844, width: 390 });
    await mockCustomerAPIs(page, { locale: 'en-US' });
    await loginCustomer(page);

    await expect(page.getByText('Quick access', { exact: true })).toBeVisible();
    await expect(
      page.getByText('Account balance', { exact: true }),
    ).toBeVisible();
    await expect(
      page.locator('main').getByText('Browse products', { exact: true }).first(),
    ).toBeVisible();
    const mobileSidebar = page.locator('aside');
    const mobileSidebarToggle = page.getByTestId('sidebar-mobile-toggle');
    await expect(mobileSidebarToggle).toBeVisible();
    await expect(mobileSidebar).toHaveCSS('width', '0px');

    await mobileSidebarToggle.click();
    await expect(mobileSidebar).toHaveCSS('width', '204px');
    await expect(
      mobileSidebar.getByText('Products & Services', { exact: true }),
    ).toBeVisible();
    await expect(
      mobileSidebar.getByRole('menuitem', { name: 'Browse products' }),
    ).toBeVisible();
    await page.getByTestId('sidebar-mobile-mask').dispatchEvent('click');
    await expect(mobileSidebar).toHaveCSS('width', '0px');

    await mobileSidebarToggle.click();
    await mobileSidebar
      .getByRole('menuitem', { name: 'Browse products' })
      .click();
    await page.waitForURL(/\/account\/products\/catalog$/u);
    await expect(mobileSidebar).toHaveCSS('width', '0px');

    const mobileAccountAction = page.getByRole('button', {
      name: 'Manage account',
    });
    await expect(mobileAccountAction).toBeVisible();
    expect(
      await mobileAccountAction.evaluate(
        (element) => getComputedStyle(element).justifyContent,
      ),
    ).toBe('center');
    await expect(page.locator('body')).not.toContainText('常用入口');
    await expect(page.locator('body')).not.toContainText(
      'page.customer.quickAccess',
    );
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await captureCustomerScreenshot(
      page,
      'customer-console-overview-en-mobile',
    );
  });

  test('TC029c: 后台自定义菜单排在末尾并原样安全打开外链', async ({ page }) => {
    await page.addInitScript(() => {
      const browserWindow = window as typeof window & {
        __externalMenuOpenCalls?: Array<{
          features?: string;
          target?: string;
          url?: string;
        }>;
      };
      browserWindow.__externalMenuOpenCalls = [];
      window.open = ((
        url?: string | URL,
        target?: string,
        features?: string,
      ) => {
        browserWindow.__externalMenuOpenCalls?.push({
          features,
          target,
          url: String(url || ''),
        });
        return null;
      }) as typeof window.open;
    });
    await mockCustomerAPIs(page, {
      externalMenus: [
        {
          enabled: true,
          icon: 'lucide:circle-help',
          id: 1,
          nameEn: 'Help center',
          nameZh: '帮助中心',
          sortOrder: 10,
          url: 'https://docs.example.com/help?source=console#start',
          version: 1,
        },
      ],
    });
    await loginCustomer(page);

    const sidebar = page.locator('aside');
    await expect(sidebar.getByText('更多服务', { exact: true })).toBeVisible();
    await expect(sidebar.getByText('帮助中心', { exact: true })).toBeVisible();

    const menuText = await sidebar.innerText();
    expect(menuText.indexOf('更多服务')).toBeGreaterThan(
      menuText.indexOf('服务支持'),
    );
    await sidebar.getByText('帮助中心', { exact: true }).click();
    const calls = await page.evaluate(() => {
      return (
        window as typeof window & {
          __externalMenuOpenCalls?: Array<unknown>;
        }
      ).__externalMenuOpenCalls;
    });
    expect(calls).toEqual([
      {
        features: 'noopener=yes,noreferrer=yes',
        target: '_blank',
        url: 'https://docs.example.com/help?source=console#start',
      },
    ]);
  });
});

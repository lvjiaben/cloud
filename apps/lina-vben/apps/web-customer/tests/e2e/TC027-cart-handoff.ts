import { CommercePage } from '../pages/CommercePage';
import { installCustomerCommerceMock } from '../support/customer-commerce-api-mock';
import { expect, test } from '../fixtures/customer';

test.describe('TC027 官网购物车安全接收', () => {
  test('TC027a: 登录后只凭 HttpOnly Cookie 请求后端合并且地址栏不带凭据', async ({
    context,
    page,
  }) => {
    await installCustomerCommerceMock(page);
    await context.addCookies([
      {
        httpOnly: true,
        name: 'lina_idc_guest_cart',
        sameSite: 'Lax',
        url: 'http://127.0.0.1:6032',
        value: 'gcart_browser_only',
      },
    ]);
    let mergeRequests = 0;
    let credentialReachedMerge = false;
    await page.route(
      '**/x/idc-core/api/v1/account/commerce/guest-cart-merge',
      async (route) => {
        mergeRequests += 1;
        credentialReachedMerge = (
          route.request().headers().cookie ?? ''
        ).includes('lina_idc_guest_cart=gcart_browser_only');
        expect(route.request().postData()).toBeNull();
        await route.fulfill({
          body: JSON.stringify({
            code: 0,
            data: { merged: true },
            message: 'OK',
          }),
          contentType: 'application/json',
          headers: {
            'set-cookie':
              'lina_idc_guest_cart=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0',
          },
          status: 200,
        });
      },
    );

    const commerce = new CommercePage(page);
    await commerce.login();
    await commerce.open('/account/cart');

    await expect(commerce.root).toContainText('入门云服务器');
    expect(mergeRequests).toBe(1);
    expect(credentialReachedMerge).toBe(true);
    expect(page.url()).not.toContain('gcart_');
    expect(await page.evaluate(() => document.cookie)).not.toContain(
      'lina_idc_guest_cart',
    );
    expect(
      await page.evaluate(() =>
        JSON.stringify({
          local: { ...localStorage },
          session: { ...sessionStorage },
        }),
      ),
    ).not.toContain('gcart_browser_only');
  });

  test('TC027b: 中文移动端收到购物车后可完成核价、结账和付款准备', async ({
    page,
  }) => {
    await page.setViewportSize({ height: 844, width: 390 });
    await installCustomerCommerceMock(page);
    const commerce = new CommercePage(page);
    await commerce.login();
    await commerce.open('/account/cart');

    await expect(commerce.root).toContainText('入门云服务器');
    await page.getByTestId('commerce-primary-action').click();
    await expect(page.getByTestId('commerce-final-quote')).toContainText(
      'CNY 108.12',
    );
    await page.getByTestId('commerce-primary-action').click();
    await expect(page.getByTestId('commerce-created-order')).toBeVisible();
    await expect(commerce.root).toContainText('等待实名认证');
    expect(await commerce.hasHorizontalOverflow()).toBe(false);
  });
});

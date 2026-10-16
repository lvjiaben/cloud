import { randomBytes } from 'node:crypto';
import { expect, test } from '@host-tests/fixtures/auth';
import { CatalogPricingPage } from '../pages/CatalogPricingPage';
import { LevelPricingPage } from '../pages/LevelPricingPage';
import { queryPgScalar } from '@host-tests/support/postgres';
import { CatalogPricingFixture } from '../support/catalog-pricing-fixture';

test.describe('TC-36 商品代理等级价和客户定制价格', () => {
  test('TC-36a: 商品入口等级价格增查改删，失败保留输入和历史', async ({ adminPage }) => {
    test.setTimeout(120_000);
    const fixture = new CatalogPricingFixture(randomBytes(16).toString('hex'));
    const pricing = new CatalogPricingPage(adminPage);
    try {
      fixture.seed();
      const initial = await pricing.openAgents(fixture.product(), fixture.productName());
      expect(initial.code).toBe(0); expect(initial.data.total).toBe(0);
      await expect(pricing.text(`代理价格 · ${fixture.productName()}`)).toBeVisible();
      await expect(pricing.prices).toContainText('先用客户定制价，再用商品单独代理价，再用等级总百分比，最后用普通价。');
      await expect(adminPage.getByTestId(`catalog-product-agent-prices-${fixture.product()}`)).toHaveCount(0);
      await expect(adminPage.getByTestId('catalog-edit-agent-prices')).toHaveText('设置代理价格');
      await pricing.screenshot('agent-initial');
      await pricing.addFixed('80.00', 'levelNo', fixture.levelName);
      await expect(pricing.field('status')).toContainText('启用');
      await expect(pricing.field('customerNo')).toHaveCount(0);
      await pricing.screenshot('agent-form');
      const created = await pricing.save('POST'); expect(created.code, JSON.stringify(created)).toBe(0);
      const priceNo = created.data.item.priceNo;
      expect(created.data.item).toMatchObject({ productNo: fixture.product(), levelNo: fixture.level, fixedPriceMinor: '8000', version: 1 });
      expect(fixture.latest(priceNo)).toBe('active:8000:1');
      await expect(pricing.drawer).toBeHidden(); await expect(pricing.prices).toContainText(fixture.levelName);
      await expect(pricing.prices.getByText('CNY 80.00', { exact: true })).toBeVisible(); await pricing.screenshot('agent-created');
      const filtered = await pricing.searchPrices(fixture.levelName);
      expect(filtered.code).toBe(0); expect(filtered.data.total).toBe(1);
      expect(filtered.data.items[0].productNo).toBe(fixture.product()); await pricing.screenshot('agent-filtered');
      await pricing.edit(priceNo); await pricing.fill('amount', '75.00');
      const conflict = '**/sales-management/prices';
      await adminPage.route(conflict, route => route.request().method() === 'PUT'
        ? route.fulfill({ json: { code: 409, message: '价格已经被其他管理员修改，请刷新后重试。', data: null } }) : route.continue());
      const failed = await pricing.save('PUT'); expect(failed.code).not.toBe(0);
      await expect(pricing.formError).toHaveText('保存失败。请检查必填内容、金额和时间；若数据已变更，请关闭后刷新重新操作。输入内容已保留。'); await expect(await pricing.input('amount')).toHaveValue('75.00');
      expect(fixture.latest(priceNo)).toBe('active:8000:1'); await pricing.screenshot('agent-edit-conflict');
      await adminPage.unroute(conflict);
      const edited = await pricing.save('PUT'); expect(edited.code).toBe(0);
      expect(fixture.latest(priceNo)).toBe('active:7500:2'); expect(fixture.history(priceNo)).toBe('2');
      await expect(pricing.drawer).toBeHidden(); await expect(pricing.prices.getByText('CNY 75.00', { exact: true })).toBeVisible(); await pricing.screenshot('agent-updated');
      await adminPage.route(conflict, route => route.request().method() === 'DELETE'
        ? route.fulfill({ json: { code: 409, message: '价格已经变更，请刷新后重新删除。', data: null } }) : route.continue());
      expect((await pricing.remove(priceNo)).code).not.toBe(0);
      await expect(adminPage.getByTestId('price-delete-error')).toHaveText('删除失败，价格可能已被其他管理员修改。请刷新列表后重试。');
      await expect(pricing.prices).toContainText(fixture.levelName); expect(fixture.history(priceNo)).toBe('2');
      await pricing.screenshot('agent-delete-conflict'); await adminPage.unroute(conflict);
      expect((await pricing.remove(priceNo)).code).toBe(0);
      expect(fixture.latest(priceNo)).toBe('deleted:7500:3'); expect(fixture.history(priceNo)).toBe('3');
      await expect(pricing.prices.getByText(fixture.levelName, { exact: true })).toHaveCount(0); await pricing.screenshot('agent-deleted');
      const otherProduct = await pricing.openAgents(fixture.product(1), fixture.productName(1));
      expect(otherProduct.code).toBe(0); expect(otherProduct.data.total).toBe(0); await pricing.screenshot('agent-other-product-empty');
    } finally { fixture.cleanup(); }
  });
  test('TC-36b: 定制价格指定客户商品完整 CRUD，不混入代理等级价格', async ({ adminPage }) => {
    test.setTimeout(120_000);
    const fixture = new CatalogPricingFixture(randomBytes(16).toString('hex'));
    const pricing = new CatalogPricingPage(adminPage);
    try {
      fixture.seed();
      await pricing.openAgents(fixture.product(), fixture.productName());
      await pricing.addFixed('80.00', 'levelNo', fixture.levelName); expect((await pricing.save('POST')).code).toBe(0);
      await pricing.openCustom(); await expect(pricing.prices).toContainText('定制价格');
      await expect(pricing.prices).toContainText('普通价100元、代理价80元、定制价50元，这位客户就按50元买。');
      await expect(pricing.prices.getByText(fixture.levelName, { exact: true })).toHaveCount(0);
      await pricing.screenshot('custom-initial');
      await pricing.addFixed('50.00', 'customerNo', fixture.customerLabel, fixture.productName());
      await expect(pricing.field('levelNo')).toHaveCount(0); await expect(pricing.field('status')).toContainText('启用');
      await pricing.screenshot('custom-form');
      const created = await pricing.save('POST'); expect(created.code, JSON.stringify(created)).toBe(0);
      const priceNo = created.data.item.priceNo;
      expect(created.data.item).toMatchObject({ productNo: fixture.product(), customerNo: fixture.customer, fixedPriceMinor: '5000', version: 1 });
      expect(fixture.latest(priceNo)).toBe('active:5000:1'); await expect(pricing.drawer).toBeHidden();
      await expect(pricing.prices).toContainText(fixture.customerName); await expect(pricing.prices.getByText('CNY 50.00', { exact: true })).toBeVisible(); await pricing.screenshot('custom-created');
      const filtered = await pricing.searchPrices(fixture.customerName);
      expect(filtered.code).toBe(0); expect(filtered.data.total).toBe(1);
      expect(filtered.data.items[0].customerNo).toBe(fixture.customer); await pricing.screenshot('custom-filtered');
      await pricing.edit(priceNo); await pricing.fill('amount', '45.00');
      expect((await pricing.save('PUT')).code).toBe(0); expect(fixture.latest(priceNo)).toBe('active:4500:2');
      await expect(pricing.drawer).toBeHidden(); await expect(pricing.prices.getByText('CNY 45.00', { exact: true })).toBeVisible(); await pricing.screenshot('custom-updated');
      expect((await pricing.remove(priceNo)).code).toBe(0);
      expect(fixture.latest(priceNo)).toBe('deleted:4500:3'); expect(fixture.history(priceNo)).toBe('3');
      await expect(pricing.prices.getByText(fixture.customerName, { exact: true })).toHaveCount(0); await pricing.screenshot('custom-deleted');
      const agent = await pricing.openAgents(fixture.product(), fixture.productName());
      expect(agent.code).toBe(0); expect(agent.data.total).toBe(1);
      expect(agent.data.items[0]).toMatchObject({ levelNo: fixture.level, fixedPriceMinor: '8000' });
      await expect(pricing.prices.getByText('CNY 80.00', { exact: true })).toBeVisible(); await pricing.screenshot('custom-delete-retains-agent');
    } finally { fixture.cleanup(); }
  });
  test('TC-36c: 等级总百分比独立保存修改清除，普通等级编辑不覆盖且冲突保留', async ({ adminPage }) => {
    test.setTimeout(120_000);
    const fixture = new CatalogPricingFixture(randomBytes(16).toString('hex'));
    const pricing = new LevelPricingPage(adminPage);
    const state = () => queryPgScalar(`SELECT COALESCE(percentage_basis_points::text,'null') || ':' || status_code || ':' || version FROM plugin_idc_core_sales_level_price WHERE level_no='${fixture.level}' ORDER BY version DESC LIMIT 1`);
    try {
      fixture.seed(); await pricing.findLevel(fixture.levelName);
      await expect(pricing.pricingButton(fixture.level)).toHaveText('代理价格'); await pricing.screenshot('level-percentage-initial');
      const empty = await pricing.openPricing(fixture.level); expect(empty.code).toBe(0);
      expect(empty.data.item).toMatchObject({ levelNo: fixture.level, percentageBasisPoints: null, version: 0 });
      await expect(pricing.pricingDrawer).toContainText('填80即按原价80%收费，留空取消设置；客户定制价和商品单独代理价优先。');
      await expect(adminPage.getByTestId('sales-level-pricing-clear')).toHaveCount(0);
      await expect(adminPage.getByTestId('sales-level-pricing-reload')).toHaveCount(0);
      const bounds = await pricing.pricingDrawer.locator('xpath=ancestor::*[@role="dialog"]').boundingBox(); expect(bounds!.width).toBeLessThan(460);
      const actionRow = pricing.pricingButton(fixture.level).locator('xpath=ancestor::tr');
      const editBounds = await actionRow.getByRole('button', { name: /^修\s*改$/u }).boundingBox(); const priceBounds = await pricing.pricingButton(fixture.level).boundingBox();
      expect(Math.abs(editBounds!.y - priceBounds!.y)).toBeLessThan(5);
      await pricing.percentageInput.fill('80'); await pricing.screenshot('level-percentage-form');
      const saved = await pricing.savePricing(fixture.level); expect(saved.code, JSON.stringify(saved)).toBe(0);
      expect(saved.data.item).toMatchObject({ percentageBasisPoints: 8000, version: 1 }); expect(state()).toBe('8000:active:1');
      await expect(pricing.pricingDrawer).toBeHidden(); await expect(pricing.text('80%')).toBeVisible(); await pricing.screenshot('level-percentage-saved');
      // The ordinary level editor has its own version and must not clear pricing.
      await pricing.editLevel(fixture.levelName); await pricing.fill('englishName', 'Level edit keeps negotiated percentage');
      const edited = pricing.waitForResponse(response => response.url().endsWith('/sales-management/levels') && response.request().method() === 'PUT');
      await pricing.button(/确\s*认/u).click(); expect((await (await edited).json()).code).toBe(0);
      await expect(pricing.drawer).toBeHidden(); expect(state()).toBe('8000:active:1');
      await pricing.openPricing(fixture.level); await expect(pricing.percentageInput).toHaveValue('80');
      await pricing.percentageInput.fill('75.25');
      const path = `**/sales-management/levels/${fixture.level}/pricing`;
      await adminPage.route(path, route => route.request().method() === 'PUT' ? route.fulfill({ json: { code: 409, message: '价格已变更，请重新加载。', data: null } }) : route.continue());
      expect((await pricing.savePricing(fixture.level)).code).not.toBe(0);
      await expect(pricing.pricingError).toHaveText('保存失败，输入已保留。价格有变更时请关闭后重新打开；也请检查价格管理权限。'); await expect(pricing.percentageInput).toHaveValue('75.25');
      expect(state()).toBe('8000:active:1'); await pricing.screenshot('level-percentage-conflict'); await adminPage.unroute(path);
      const changed = await pricing.savePricing(fixture.level); expect(changed.code).toBe(0);
      expect(changed.data.item).toMatchObject({ percentageBasisPoints: 7525, version: 2 }); expect(state()).toBe('7525:active:2');
      await expect(pricing.pricingDrawer).toBeHidden(); await expect(pricing.text('75.25%')).toBeVisible(); await pricing.screenshot('level-percentage-updated');
      await pricing.openPricing(fixture.level); await pricing.percentageInput.fill('101'); await pricing.button(/确\s*认/u).click();
      await expect(pricing.pricingError).toHaveText('请输入0到100之间的数字，最多保留两位小数。'); expect(state()).toBe('7525:active:2'); await pricing.screenshot('level-percentage-invalid');
      await pricing.percentageInput.fill('0');
      const free = await pricing.savePricing(fixture.level); expect(free.code).toBe(0);
      expect(free.data.item).toMatchObject({ percentageBasisPoints: 0, version: 3 }); expect(state()).toBe('0:active:3');
      await pricing.openPricing(fixture.level); await expect(pricing.percentageInput).toHaveValue('0');
      const cleared = pricing.waitForResponse(response => response.url().includes(`/levels/${fixture.level}/pricing`) && response.request().method() === 'PUT');
      await pricing.clearPricing(fixture.level);
      const clearResult = await (await cleared).json(); expect(clearResult.code).toBe(0);
      expect(clearResult.data.item).toMatchObject({ percentageBasisPoints: null, version: 4 }); expect(state()).toBe('0:cleared:4');
      await expect(pricing.pricingDrawer).toBeHidden(); await expect(pricing.text('未设置')).toBeVisible(); await pricing.screenshot('level-percentage-cleared');
    } finally { fixture.cleanup(); }
  });

  test('TC-36d: 只有代理等级查看权限时隐藏总百分比操作且不发写请求', async ({ adminPage }) => {
    test.setTimeout(60_000);
    const fixture = new CatalogPricingFixture(randomBytes(16).toString('hex'));
    const pricing = new LevelPricingPage(adminPage); let writes = 0;
    const onRequest = (request: import('@host-tests/support/playwright').Request) => {
      if (request.url().includes('/sales-management/') && !['GET', 'HEAD'].includes(request.method())) writes++;
    };
    try {
      fixture.seed(); adminPage.on('request', onRequest);
      await adminPage.route('**/api/v1/user/info', async route => {
        const response = await route.fetch(); const body = await response.json();
        body.data.permissions = ['idc:admin:sales-channel:overview:view'];
        await route.fulfill({ response, json: body });
      });
      const list = await pricing.findLevel(fixture.levelName); expect(list.code).toBe(0); expect(list.data.items[0].levelNo).toBe(fixture.level);
      await expect(pricing.text('代理价格')).toHaveCount(0);
      await expect(pricing.pricingButton(fixture.level)).toHaveCount(0);
      await expect(pricing.addButton).toHaveCount(0); await pricing.screenshot('level-percentage-read-only');
      expect(writes).toBe(0);
    } finally { adminPage.off('request', onRequest); fixture.cleanup(); }
  });

});

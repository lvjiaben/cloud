import { test, expect } from '../support/catalog-auth';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';
import { installCatalogManagementMock } from '../support/catalog-mock';
import { config } from '@host-tests/fixtures/config';

test.describe('TC-41 商品计费与基础价格编辑', () => {
  test('TC-41a: 草稿添加周期、金额精确保存及失败保留输入', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    await adminPage.route("**/x/idc-core/api/v1/currencies", route => route.fulfill({ json: { code: 0, data: { currencies: ["CNY"] }, message: "success" } }));
    let rule: Record<string, unknown> | undefined;
    let price: Record<string, unknown> | undefined;
    let rejectPrice = true;
    let tax: Record<string, unknown> | undefined;
    let rejectTax = true;
    const writes: Record<string, unknown>[] = [];
    await adminPage.route('**/admin/catalog/products/product-manual-service', async route => {
      await route.fulfill({ json: { code: 0, data: {
        product: { productNo: 'product-manual-service', nameZhCn: '人工迁移服务', nameEnUs: 'Migration', categoryNo: '', status: 'draft' },
        version: { versionNo: 'version-manual-service-1', status: 'draft', source: 'manual', versionNumber: 1 },
        billingRules: rule ? [rule] : [], prices: price ? [price] : [], taxPolicies: tax ? [tax] : [], inventory: { trackingEnabled: false, sellableQuantity: 0, version: 1 }, fulfillmentBinding: null, fulfillmentPolicy: null,
      }, message: 'success' } });
    });
    await adminPage.route('**/admin/catalog/product-versions/*/billing-rules', async route => {
      const input = route.request().postDataJSON(); writes.push(input);
      rule = { ...input, billingRuleNo: 'billing-editor' };
      await route.fulfill({ json: { code: 0, data: rule, message: 'success' } });
    });
    await adminPage.route('**/admin/catalog/product-versions/*/billing-rules/*/price-schedules', async route => {
      const input = route.request().postDataJSON(); writes.push(input);
      if (rejectPrice) { rejectPrice = false; await route.fulfill({ json: { code: 500, data: null, message: '价格保存失败，请重试' } }); return; }
      price = { ...input.tiers[0], currencyCode: input.currencyCode, priceNo: 'price-editor', billingRuleNo: 'billing-editor', configurationPriceDimensionNo: '' };
      await route.fulfill({ json: { code: 0, data: {}, message: 'success' } });
    });
    await adminPage.route('**/admin/catalog/product-versions/*/tax-policies', async route => {
      const input = route.request().postDataJSON(); writes.push(input);
      if (rejectTax) { rejectTax = false; await route.fulfill({ json: { code: 500, message: '税务保存失败，请重试', data: null } }); return; }
      tax = { ...input, taxPolicyNo: 'tax-editor' };
      await route.fulfill({ json: { code: 0, data: tax, message: 'success' } });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.openProductDetail('product-manual-service');
    await workspace.addBillingPeriod();
    await expect(adminPage.getByTestId('catalog-billing-modal')).toContainText('收费周期');
    await workspace.screenshot('billing-form'); await workspace.confirmBillingPeriod();
    await expect(adminPage.getByTestId('catalog-billing-modal')).toBeHidden();
    expect(writes[0]).toMatchObject({ chargeType: 'recurring', periodUnit: 'month', periodCount: 1, pricePhase: 'both', quantityMode: 'fixed', trialQuantity: 0 });
    await workspace.addBasePrice('billing-editor', '100.25', '90.50');
    await workspace.screenshot('base-price-form'); await workspace.confirmBasePrice();
    await expect(adminPage.getByTestId('catalog-base-price-modal')).toBeVisible();
    await expect(adminPage.getByTestId('catalog-base-price-amount')).toHaveValue('100.25');
    await workspace.screenshot('base-price-retry'); await workspace.confirmBasePrice();
    await expect(adminPage.getByTestId('catalog-base-price-modal')).toBeHidden();
    expect(writes.at(-1)).toMatchObject({ currencyCode: 'CNY', status: 'active', tiers: [{ minimumQuantity: 0, maximumQuantity: null, amountMinor: '10025', setupFeeMinor: '0', renewalAmountMinor: '9050' }] });
    await expect(adminPage.getByTestId('catalog-detail-pricing')).toContainText('100.25');
    await workspace.screenshot('base-price-saved');
    await workspace.addTax('6.25', 'cloud_service');
    await expect(adminPage.getByTestId('catalog-tax-modal')).toContainText('应税');
    await expect(adminPage.getByTestId('catalog-tax-modal')).toContainText('四舍五入');
    await workspace.screenshot('tax-form');
    await workspace.confirmTax();
    await expect(adminPage.getByTestId('catalog-tax-rate')).toHaveValue('6.25');
    await workspace.screenshot('tax-retry');
    await workspace.confirmTax();
    await expect(adminPage.getByTestId('catalog-tax-modal')).toBeHidden();
    expect(writes.at(-1)).toMatchObject({ currencyCode: 'CNY', taxTreatment: 'taxable', taxRateBasisPoints: 625, priceIncludesTax: false, taxCategoryCode: 'cloud_service', roundingScope: 'line', roundingMode: 'half_up', roundingQuantumMinor: '1' });
    await expect(adminPage.getByTestId('catalog-detail-tax')).toContainText('6.25');
    await expect(adminPage.getByTestId('catalog-tax-add')).toHaveCount(0);
    await workspace.screenshot('tax-saved');
  });
  for (const [suffix, treatment, label, expectedRate] of [
    ['b', 'exempt', '免税', null], ['c', 'zero_rated', '零税率', 0],
  ] as const) {
    test(`TC-41${suffix}: ${label}明确保存，不沿用应税税率`, async ({ adminPage }) => {
      await installCatalogManagementMock(adminPage);
      let saved: Record<string, unknown> | undefined;
      await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({ json: { code: 0, data: {
        product: { productNo: 'product-manual-service', nameZhCn: '人工迁移服务', nameEnUs: 'Migration', status: 'draft' },
        version: { versionNo: 'version-manual-service-1', status: 'draft', source: 'manual', versionNumber: 1 },
        billingRules: [], prices: [{ currencyCode: 'CNY', amountMinor: '10000' }], taxPolicies: saved ? [saved] : [],
        inventory: { trackingEnabled: false, sellableQuantity: 0, version: 1 }, fulfillmentBinding: null, fulfillmentPolicy: null,
      }, message: 'success' } }));
      await adminPage.route('**/admin/catalog/product-versions/*/tax-policies', async route => {
        saved = { ...route.request().postDataJSON(), taxPolicyNo: 'tax-editor' };
        await route.fulfill({ json: { code: 0, data: saved, message: 'success' } });
      });
      const workspace = new CatalogWorkspacePage(adminPage);
      await workspace.goto(); await workspace.openProductDetail('product-manual-service');
      await workspace.openTax(label, 'service');
      await expect(adminPage.getByTestId('catalog-tax-rate')).toHaveCount(0);
      await expect(adminPage.getByTestId('catalog-tax-modal')).toContainText(label);
      await workspace.screenshot(`tax-${treatment}-form`);
      await workspace.confirmTax();
      await expect(adminPage.getByTestId('catalog-tax-modal')).toBeHidden();
      expect(saved).toMatchObject({ taxTreatment: treatment, taxRateBasisPoints: expectedRate, currencyCode: 'CNY' });
      await expect(adminPage.getByTestId('catalog-detail-tax')).toContainText(label);
      await workspace.screenshot(`tax-${treatment}-saved`);
    });
  }
  test('TC-41d: 非法税率不提交，修正后可以保存', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    const writes: Record<string, unknown>[] = [];
    await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({ json: { code: 0, data: {
      product: { productNo: 'product-manual-service', nameZhCn: '人工迁移服务', status: 'draft' },
      version: { versionNo: 'version-manual-service-1', status: 'draft', source: 'manual', versionNumber: 1 },
      billingRules: [], prices: [{ currencyCode: 'CNY' }], taxPolicies: [], inventory: { trackingEnabled: false }, fulfillmentBinding: null,
    }, message: 'success' } }));
    await adminPage.route('**/admin/catalog/product-versions/*/tax-policies', route => {
      writes.push(route.request().postDataJSON()); return route.fulfill({ json: { code: 0, data: {}, message: 'success' } });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.openProductDetail('product-manual-service');
    await workspace.addTax('100.01', 'service'); await workspace.confirmTax();
    await expect(adminPage.getByTestId('catalog-tax-modal')).toBeVisible();
    expect(writes).toHaveLength(0);
    await workspace.setTaxRate('6.251'); await workspace.confirmTax();
    expect(writes).toHaveLength(0);
    await workspace.screenshot('tax-invalid-rate');
    await workspace.setTaxRate('100'); await workspace.confirmTax();
    await expect(adminPage.getByTestId('catalog-tax-modal')).toBeHidden();
    expect(writes).toHaveLength(1); expect(writes[0]).toMatchObject({ taxRateBasisPoints: 10000 });
  });
  test('TC-41e: 只有查看权限不能添加周期、价格或税务', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    await adminPage.route('**/api/v1/user/info', async route => {
      const response = await route.fetch({ url: `${config.apiBaseURL}user/info` }); const body = await response.json();
      body.data.permissions = ['idc:admin:catalog:product:list', 'idc:admin:catalog:product:query'];
      await route.fulfill({ response, json: body });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.openProductDetail('product-manual-service');
    await workspace.selectEditSection('价格与税务');
    await expect(adminPage.getByTestId('catalog-billing-add')).toHaveCount(0);
    await expect(adminPage.getByTestId('catalog-tax-add')).toHaveCount(0);
    await expect(adminPage.locator('[data-testid^="catalog-base-price-add-"]')).toHaveCount(0);
    await workspace.screenshot('pricing-read-only');
  });
  test('TC-41f: 一次性周期不带续费，缺币种禁止保存价格', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    let rule: Record<string, unknown> | undefined;
    let priceCalls = 0;
    await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({ json: { code: 0, data: {
      product: { productNo: 'product-manual-service', nameZhCn: '人工迁移服务', status: 'draft' },
      version: { versionNo: 'version-manual-service-1', status: 'draft', source: 'manual', versionNumber: 1 },
      billingRules: rule ? [rule] : [], prices: [], taxPolicies: [], inventory: { trackingEnabled: false }, fulfillmentBinding: null,
    }, message: 'success' } }));
    await adminPage.route('**/admin/catalog/product-versions/*/billing-rules', route => {
      rule = { ...route.request().postDataJSON(), billingRuleNo: 'once-editor' };
      return route.fulfill({ json: { code: 0, data: rule, message: 'success' } });
    });
    await adminPage.route('**/x/idc-core/api/v1/currencies', route => route.fulfill({ json: { code: 0, data: { currencies: [] }, message: 'success' } }));
    await adminPage.route('**/price-schedules', route => { priceCalls++; return route.fulfill({ json: { code: 0, data: {} } }); });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.openProductDetail('product-manual-service');
    await workspace.addBillingPeriod(); await workspace.selectBillingPeriod('一次'); await workspace.confirmBillingPeriod();
    await expect(adminPage.getByTestId('catalog-billing-modal')).toBeHidden();
    expect(rule).toMatchObject({ chargeType: 'one_time', pricePhase: 'new_purchase', periodUnit: 'once', periodCount: 1 });
    await workspace.addBasePrice('once-editor', '100', '');
    await expect(adminPage.getByTestId('catalog-base-price-renewal')).toHaveCount(0);
    await expect(adminPage.getByText('没有可添加价格的币种。请先配置货币，或检查该周期是否已有价格。', { exact: true })).toBeVisible();
    await expect(adminPage.getByRole('dialog').filter({ has: adminPage.getByTestId('catalog-base-price-modal') }).getByRole('button', { name: /确\s*认/ })).toBeDisabled();
    expect(priceCalls).toBe(0);
    await workspace.screenshot('base-price-no-currency');
  });

  test('TC-41g: 配置价格先验证金额，保存失败重试复用收费项', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    await adminPage.route('**/x/idc-core/api/v1/currencies', route => route.fulfill({ json: { code: 0, data: { currencies: ['CNY'] }, message: 'success' } }));
    let dimensionCalls = 0;
    let dimensionNo = '';
    const priceWrites: Record<string, unknown>[] = [];
    await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({ json: { code: 0, data: {
      product: { productNo: 'product-manual-service', nameZhCn: '人工迁移服务', status: 'draft' },
      version: { versionNo: 'version-manual-service-1', status: 'draft', source: 'manual', versionNumber: 1 },
      billingRules: [{ billingRuleNo: 'config-rule', status: 'active', chargeType: 'recurring', periodUnit: 'month', periodCount: 1, pricePhase: 'both', quantityMode: 'fixed' }],
      prices: [], taxPolicies: [], inventory: { trackingEnabled: false }, fulfillmentBinding: null,
    }, message: 'success' } }));
    await adminPage.route('**/configuration-price-candidates', route => route.fulfill({ json: { code: 0, data: { candidates: [{
      candidateToken: 'configuration-token', fieldLabelZhCN: '内存', fieldLabelEnUS: 'Memory', candidateLabelZhCN: '8GB', candidateLabelEnUS: '8GB', dimensionNo,
    }] }, message: 'success' } }));
    await adminPage.route('**/configuration-price-dimensions', route => {
      dimensionCalls++; dimensionNo = 'dimension-memory';
      return route.fulfill({ json: { code: 0, data: { dimensionNo }, message: 'success' } });
    });
    await adminPage.route('**/billing-rules/*/price-schedules', route => {
      priceWrites.push(route.request().postDataJSON());
      return route.fulfill({ json: priceWrites.length === 1
        ? { code: 500, data: null, message: '配置价格保存失败，请重试' }
        : { code: 0, data: {}, message: 'success' } });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.openProductDetail('product-manual-service');
    await workspace.openConfigurationPrice();
    const modal = adminPage.getByTestId('catalog-configuration-price-modal');
    await expect(modal).toContainText('收费配置项');
    await expect(modal).toContainText('内存 · 8GB');
    await workspace.setConfigurationPrice('1.001', '5.50');
    await workspace.confirmConfigurationPrice();
    await expect(adminPage.getByText('请输入正确的金额', { exact: true })).toBeVisible();
    expect(dimensionCalls).toBe(0); expect(priceWrites).toHaveLength(0);
    await workspace.screenshot('configuration-price-invalid');
    await workspace.setConfigurationPrice('10.25', '5.50');
    await workspace.confirmConfigurationPrice();
    await expect(adminPage.getByText('配置价格保存失败，请重试', { exact: true })).toBeVisible();
    await expect(modal).toBeVisible();
    await expect(adminPage.getByTestId('catalog-configuration-price-amount')).toHaveValue('10.25');
    expect(dimensionCalls).toBe(1);
    await workspace.screenshot('configuration-price-retry');
    await workspace.confirmConfigurationPrice();
    await expect(modal).toBeHidden();
    expect(dimensionCalls).toBe(1); expect(priceWrites).toHaveLength(2);
    expect(priceWrites[1]).toMatchObject({ configurationPriceDimensionNo: 'dimension-memory', currencyCode: 'CNY', tiers: [{ amountMinor: '1025', setupFeeMinor: '0', renewalAmountMinor: '550', minimumQuantity: 0, maximumQuantity: null }] });
    await workspace.screenshot('configuration-price-saved');
  });

  for (const [suffix, mode, label] of [['h', 'per_unit', '按数量单价累加'], ['i', 'band_total', '按区间整档收费']] as const) {
    test(`TC-41${suffix}: 数量配置${label}及区间精确保存`, async ({ adminPage }) => {
      await installCatalogManagementMock(adminPage);
      await adminPage.route('**/x/idc-core/api/v1/currencies', route => route.fulfill({ json: { code: 0, data: { currencies: ['CNY'] }, message: 'success' } }));
      const writes: Record<string, unknown>[] = [];
      await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({ json: { code: 0, data: {
        product: { productNo: 'product-manual-service', nameZhCn: '人工迁移服务', status: 'draft' },
        version: { versionNo: 'version-manual-service-1', status: 'draft', source: 'manual', versionNumber: 1 },
        billingRules: [{ billingRuleNo: 'config-rule', status: 'active', chargeType: 'recurring', periodUnit: 'month', periodCount: 1, pricePhase: 'both', quantityMode: 'fixed' }],
        prices: [], taxPolicies: [], inventory: { trackingEnabled: false }, fulfillmentBinding: null,
      }, message: 'success' } }));
      await adminPage.route('**/configuration-price-candidates', route => route.fulfill({ json: { code: 0, data: { candidates: [{
        candidateToken: 'quantity-token', fieldLabelZhCN: '数据盘容量', fieldLabelEnUS: 'Data disk', candidateLabelZhCN: '数据盘容量', candidateLabelEnUS: 'Data disk', dimensionNo: '', quantity: true, pricingMode: '',
      }] }, message: 'success' } }));
      await adminPage.route('**/configuration-price-dimensions', route => {
        writes.push(route.request().postDataJSON());
        return route.fulfill({ json: { code: 0, data: { dimensionNo: 'quantity-dimension', pricingMode: mode }, message: 'success' } });
      });
      await adminPage.route('**/billing-rules/*/price-schedules', route => {
        writes.push(route.request().postDataJSON());
        return route.fulfill({ json: { code: 0, data: {}, message: 'success' } });
      });
      const workspace = new CatalogWorkspacePage(adminPage);
      await workspace.goto(); await workspace.openProductDetail('product-manual-service');
      await workspace.openConfigurationPrice();
      const modal = adminPage.getByTestId('catalog-configuration-price-modal');
      await expect(modal).toContainText('数量收费方式');
      await workspace.selectConfigurationPriceMode(label);
      await expect(modal).toContainText(label);
      await workspace.setConfigurationPrice('2.00', '1.50');
      await workspace.addConfigurationPriceBand('100', '1.00', '0.80');
      await expect(modal).toContainText('第2档：数量超过');
      await adminPage.getByTestId("catalog-configuration-price-add-band").click();
      await expect(adminPage.getByTestId("catalog-configuration-price-minimum_2")).toHaveValue("200");
      await expect(modal).toContainText("移除最后一档");
      await workspace.removeLastConfigurationPriceBand();
      await expect(adminPage.getByTestId("catalog-configuration-price-minimum_2")).toHaveCount(0);
      await expect(adminPage.getByTestId("catalog-configuration-price-amount_1")).toHaveValue("1.00");
      await workspace.screenshot(`configuration-${mode}-bands`);
      await workspace.confirmConfigurationPrice();
      await expect(modal).toBeHidden();
      expect(writes[0]).toEqual({ candidateToken: 'quantity-token', pricingMode: mode });
      expect(writes[1]).toMatchObject({ currencyCode: 'CNY', configurationPriceDimensionNo: 'quantity-dimension', tiers: [
        { minimumQuantity: 0, maximumQuantity: 100, amountMinor: '200', renewalAmountMinor: '150', setupFeeMinor: '0' },
        { minimumQuantity: 100, maximumQuantity: null, amountMinor: '100', renewalAmountMinor: '80', setupFeeMinor: '0' },
      ] });
      await workspace.screenshot(`configuration-${mode}-saved`);
    });
  }

  test('TC-41j: 支持试用的商品设置时长和累计资格，失败保留并可重试', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    await adminPage.route('**/x/idc-core/api/v1/currencies', route => route.fulfill({ json: { code: 0, data: { currencies: ['CNY'] } } }));
    const writes: Record<string, unknown>[] = [];
    let saved: Record<string, unknown> | undefined;
    await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({ json: { code: 0, data: {
      product: { productNo: 'product-manual-service', nameZhCn: '云服务器试用', status: 'draft' },
      version: { versionNo: 'version-manual-service-1', status: 'draft', source: 'manual', versionNumber: 1 },
      billingRules: saved ? [saved] : [], prices: [], taxPolicies: [], inventory: { trackingEnabled: false }, fulfillmentBinding: null,
      fulfillmentPolicy: { strategyCode: 'payment_confirmed_plugin', trialSupported: true, quantityHandling: 'separately', capabilityZhCn: '云服务器开通与管理', capabilityEnUs: 'Cloud server' },
    } } }));
    await adminPage.route('**/admin/catalog/product-versions/*/billing-rules', route => {
      const input = route.request().postDataJSON(); writes.push(input);
      if (writes.length === 1) return route.fulfill({ json: { code: 500, data: null, message: '试用保存失败，请重试' } });
      saved = { ...input, billingRuleNo: 'billing-trial' };
      return route.fulfill({ json: { code: 0, data: saved } });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.openProductDetail('product-manual-service');
    await workspace.addTrialPeriod();
    const modal = adminPage.getByTestId('catalog-billing-modal');
    await expect(modal).toContainText('试用时长');
    await expect(modal).toContainText('每位客户累计可试用台数');
    await expect(modal).toContainText('从开通成功开始计时');
    await expect(modal).not.toContainText('每次最多续费期数');
    await workspace.setTrialDuration('3', '2');
    await workspace.screenshot('trial-period-form');
    await workspace.confirmBillingPeriod();
    await expect(modal).toBeVisible();
    await expect(adminPage.getByTestId('catalog-billing-count')).toHaveValue('3');
    await workspace.confirmBillingPeriod();
    await expect(modal).toBeHidden();
    expect(writes).toHaveLength(2);
    expect(writes[1]).toMatchObject({ chargeType: 'trial', periodUnit: 'day', periodCount: 3, trialQuantity: 2, pricePhase: 'new_purchase', minimumPurchaseCycles: 1, quantityMode: 'fixed' });
    expect(writes[1]).not.toHaveProperty('maximumRenewalCycles');
    await expect(adminPage.getByTestId('catalog-base-price-add-billing-trial')).toBeVisible();
    await workspace.addBasePrice('billing-trial', '0', '');
    await expect(adminPage.getByTestId('catalog-base-price-renewal')).toHaveCount(0);
    await expect(adminPage.getByTestId('catalog-base-price-amount')).toHaveValue('0');
    await expect(adminPage.getByTestId('catalog-base-price-amount')).toBeDisabled();
    await expect(adminPage.getByTestId('catalog-base-price-setup')).toBeDisabled();
    await expect(adminPage.getByTestId('catalog-base-price-modal')).toContainText('试用免费，无安装费');
    await workspace.screenshot('trial-price-form');
  });

  for (const [suffix, strategy] of [['k', 'manual'], ['l', 'payment_confirmed_plugin']] as const) {
  test(`TC-41${suffix}: ${strategy}未声明暂停恢复能力隐藏试用入口`, async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({ json: { code: 0, data: {
      product: { productNo: 'product-manual-service', nameZhCn: '待配置商品', status: 'draft' },
      version: { versionNo: 'version-manual-service-1', status: 'draft', source: 'manual', versionNumber: 1 },
      billingRules: [], prices: [], taxPolicies: [], inventory: { trackingEnabled: false }, fulfillmentBinding: null,
      fulfillmentPolicy: { strategyCode: strategy, trialSupported: false, quantityHandling: 'together', capabilityZhCn: '', capabilityEnUs: '' },
    } } }));
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.openProductDetail('product-manual-service');
    await workspace.selectEditSection('价格与税务');
    await expect(adminPage.getByTestId('catalog-billing-add')).toBeVisible();
    await expect(adminPage.getByTestId('catalog-trial-add')).toHaveCount(0);
    await workspace.screenshot(`trial-unavailable-${strategy}`);
  });
  }

});

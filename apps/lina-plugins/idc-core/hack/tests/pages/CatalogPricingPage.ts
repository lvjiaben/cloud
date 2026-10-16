import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import type { Page } from '@host-tests/support/playwright';
import { CatalogManagementPage } from './CatalogManagementPage';
import { SalesManagementPage } from './SalesManagementPage';

export class CatalogPricingPage extends SalesManagementPage {
  readonly catalog: CatalogManagementPage;
  constructor(private readonly browserPage: Page) { super(browserPage); this.catalog = new CatalogManagementPage(browserPage); }
  get prices() { return this.browserPage.getByTestId('prices-workspace'); }
  get priceAdd() { return this.browserPage.getByTestId('price-add'); }
  async openAgents(productNo: string, name: string) {
    await this.catalog.goto();
    const searched = this.waitForResponse(response => response.url().includes('/catalog/products') && new URL(response.url()).searchParams.get('namePrefix') === name);
    await this.catalog.searchNamePrefix(name);
    const result = await (await searched).json();
    if (result.code !== 0 || !result.data.products.some((row: { productNo: string }) => row.productNo === productNo)) throw new Error(`Catalog fixture missing: code=${result.code},total=${result.data?.total}`);
    const detailLoaded = this.browserPage.waitForResponse(response => response.url().includes(`/catalog/products/${productNo}`) && response.request().method() === 'GET');
    await this.browserPage.getByTestId(`catalog-product-detail-${productNo}`).click();
    const detailResult = await (await detailLoaded).json();
    if (detailResult.code !== 0) throw new Error(`Product edit failed: ${detailResult.code}`);
    if (detailResult.data.version.status !== 'draft') throw new Error(`Expected draft product version, got ${detailResult.data.version.status}`);
    await this.screenshot('product-edit');
    await this.catalog.selectEditSection('价格与税务');
    const loaded = this.browserPage.waitForResponse(response => response.url().includes('/sales-management/prices') && response.request().method() === 'GET', { timeout: 15_000 });
    await this.browserPage.getByTestId('catalog-edit-agent-prices').click();
    return (await loaded).json();
  }
  async openCustom() {
    await this.catalog.goto();
    const loaded = this.browserPage.waitForResponse(response => response.url().includes('/sales-management/prices') && response.request().method() === 'GET', { timeout: 15_000 });
    await this.browserPage.getByTestId('catalog-custom-prices-open').click(); return (await loaded).json();
  }
  async addFixed(amount: string, targetField: string, targetName: string, productName?: string) {
    await this.priceAdd.click(); await this.fill(targetField, targetName.split(' · #')[0]); await this.select(targetField, targetName);
    if (productName) { await this.fill('productNo', productName); await this.select('productNo', productName); }
    await this.select('term', 'CNY · 1 月 · 周期付费');
    await this.text('固定拿货价').click(); await this.fill('amount', amount);
  }
  async save(method: 'POST' | 'PUT') {
    const saved = this.waitForResponse(response => response.url().includes('/sales-management/prices') && response.request().method() === method);
    await this.button(/确\s*认/u).click(); return (await saved).json();
  }
  async edit(priceNo: string) { await this.browserPage.getByTestId(`price-edit-${priceNo}`).click(); }
  async searchPrices(value: string) {
    await this.prices.getByPlaceholder(/搜索客户名称或商品名称|搜索代理等级名称/).fill(value);
    const loaded = this.waitForResponse(response => response.url().includes('/sales-management/prices') && response.request().method() === 'GET' && new URL(response.url()).searchParams.get('search') === value);
    await this.prices.getByRole('button', { name: /搜\s*索/u }).click(); return (await loaded).json();
  }
  async remove(priceNo: string) {
    await this.browserPage.getByTestId(`price-delete-${priceNo}`).click();
    const result = this.waitForResponse(response => response.url().includes('/sales-management/prices') && response.request().method() === 'DELETE');
    await this.browserPage.locator('.ant-popover:visible').getByRole('button', { name: /确\s*定|删\s*除/u }).click();
    return (await result).json();
  }
  async screenshot(name: string) {
    const date = new Date().toISOString().slice(0, 10).replaceAll('-', '');
    const directory = path.resolve(import.meta.dirname, `../../../../../../temp/${date}`); await mkdir(directory, { recursive: true });
    await this.browserPage.screenshot({ path: path.join(directory, `${Date.now()}-catalog-prices-${name}.png`), animations: 'disabled' });
  }
}

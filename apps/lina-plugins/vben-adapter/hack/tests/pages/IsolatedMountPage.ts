import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import type { Page } from '@host-tests/support/playwright';
import { MainLayout } from '@host-tests/pages/MainLayout';

/** Exercises the shipped mount contract inside the real, initialized host.
 * The fixture container does not replace or restyle the host application.
 */
export class IsolatedMountPage {
  constructor(readonly page: Page) {}
  get root() { return this.page.getByTestId('isolated-mount-contract'); }
  get rows() { return this.root.locator('.vxe-table--main-wrapper .vxe-body--row'); }
  get headers() { return this.root.locator('.vxe-table--main-wrapper .vxe-header--row'); }

  async mount(asset: string, routePath: string, locale = 'zh-CN', messages: Record<string, string> = {}) {
    await this.page.goto('http://127.0.0.1:6031/admin/idc/resource/provider-capacity', { waitUntil: 'domcontentloaded' });
    await this.page.locator('.vxe-grid').first().waitFor({ state: 'visible' });
    await new MainLayout(this.page).prepareThemeModeWithoutAnimation('light');
    await this.page.evaluate(async ({ asset, routePath, locale, messages }) => {
      const root = document.createElement('div');
      root.dataset.testid = 'isolated-mount-contract';
      root.style.cssText = 'position:fixed;inset:100px 16px 16px 220px;z-index:500;overflow:auto;background:hsl(var(--background-deep))';
      document.body.append(root);
      const module = await import(/* @vite-ignore */ asset);
      const context = { accessToken: '', assetURL: asset, baseURL: location.origin, container: root, locale, messages, query: {}, route: { path: routePath }, routePath, t: (key: string, fallback?: string) => messages[key] ?? fallback ?? key, title: '' };
      (window as any).__isolatedMountContract = { context, instance: module.mount(context) };
    }, { asset, routePath, locale, messages });
    await this.root.locator('.vxe-grid').first().waitFor({ state: 'visible' });
  }

  async setLocale(locale: string, messages: Record<string, string> = {}) {
    await this.page.evaluate(({ locale, messages }) => {
      const fixture = (window as any).__isolatedMountContract;
      fixture.context = { ...fixture.context, locale, messages, t: (key: string, fallback?: string) => messages[key] ?? fallback ?? key };
      fixture.instance.update(fixture.context);
    }, { locale, messages });
  }

  async click(name: string) { await this.root.getByRole('button', { name, exact: true }).last().click(); }
  async whiteSurfaces() {
    await this.page.mouse.move(0, 0);
    return this.root.locator('.vxe-grid').evaluateAll(grids => grids.flatMap(grid => {
      function color(element: Element | null): string {
        while (element) {
          const value = getComputedStyle(element).backgroundColor;
          if (value !== 'transparent' && value !== 'rgba(0, 0, 0, 0)') return value;
          element = element.parentElement;
        }
        return 'transparent';
      }
      return [grid, ...grid.querySelectorAll('.vxe-body--column,.vxe-table--empty-block')]
        .filter(element => element.getBoundingClientRect().height > 0).map(color);
    }));
  }

  async screenshot(name: string) {
    await this.page.evaluate(async () => { await Promise.all(document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(() => undefined))); });
    const now = new Date();
    const stamp = now.toISOString().replace(/[-:.TZ]/g, '');
    const folder = resolve(process.cwd(), '../../temp', stamp.slice(0, 8));
    await mkdir(folder, { recursive: true });
    await this.page.screenshot({ path: resolve(folder, `${stamp.slice(8)}-${name}.png`), fullPage: false });
  }

  async dispose() {
    await this.page.evaluate(() => {
      (window as any).__isolatedMountContract?.instance.unmount();
      document.querySelector('[data-testid="isolated-mount-contract"]')?.remove();
      delete (window as any).__isolatedMountContract;
    });
  }
}

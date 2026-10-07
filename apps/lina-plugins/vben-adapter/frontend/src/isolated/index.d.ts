import type { IsolatedTableHandle, IsolatedTableOptions, IsolatedFormDrawerHandle, IsolatedFormDrawerOptions, IsolatedEchartsHandle, IsolatedEchartsOptions } from './types';
export type { IsolatedTableColumn, IsolatedTableHandle, IsolatedTableOptions, IsolatedTablePagination, IsolatedFormField, IsolatedFormDrawerHandle, IsolatedFormDrawerOptions, IsolatedEchartsHandle, IsolatedEchartsOptions, IsolatedEchartsOption } from './types';
/** Owns only its container. Styles are a separate opt-in import. */
export declare function mountIsolatedVbenTable<Row extends Record<string, unknown>>(
  container: HTMLElement,
  options: IsolatedTableOptions<Row>,
): IsolatedTableHandle<Row>;
/** Uses the same isolated Vue runtime; business requests stay with the caller. */
export declare function mountIsolatedVbenFormDrawer(container: HTMLElement, options: IsolatedFormDrawerOptions): IsolatedFormDrawerHandle;
/** Owns only its container element and one ECharts instance. */
export declare function mountIsolatedVbenEcharts(container: HTMLElement, options: IsolatedEchartsOptions): IsolatedEchartsHandle;

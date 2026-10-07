import { setupVbenVxeTable, useVbenVxeGrid } from '@vben/plugins/vxe-table';
import { setupVbenForm, useVbenForm } from '@vben-core/form-ui';
import { createApp, h, nextTick, shallowRef } from 'vue';
import { Button } from 'ant-design-vue';
import type { IsolatedTableColumn, IsolatedTableOptions, IsolatedTableHandle } from './types';
import { initializeIsolatedVben, setIsolatedVbenContext } from './context';
import { isolatedFormSchema } from './form-schema';
import '@vben-core/design';
import 'vxe-table/lib/style.css';
import './vxe-components.scss';
import './table.css';

// Vben search fields also generate IDs inside separately mounted Vue apps.
let tableInstance = 0;

setupVbenForm({});
setupVbenVxeTable({
  useVbenForm,
  configVxeTable: (ui) => ui.setConfig({ grid: { border: 'inner', minHeight: 180 } }),
});

export { mountIsolatedVbenFormDrawer } from './form-drawer';
export { mountIsolatedVbenEcharts } from './echarts';

export function mountIsolatedVbenTable<Row extends Record<string, unknown>>(
  container: HTMLElement,
  initial: IsolatedTableOptions<Row>,
): IsolatedTableHandle<Row> {
  const options = shallowRef(initial);
  let gridApi: ReturnType<typeof useVbenVxeGrid>[1] | undefined;
  let destroyed = false;
  let sourceRows = new Map<unknown, Row>(initial.rows.map(row => [row[initial.rowKey], row]));
  function columns() {
    return options.value.columns.map((column: IsolatedTableColumn<Row>) => {
      const { cell, ...definition } = column;
      return { ...definition, ...(cell ? { slots: { default: `cell-${column.field}` } } : {}) };
    });
  }
  function gridOptions() {
    return { columns: columns(), data: options.value.rows, loading: options.value.loading, emptyText: options.value.emptyText, height: options.value.height ?? 480, rowConfig: { keyField: options.value.rowKey, isHover:true }, showOverflow: false, pagerConfig: pagerConfig(), fit: true, border: 'inner' as const };
  }
  function pagerConfig() {
    const pagination = options.value.pagination;
    // Official Vben owns layouts and mobile behavior. The business owner alone
    // supplies the exact total/page and loads records after this grid's event.
    return pagination ? { enabled:true, currentPage:pagination.currentPage, pageSize:pagination.pageSize, total:pagination.total, pageSizes:[10,20,50,100] } : { enabled:false };
  }
  function searchFormOptions() {
    const search = options.value.search;
    if (!search?.fields.length) return undefined;
    return {
      schema: isolatedFormSchema(search).map(field => ({ ...field, defaultValue: search.values?.[field.fieldName] })),
      submitOnChange: false,
      handleSubmit: async (values: Record<string, unknown>) => {
        if (!destroyed && options.value.search?.fields.length) await options.value.search.onSubmit(values);
      },
      handleReset: async () => {
        if (destroyed || !gridApi) return;
        await gridApi.formApi.resetForm();
        if (destroyed || !options.value.search?.fields.length) return;
        const values = await gridApi.formApi.getValues();
        if (!destroyed) await options.value.search?.onSubmit(values);
      },
    };
  }
  const cells = new WeakMap<HTMLElement, { row: Row; render: IsolatedTableColumn<Row>['cell']; locale?: string }>();
  const app = createApp({
    setup() {
      const [Grid, api] = useVbenVxeGrid<Record<string, any>>({ formOptions: searchFormOptions(), gridOptions: gridOptions(), gridEvents: { pageChange: ({ currentPage, pageSize }: {currentPage:number;pageSize:number}) => options.value.pagination?.onChange({currentPage,pageSize}) }, class: 'vben-isolated-table' });
      gridApi = api;
      return () => h(Grid, { tableTitle: options.value.tableTitle }, { ...Object.fromEntries(options.value.columns.filter((column) => column.cell).map((column) => [
        `cell-${column.field}`,
        ({ row }: { row: Row }) => h('div', { class: 'vben-isolated-cell', ref: (element) => {
          if (!(element instanceof HTMLElement)) return;
          // Vben merges/clones its internal grid state when loading changes.
          // Keep cells keyed to the caller's actual record, not that UI clone.
          const sourceRow = sourceRows.get(row[options.value.rowKey]) ?? row;
          const previous = cells.get(element);
          if (previous?.row === sourceRow && previous.render === column.cell && previous.locale === options.value.locale) return;
          const result = column.cell!(sourceRow);
          element.replaceChildren(...(Array.isArray(result) ? result : [result]));
          cells.set(element, { row:sourceRow, render: column.cell, locale: options.value.locale });
        } }),
      ])), ...(options.value.actions?.length ? { 'toolbar-actions': () => options.value.actions!.map(action => h(Button, {
        key: action.key, 'data-action': action.key, disabled: action.disabled, loading: action.loading, type: 'primary',
        onClick: () => { if (!destroyed && !action.disabled && !action.loading) return action.onClick(); },
      }, { default: () => action.label })) } : {}) });
    },
  });
  app.config.idPrefix = `vben-isolated-table-${++tableInstance}`;
  const ready = initializeIsolatedVben(app, initial.locale ?? 'zh-CN').then(() => {
    if (!destroyed) app.mount(container);
  });
  return {
    ready,
    update(next) {
      if (destroyed) return;
      const previous = options.value;
      options.value = { ...options.value, ...next };
      if ((next.rows && next.rows !== previous.rows) || (next.rowKey && next.rowKey !== previous.rowKey)) {
        sourceRows = new Map(options.value.rows.map(row => [row[options.value.rowKey], row]));
      }
      const updates: Record<string, unknown> = {};
      if (next.columns && next.columns !== previous.columns) updates.columns = columns();
      if (next.rows && next.rows !== previous.rows) updates.data = next.rows;
      for (const key of ['height', 'loading', 'emptyText'] as const) {
        if (key in next && next[key] !== previous[key]) updates[key] = next[key];
      }
      if (next.rowKey && next.rowKey !== previous.rowKey) updates.rowConfig = {keyField:next.rowKey};
      // Always accept the caller's committed page, including a same-reference
      // rollback after a failed request. Never synthesize a total from rows.
      if ('pagination' in next) updates.pagerConfig = pagerConfig();
      if (Object.keys(updates).length) gridApi?.setGridOptions(updates);
      if ('search' in next) {
        gridApi?.setState({ formOptions: searchFormOptions(), showSearchForm: Boolean(options.value.search?.fields.length) });
        const search = options.value.search;
        if (search?.fields.length && search.values !== previous.search?.values) {
          void nextTick().then(async () => {
            if (!destroyed && options.value.search === search) await gridApi?.formApi.resetForm({ values: search.values ?? {} }, { force: true });
          });
        }
      }
      if (next.locale) void setIsolatedVbenContext(next.locale);
    },
    destroy() { destroyed = true; if (gridApi) app.unmount(); },
  };
}

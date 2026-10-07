import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { createContext, SourceTextModule, SyntheticModule } from 'node:vm';
import ts from 'typescript';

// Execute the production adapter, replacing only Vue/Vben/DOM boundaries.
// These tests verify adapter callbacks and lifecycle, not browser rendering.
async function fixture() {
  const apps = [];
  const grids = [];
  const forms = [];
  const drawers = [];
  const events = [];
  class Element {
    children = [];
    append(...children) { this.children.push(...children); }
    replaceChildren(...children) { this.children = children; }
    remove() { this.removed = true; }
  }
  const context = createContext({ HTMLElement: Element, document: { createElement: () => new Element() }, URL });
  let resolveInitialization;
  const initialization = new Promise(resolve => { resolveInitialization = resolve; });
  function formApi(initial = {}) {
    return {
      values: { ...initial }, defaults: { ...initial }, resets: [],
      async getValues() { events.push('values'); return { ...this.values }; },
      async resetForm(state) {
        events.push('reset');
        this.resets.push(state);
        if (state?.values) this.defaults = { ...state.values };
        this.values = { ...this.defaults };
      },
      async validate() { return { valid: true }; },
      setState(state) { this.state = state; },
    };
  }
  const stubExports = {
    vue: {
      shallowRef: value => ({ value }),
      h: (component, props, children) => ({ component, props, children }),
      nextTick: () => Promise.resolve(),
      createApp(definition) {
        const app = {
          config: {},
          mount(container) { this.container = container; this.render = definition.setup(); this.mounted = true; },
          unmount() { this.unmounted = true; },
        };
        apps.push(app);
        return app;
      },
    },
    '@vben/plugins/vxe-table': {
      setupVbenVxeTable() {},
      useVbenVxeGrid(options) {
        const api = {
          formApi: formApi(Object.fromEntries((options.formOptions?.schema ?? []).map(field => [field.fieldName, field.defaultValue]))),
          states: [], grids: [],
          setState(value) { this.states.push(value); },
          setGridOptions(value) { this.grids.push(value); },
        };
        grids.push({ options, api });
        return ['Grid', api];
      },
    },
    '@vben-core/form-ui': {
      setupVbenForm() {},
      z: { any: () => ({ refine() { return this; } }) },
      useVbenForm(options) { const api = formApi(); forms.push({ options, api }); return ['Form', api]; },
    },
    '@vben-core/popup-ui': {
      useVbenDrawer(options) {
        const api = {
          open() { options.onOpenChange(true); },
          async close() { options.onOpenChange(false); options.onClosed(); },
          setState(state) { this.state = state; }, lock() {}, unlock() {},
        };
        drawers.push({ options, api });
        return ['Drawer', api];
      },
    },
    '@vben-core/shared/global-state': { globalShareState: { setComponents() {} } },
    '@vben/locales': { $t: key => key },
    'ant-design-vue': Object.fromEntries(['Alert', 'Button', 'Input', 'InputNumber', 'Select', 'Switch', 'DatePicker', 'TreeSelect', 'Cascader', 'ConfigProvider'].map(key => [key, key === 'Input' ? { Password: 'InputPassword', TextArea: 'Textarea' } : key])),
    './context': { initializeIsolatedVben: () => initialization, setIsolatedVbenContext: async () => {} },
    './echarts': undefined,
    '@linapro-isolated/echarts-instance': { default: { init: () => ({ setOption() {}, resize() {}, dispose() {} }) } },
  };
  const modules = new Map();
  async function load(specifier) {
    if (modules.has(specifier)) return modules.get(specifier);
    let module;
    if (['./runtime', './form-drawer', './form-schema', './echarts'].includes(specifier)) {
      const source = readFileSync(new URL(`${specifier}.ts`, import.meta.url), 'utf8');
      const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } }).outputText;
      module = new SourceTextModule(code, { context });
    } else {
      const values = stubExports[specifier] ?? {};
      assert.ok(specifier in stubExports || /\.(scss|css)$/.test(specifier) || specifier === '@vben-core/design', `unexpected import ${specifier}`);
      module = new SyntheticModule(Object.keys(values), function () { for (const [key, value] of Object.entries(values)) this.setExport(key, value); }, { context });
    }
    modules.set(specifier, module);
    await module.link(load);
    return module;
  }
  const runtime = await load('./runtime');
  await runtime.evaluate();
  return {
    api: runtime.namespace, apps, grids, forms, drawers, events,
    element: () => new Element(), initialize: resolveInitialization,
    drawerChildren: () => apps[0].render().children.default().children.default(),
  };
}

const baseTable = () => ({ columns: [{ field: 'name', title: 'Name' }], rows: [], rowKey: 'id' });
const baseDrawer = () => ({ title: 'Details', fields: [], values: {}, readOnly: true });

test('independent tables and nested drawers receive distinct Vue-generated ID namespaces', async () => {
  const f = await fixture();
  const first = f.api.mountIsolatedVbenFormDrawer(f.element(), baseDrawer());
  const second = f.api.mountIsolatedVbenFormDrawer(f.element(), baseDrawer());
  const table = f.api.mountIsolatedVbenTable(f.element(), baseTable());
  f.initialize();
  await Promise.all([first.ready, second.ready, table.ready]);
  const prefixes = f.apps.map(app => app.config.idPrefix);
  assert.ok(prefixes.every(prefix => typeof prefix === 'string' && prefix.length > 0));
  assert.equal(new Set(prefixes).size, 3);
  first.destroy(); second.destroy(); table.destroy();
});

test('drawer secondary actions use official buttons and current unsaved form values, but never run after closing', async () => {
  const f = await fixture();
  const values = [];
  const drawer = f.api.mountIsolatedVbenFormDrawer(f.element(), { title: 'Edit', fields: [{ name: 'price', label: 'Price', component: 'Input' }], values: { price: '1.005' }, actions: [{ key: 'add', label: 'Add', onClick: value => values.push(value) }] });
  f.initialize();
  await drawer.ready;
  f.forms[0].api.values.price = '12.345';
  const button = f.drawerChildren()[3].children[0];
  assert.equal(button.component, 'Button');
  await button.props.onClick();
  assert.equal(values[0].price, '12.345');
  drawer.close();
  await button.props.onClick();
  assert.equal(values.length, 1);
  drawer.destroy();
});

test('official table search preserves field values, resets before submit, and retains exact pagination', async () => {
  const f = await fixture();
  const submitted = [];
  const pages = [];
  const table = f.api.mountIsolatedVbenTable(f.element(), {
    ...baseTable(),
    pagination: { currentPage: 3, pageSize: 20, total: 137, onChange: value => pages.push(value) },
    search: { fields: [{ name: 'name', label: 'Name', component: 'Input' }], values: { name: 'initial' }, onSubmit: value => submitted.push(value) },
  });
  f.initialize();
  await table.ready;
  const grid = f.grids[0];
  assert.equal(grid.options.formOptions.schema[0].component, 'Input');
  assert.equal(grid.options.formOptions.schema[0].defaultValue, 'initial');
  assert.equal(grid.options.gridOptions.pagerConfig.total, 137);
  await grid.options.formOptions.handleSubmit({ name: 'query' });
  assert.equal(submitted[0].name, 'query');
  grid.api.formApi.values = { name: 'edited' };
  await grid.options.formOptions.handleReset();
  assert.deepEqual(f.events, ['reset', 'values']);
  assert.equal(submitted[1].name, 'initial');
  grid.options.gridEvents.pageChange({ currentPage: 4, pageSize: 20 });
  assert.equal(pages[0].currentPage, 4);
  assert.equal(grid.api.grids.length, 0, 'search does not fabricate a page or request');
  table.destroy();
  await grid.options.formOptions.handleSubmit({ name: 'late' });
  assert.equal(submitted.length, 2);
});

test('table toolbar actions use official buttons and stop after destruction', async () => {
  const f = await fixture();
  let calls = 0;
  const table = f.api.mountIsolatedVbenTable(f.element(), { ...baseTable(), actions: [{ key: 'import', label: 'Import', onClick: () => { calls++; } }] });
  f.initialize(); await table.ready;
  const button = f.apps[0].render().children['toolbar-actions']()[0];
  assert.equal(button.component, 'Button');
  assert.equal(button.children.default(), 'Import');
  button.props.onClick(); assert.equal(calls, 1);
  table.destroy(); button.props.onClick(); assert.equal(calls, 1);
});

test('empty search has no form, while updates switch to the latest owner callback and values', async () => {
  const f = await fixture();
  const submitted = [];
  const table = f.api.mountIsolatedVbenTable(f.element(), { ...baseTable(), search: { fields: [], onSubmit() {} } });
  f.initialize();
  await table.ready;
  const grid = f.grids[0];
  assert.equal(grid.options.formOptions, undefined);
  const search = { fields: [{ name: 'name', label: 'Name', component: 'Input' }], values: { name: 'updated' }, onSubmit: value => submitted.push(value) };
  table.update({ search });
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(grid.api.states.at(-1).showSearchForm, true);
  assert.equal(grid.api.formApi.values.name, 'updated');
  await grid.api.states.at(-1).formOptions.handleSubmit({ name: 'new' });
  assert.equal(submitted[0].name, 'new');
  table.update({ search: undefined });
  assert.equal(grid.api.states.at(-1).showSearchForm, false);
  assert.equal(grid.api.states.at(-1).formOptions, undefined);
});

test('drawer mounts owner DOM once and cleans on replacement, removal, close, and destroy', async () => {
  const f = await fixture();
  let firstMounts = 0;
  let firstCleanups = 0;
  let secondMounts = 0;
  let secondCleanups = 0;
  const first = container => { firstMounts++; container.append('owner DOM'); return () => firstCleanups++; };
  const second = container => { secondMounts++; container.append('replacement DOM'); return () => secondCleanups++; };
  const drawer = f.api.mountIsolatedVbenFormDrawer(f.element(), { ...baseDrawer(), mountContent: first });
  f.initialize();
  await drawer.ready;
  const children = f.drawerChildren();
  assert.equal(children[1], null, 'empty fields must not mount a placeholder form');
  assert.equal(f.forms[0].api.resets.length, 0);
  const element = f.element();
  const ref = children[2].props.ref;
  ref(element);
  ref(element);
  drawer.update({ title: 'Changed', mountContent: first });
  assert.equal(firstMounts, 1);
  drawer.update({ mountContent: second });
  assert.equal(firstCleanups, 1);
  assert.equal(secondMounts, 1);
  assert.deepEqual(element.children, ['replacement DOM']);
  drawer.update({ mountContent: undefined });
  assert.equal(secondCleanups, 1);
  assert.deepEqual(element.children, []);
  drawer.update({ mountContent: second });
  assert.equal(secondMounts, 2);
  drawer.close();
  assert.equal(secondCleanups, 2);
  ref(element);
  drawer.destroy();
  assert.equal(secondMounts, 2);
  assert.equal(secondCleanups, 2, 'cleanup must not repeat after close');
  assert.ok(f.apps[0].unmounted);
});

test('drawer UI close and content ref removal both clean exactly once', async () => {
  const f = await fixture();
  let cleaned = 0;
  let mounted = 0;
  const drawer = f.api.mountIsolatedVbenFormDrawer(f.element(), { ...baseDrawer(), mountContent: () => { mounted++; return () => cleaned++; } });
  f.initialize();
  await drawer.ready;
  const ref = f.drawerChildren()[2].props.ref;
  ref(f.element());
  ref(null);
  assert.equal(cleaned, 1);
  ref(f.element());
  f.drawers[0].options.onOpenChange(false);
  assert.equal(cleaned, 2);
  ref(null);
  drawer.destroy();
  assert.equal(cleaned, 2);
  assert.equal(mounted, 2);
});

test('closed-before-ready drawer never mounts owner content, and existing forms still initialize values', async () => {
  const f = await fixture();
  let mounted = 0;
  const drawer = f.api.mountIsolatedVbenFormDrawer(f.element(), { ...baseDrawer(), mountContent: () => { mounted++; } });
  drawer.close();
  f.initialize();
  await drawer.ready;
  assert.equal(mounted, 0);
  assert.equal(f.apps[0].mounted, undefined);
  const formDrawer = f.api.mountIsolatedVbenFormDrawer(f.element(), { title: 'Edit', fields: [{ name: 'count', label: 'Count', component: 'InputNumber' }], values: { count: 0 } });
  await formDrawer.ready;
  assert.equal(f.forms[0].api.values.count, 0);
  formDrawer.destroy();
});

import { createApp, h, nextTick, shallowRef } from 'vue';
import { setupVbenForm, useVbenForm, z } from '@vben-core/form-ui';
import { useVbenDrawer } from '@vben-core/popup-ui';
import { globalShareState } from '@vben-core/shared/global-state';
import { $t } from '@vben/locales';
import { Alert, Button, Input, InputNumber, Select, Switch, DatePicker, TreeSelect, Cascader, ConfigProvider } from 'ant-design-vue';
import type { IsolatedFormDrawerHandle, IsolatedFormDrawerOptions } from './types';
import { initializeIsolatedVben, setIsolatedVbenContext } from './context';
import { isolatedFormSchema, isolatedFormValuePresent, isolatedFormURLValid } from './form-schema';

// Independent Vue apps share a document, so their generated ARIA IDs must not.
let drawerInstance = 0;

// Registration belongs to this bundle's isolated Vue/form state, never the host.
globalShareState.setComponents({ Input, InputNumber, Select, Switch, DatePicker, TreeSelect, Cascader, InputPassword: Input.Password, Textarea: Input.TextArea });
setupVbenForm({
  config: { baseModelPropName: 'value', modelPropNameMap: { Switch: 'checked' } },
  defineRules: {
    required: (value, _params, ctx) => isolatedFormValuePresent(value) || $t('ui.formRules.required', [ctx.label]),
  },
});

/** A real Vben form and drawer, with no business API or host state access. */
export function mountIsolatedVbenFormDrawer(container: HTMLElement, initial: IsolatedFormDrawerOptions): IsolatedFormDrawerHandle {
  const options = shallowRef(initial);
  let form: ReturnType<typeof useVbenForm>[1] | undefined;
  let drawer: ReturnType<typeof useVbenDrawer>[1] | undefined;
  let destroyed = false;
  let closed = false;
  let revision = 0;
  let applyingValues = false;
  const submitting = shallowRef(false);
  let contentElement: HTMLElement | undefined;
  let mountedContent: IsolatedFormDrawerOptions['mountContent'];
  let contentCleanup: (() => void) | undefined;
  function cleanupContent() {
    const cleanup = contentCleanup;
    contentCleanup = undefined;
    mountedContent = undefined;
    try { cleanup?.(); } finally { contentElement?.replaceChildren(); }
  }
  function syncContent() {
    const mount = options.value.mountContent;
    if (destroyed || closed || !contentElement || !mount || mountedContent === mount) return;
    mountedContent = mount;
    const cleanup = mount(contentElement);
    if (typeof cleanup === 'function') {
      if (destroyed || closed || mountedContent !== mount) cleanup();
      else contentCleanup = cleanup;
    }
  }
  function contentRef(element: unknown) {
    const next = element instanceof HTMLElement ? element : undefined;
    if (next !== contentElement) { cleanupContent(); contentElement = next; }
    syncContent();
  }
  const style = document.createElement('style');
  // Only this runtime's own drawer is affected; no host/body/root selectors.
  style.textContent = '.vben-isolated-form-drawer{width:min(960px,95vw)!important;max-width:95vw!important}.vben-isolated-form-drawer .ant-input-number,.vben-isolated-form-drawer .ant-select,.vben-isolated-form-drawer .ant-picker{width:100%}.vben-isolated-form-drawer .vben-isolated-drawer-header>div:first-child{min-width:0;flex-direction:column;align-items:flex-start;gap:4px}';
  container.append(style);
  const appRoot = document.createElement('div');
  container.append(appRoot);
  function schema() {
    return isolatedFormSchema(options.value).map((field, index) => ({ ...field,
      rules: !options.value.readOnly && options.value.fields[index]?.props?.type === 'url'
        ? z.any().refine(value => !field.rules || isolatedFormValuePresent(value), $t('ui.formRules.required', [field.label])).refine(isolatedFormURLValid, $t('ui.formRules.invalidURL'))
        : field.rules,
    }));
  }
  const app = createApp({ setup() {
    const [Form, api] = useVbenForm({ schema: schema(), showDefaultActions: false, wrapperClass: 'grid-cols-1', commonConfig: { labelWidth: 160 }, handleValuesChange: values => { if (!applyingValues && !destroyed && !closed) options.value.onValuesChange?.(values); } });
    form = api;
    const [Drawer, drawerApi] = useVbenDrawer({
      title: options.value.title, description: options.value.description, class: 'vben-isolated-form-drawer', headerClass: 'vben-isolated-drawer-header', destroyOnClose: true,
      showConfirmButton: !options.value.readOnly, showCancelButton: !options.value.readOnly,
      async onConfirm() {
        const current = revision;
        if (submitting.value || options.value.readOnly || !options.value.onSubmit) return;
        const result = await api.validate();
        if (!result.valid || submitting.value || destroyed || current !== revision) return;
        submitting.value = true;
        drawerApi.lock();
        try {
          const saved = await options.value.onSubmit(await api.getValues());
          if (!destroyed && current === revision && saved) await drawerApi.close();
        } catch (error) { if (!destroyed && current === revision) options.value.onError?.(error); }
        finally { submitting.value = false; if (!destroyed) drawerApi.unlock(); }
      },
      onOpenChange(open) { if (!open) { closed = true; revision++; cleanupContent(); if (options.value.fields.length) void api.resetForm(); } },
      onClosed() { if (!destroyed) options.value.onClosed?.(); },
    });
    drawer = drawerApi;
    return () => h(ConfigProvider, { getPopupContainer: (trigger?: HTMLElement) => trigger?.closest<HTMLElement>('.vben-isolated-form-drawer') ?? appRoot }, { default: () => h(Drawer, {}, { default: () => [
      options.value.errorText ? h(Alert, { type: 'error', showIcon: true, message: options.value.errorText, class: 'mb-4' }) : null,
      options.value.fields.length ? h(Form) : null,
      options.value.mountContent ? h('div', { class: 'vben-isolated-drawer-content', ref: contentRef }) : null,
      !options.value.readOnly && options.value.actions?.length ? h('div', { class: 'flex gap-2 my-3' }, options.value.actions.map(action => h(Button, {
        key: action.key, disabled: submitting.value || action.disabled,
        async onClick() {
          if (destroyed || closed || submitting.value || action.disabled || options.value.readOnly) return;
          try { const values = await api.getValues(); if (!destroyed && !closed && !submitting.value) await action.onClick(values); }
          catch (error) { if (!destroyed && !closed) options.value.onError?.(error); }
        },
      }, { default: () => action.label }))) : null,
    ] }) });
  } });
  app.config.idPrefix = `vben-isolated-drawer-${++drawerInstance}`;
  const ready = initializeIsolatedVben(app, initial.locale ?? 'zh-CN').then(async () => {
    if (destroyed || closed) return;
    app.mount(appRoot);
    drawer?.open();
    await nextTick();
    if (!destroyed && !closed && options.value.fields.length) {
      applyingValues = true;
      try { await form?.resetForm({ values: options.value.values }, { force: true }); } finally { applyingValues = false; }
    }
  });
  return {
    ready,
    update(next) {
      if (destroyed) return;
      const previousMount = options.value.mountContent;
      options.value = { ...options.value, ...next };
      if (previousMount !== options.value.mountContent) { cleanupContent(); syncContent(); }
      if (next.fields || 'readOnly' in next) form?.setState({ schema: schema() });
      if ('title' in next || 'description' in next || 'readOnly' in next) drawer?.setState({ title: options.value.title, description: options.value.description, showConfirmButton: !options.value.readOnly, showCancelButton: !options.value.readOnly });
      if ('values' in next && options.value.fields.length) { applyingValues = true; void form?.resetForm({ values: options.value.values }, { force: true }).finally(() => { applyingValues = false; }); }
      if (next.locale) setIsolatedVbenContext(next.locale);
    },
    close() { closed = true; revision++; cleanupContent(); if (drawer) void drawer.close(); else options.value.onClosed?.(); },
    destroy() { destroyed = true; revision++; cleanupContent(); if (form) app.unmount(); appRoot.remove(); style.remove(); },
  };
}

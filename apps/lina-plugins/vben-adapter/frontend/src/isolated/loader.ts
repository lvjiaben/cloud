import type { IsolatedTableHandle, IsolatedTableOptions, IsolatedFormDrawerHandle, IsolatedFormDrawerOptions, IsolatedEchartsHandle, IsolatedEchartsOptions } from './types';

// Plugin-owned, versioned asset: all consumers share one cached Vben runtime.
const runtimeUrl = '/x-assets/vben-adapter/v0.1.0/isolated/runtime.js';
let runtimeAttempt = 0;
let runtimePromise: Promise<typeof import('./runtime')> | undefined;

function loadRuntime() {
  if (!runtimePromise) {
    const url = runtimeAttempt ? `${runtimeUrl}?retry=${runtimeAttempt}` : runtimeUrl;
    runtimePromise = import(/* @vite-ignore */ url).catch(error => {
      runtimeAttempt++;
      runtimePromise=undefined;
      throw error;
    });
  }
  return runtimePromise;
}

export function mountIsolatedVbenTable<Row extends Record<string, unknown>>(
  container: HTMLElement,
  initial: IsolatedTableOptions<Row>,
): IsolatedTableHandle<Row> {
  let current = initial;
  let mounted: IsolatedTableHandle<Row> | undefined;
  let destroyed = false;
  const ready = loadRuntime().then(async (runtime) => {
    if (destroyed) return;
    mounted = runtime.mountIsolatedVbenTable(container, current) as IsolatedTableHandle<Row>;
    await mounted.ready;
  });
  return {
    ready,
    update(next) { current = { ...current, ...next }; mounted?.update(next); },
    destroy() { destroyed = true; mounted?.destroy(); },
  };
}

export function mountIsolatedVbenEcharts(container: HTMLElement, initial: IsolatedEchartsOptions): IsolatedEchartsHandle {
  let current = initial;
  let mounted: IsolatedEchartsHandle | undefined;
  let destroyed = false;
  const ready = loadRuntime().then((runtime) => {
    if (destroyed) return;
    mounted = runtime.mountIsolatedVbenEcharts(container, current);
    return mounted.ready;
  });
  return {
    ready,
    update(next) { current = { ...current, ...next }; mounted?.update(next); },
    resize() { mounted?.resize(); },
    destroy() { destroyed = true; mounted?.destroy(); },
  };
}

export function mountIsolatedVbenFormDrawer(container: HTMLElement, initial: IsolatedFormDrawerOptions): IsolatedFormDrawerHandle {
  let current = initial;
  let mounted: IsolatedFormDrawerHandle | undefined;
  let destroyed = false;
  let closed = false;
  const ready = loadRuntime().then(async runtime => {
    if (destroyed || closed) return;
    mounted = runtime.mountIsolatedVbenFormDrawer(container, current);
    await mounted.ready;
  });
  return {
    ready,
    update(next) { current = { ...current, ...next }; mounted?.update(next); },
    close() { closed = true; if (mounted) mounted.close(); else current.onClosed?.(); },
    destroy() { destroyed = true; mounted?.destroy(); },
  };
}

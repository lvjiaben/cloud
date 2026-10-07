import type { IsolatedEchartsHandle, IsolatedEchartsOptions } from './types';
// The @vben/plugins/echarts barrel re-exports only named members, so import
// the echarts instance module directly (aliased in vite.isolated.config.ts).
import echarts from '@linapro-isolated/echarts-instance';

// Owns only its container element and one chart instance. Dark mode follows
// the host body class at render time; theme stays the plugin owner's choice.
export function mountIsolatedVbenEcharts(container: HTMLElement, initial: IsolatedEchartsOptions): IsolatedEchartsHandle {
  let instance: ReturnType<typeof echarts.init> | undefined;
  let disposed = false;
  let lastOptions = initial;
  const render = () => {
    if (disposed || !instance) return;
    instance.setOption(lastOptions.option as never);
  };
  const resize = () => instance?.resize();
  const ready = Promise.resolve().then(() => {
    if (disposed) return;
    container.style.width = lastOptions.width ?? '100%';
    container.style.height = `${lastOptions.height ?? 300}px`;
    instance = echarts.init(container);
    render();
  });
  window.addEventListener('resize', resize);
  return {
    ready,
    update(next) {
      lastOptions = { ...lastOptions, ...next };
      if (next.height !== undefined) container.style.height = `${next.height}px`;
      if (next.width !== undefined) container.style.width = next.width;
      render();
      resize();
    },
    resize,
    destroy() {
      disposed = true;
      window.removeEventListener('resize', resize);
      instance?.dispose();
      instance = undefined;
    },
  };
}

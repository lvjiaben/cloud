import { onBeforeUnmount, onMounted, type ComponentPublicInstance } from 'vue';

type GridLayoutApi = {
  grid: { recalculate: (force?: boolean) => Promise<unknown> };
};

/** Keep bounded grids in sync when their search form or toolbar changes size. */
export function useAdminGridLayout(...grids: GridLayoutApi[]) {
  let layoutRoot: Element | ComponentPublicInstance | null = null;
  let resizeObserver: ResizeObserver | undefined;
  let mutationObserver: MutationObserver | undefined;
  let frame = 0;
  let disposed = false;
  const observed = new WeakSet<Element>();

  function recalculate() {
    if (disposed || frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      for (const api of grids) void api.grid?.recalculate?.(true);
    });
  }

  onMounted(() => {
    const value = layoutRoot;
    const root = value instanceof Element ? value : value?.$el;
    if (!(root instanceof HTMLElement)) return;
    resizeObserver = new ResizeObserver(recalculate);
    function observeChrome() {
      for (const element of [root, ...root.querySelectorAll('.vxe-grid--form-wrapper, .vxe-grid--toolbar-wrapper, .vxe-grid--pager-wrapper')]) {
        if (!observed.has(element)) {
          observed.add(element);
          resizeObserver?.observe(element);
        }
      }
    }
    observeChrome();
    mutationObserver = new MutationObserver(observeChrome);
    mutationObserver.observe(root, { childList: true, subtree: true });
    void document.fonts.ready.then(recalculate);
  });

  onBeforeUnmount(() => {
    disposed = true;
    resizeObserver?.disconnect();
    mutationObserver?.disconnect();
    cancelAnimationFrame(frame);
  });

  return (value: Element | ComponentPublicInstance | null) => { layoutRoot = value; };
}

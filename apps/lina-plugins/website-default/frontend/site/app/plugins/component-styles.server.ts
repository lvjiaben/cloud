// Ant Design Vue registers component CSS during rendering but does not emit it
// in SSR. Keep its cache on this request's Vue app and preserve style hashes so
// the browser can reuse the exact styles when it hydrates.
import { createCache, extractStyle } from 'ant-design-vue/es/_util/cssinjs';

export default defineNuxtPlugin((nuxtApp) => {
  const cache = createCache();
  nuxtApp.vueApp.config.globalProperties.__ANTDV_CSSINJS_CACHE__ = cache;

  nuxtApp.hook('app:rendered', ({ ssrContext }) => {
    if (!ssrContext) return;
    // This markup comes only from the component library, never remote HTML.
    // Retain its individual hashes, priorities and hydration cache-path marker.
    const styles = Array.from(extractStyle(cache).matchAll(/<style([^>]*)>([\s\S]*?)<\/style>/gu), (match) => {
      const attributes = Object.fromEntries(
        Array.from(match[1]!.matchAll(/(data-[a-z-]+)="([^"]*)"/gu), (attribute) => [attribute[1]!, attribute[2]!]),
      );
      return {
        ...attributes,
        key: `antd-ssr-${attributes['data-css-hash'] || 'cache-path'}`,
        innerHTML: match[2]!,
      };
    });
    ssrContext.head.push({ style: styles });
  });
});

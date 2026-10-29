// Preserve announcement formatting and images without executing stored HTML.
export function announcementHtml(content: string) {
  const document = new DOMParser().parseFromString(content, 'text/html');
  const allowed = new Set(['P', 'BR', 'DIV', 'SPAN', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'STRONG', 'B', 'EM', 'I', 'U', 'S', 'BLOCKQUOTE', 'UL', 'OL', 'LI', 'PRE', 'CODE', 'HR', 'IMG', 'A']);
  const removed = new Set(['SCRIPT', 'STYLE', 'IFRAME', 'OBJECT', 'EMBED', 'SVG', 'MATH', 'FORM', 'TEMPLATE']);
  function clean(parent: Element) {
    for (const node of Array.from(parent.children)) {
      if (removed.has(node.tagName)) { node.remove(); continue; }
      clean(node);
      if (!allowed.has(node.tagName)) { node.replaceWith(...Array.from(node.childNodes)); continue; }
      for (const attribute of Array.from(node.attributes)) {
        const permitted = (node.tagName === 'IMG' && ['src', 'alt', 'title'].includes(attribute.name)) || (node.tagName === 'A' && ['href', 'title'].includes(attribute.name));
        if (!permitted) node.removeAttribute(attribute.name);
      }
      for (const name of ['src', 'href']) {
        const value = node.getAttribute(name);
        if (!value) continue;
        const imageData = name === 'src' && /^data:image\/(?:png|jpeg|gif|webp|avif);base64,/i.test(value);
        try {
          const url = new URL(value, window.location.origin);
          if (!imageData && !['http:', 'https:'].includes(url.protocol)) node.removeAttribute(name);
        } catch { node.removeAttribute(name); }
      }
      if (node.tagName === 'A') { node.setAttribute('target', '_blank'); node.setAttribute('rel', 'noopener noreferrer'); }
    }
  }
  clean(document.body);
  return document.body.innerHTML;
}

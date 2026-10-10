import { Editor } from '@tiptap/vue-3';
import { describe, expect, it, vi } from 'vitest';

import { getExtensions } from './extensions';

vi.mock('#/locales', () => ({ $t: () => '请输入内容' }));

describe('tiptap 3 editor compatibility and content safety', () => {
  it('registers links and underlines once and preserves existing formatting', () => {
    const editor = new Editor({
      extensions: getExtensions(),
      content:
        '<p><strong>正文</strong><u>下划线</u><a href="https://example.com">链接</a></p>',
    });
    try {
      const names = editor.extensionManager.extensions.map((item) => item.name);
      expect(names.filter((name) => name === 'link')).toHaveLength(1);
      expect(names.filter((name) => name === 'underline')).toHaveLength(1);
      expect(editor.getHTML()).toContain('<strong>正文</strong>');
      expect(editor.getHTML()).toContain('<u>下划线</u>');
      expect(editor.getHTML()).toContain('href="https://example.com"');
    } finally {
      editor.destroy();
    }
  });

  it('silently synchronizes external content and removes executable HTML attributes', () => {
    const onUpdate = vi.fn();
    const editor = new Editor({ extensions: getExtensions(), onUpdate });
    try {
      editor.commands.setContent('<p>外部更新</p>', { emitUpdate: false });
      expect(editor.getText()).toBe('外部更新');
      expect(onUpdate).not.toHaveBeenCalled();
      editor.commands.setContent(
        '<p onclick="alert(1)">正文<script>alert(1)</script></p><img src="x" onerror="alert(1)"><a href="javascript:alert(1)">危险链接</a>',
      );
      expect(onUpdate).toHaveBeenCalledTimes(1);
      expect(editor.getHTML()).not.toMatch(
        /onclick|onerror|<script|javascript:/i,
      );
      expect(editor.getText()).toContain('正文');
    } finally {
      editor.destroy();
    }
  });
});

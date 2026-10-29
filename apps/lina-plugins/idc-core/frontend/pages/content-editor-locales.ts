import { i18n } from "@vben/locales";

// Supplement the public editor's missing messages at the plugin boundary.
// Existing host translations take precedence.
const editorMessages = {
  "en-US": {
    placeholder: "Enter content…",
    actions: {
      undo: "Undo",
      redo: "Redo",
      bold: "Bold",
      italic: "Italic",
      underline: "Underline",
      strike: "Strikethrough",
      heading1: "Heading 1",
      heading2: "Heading 2",
      heading3: "Heading 3",
      bulletList: "Bulleted List",
      orderedList: "Numbered List",
      blockquote: "Blockquote",
      codeBlock: "Code Block",
      horizontalRule: "Divider",
      image: "Image",
      link: "Link",
    },
    link: { title: "Insert Link", placeholder: "Please enter the link URL" },
    messages: { imageUploadFailed: "Image upload failed. Please try again." },
  },
  "zh-CN": {
    placeholder: "请输入正文…",
    actions: {
      undo: "撤销",
      redo: "重做",
      bold: "加粗",
      italic: "斜体",
      underline: "下划线",
      strike: "删除线",
      heading1: "标题1",
      heading2: "标题2",
      heading3: "标题3",
      bulletList: "无序列表",
      orderedList: "有序列表",
      blockquote: "引用",
      codeBlock: "代码块",
      horizontalRule: "分割线",
      image: "图片",
      link: "链接",
    },
    link: { title: "插入链接", placeholder: "请输入链接地址" },
    messages: { imageUploadFailed: "图片上传失败，请重试。" },
  },
};

export function registerContentEditorLocale(locale: string) {
  const source = locale.startsWith("zh")
    ? editorMessages["zh-CN"]
    : editorMessages["en-US"];
  function missingMessages(
    messages: Record<string, unknown>,
    prefix: string,
  ): Record<string, unknown> {
    return Object.fromEntries(
      Object.entries(messages).flatMap(([key, value]) => {
        const path = `${prefix}.${key}`;
        if (typeof value === "object" && value)
          return [
            [key, missingMessages(value as Record<string, unknown>, path)],
          ];
        return i18n.global.te(path, locale) ? [] : [[key, value]];
      }),
    );
  }
  i18n.global.mergeLocaleMessage(locale, {
    pages: { editor: missingMessages(source, "pages.editor") },
  });
}

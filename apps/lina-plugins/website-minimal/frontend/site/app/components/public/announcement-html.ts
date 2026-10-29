import sanitizeHtml from 'sanitize-html';
export function announcementHtml(content: string) {
  return sanitizeHtml(content, {
    allowedTags: [...sanitizeHtml.defaults.allowedTags, 'img'],
    allowedAttributes: { a: ['href', 'title'], img: ['src', 'alt', 'title'] },
    allowedSchemes: ['http', 'https'],
    allowProtocolRelative: false,
  });
}

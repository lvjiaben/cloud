import type { AddressInfo } from 'node:net';
import type { Page } from '@playwright/test';

import { createServer, type Server } from 'node:http';

type ContentType = 'announcement' | 'download' | 'knowledge' | 'news';

const definitions: Record<ContentType, { contentNo: string; publicPath: string }> = {
  announcement: {
    contentNo: 'platform-announcement',
    publicPath: '/pages/announcements/platform-announcement',
  },
  download: {
    contentNo: 'download-file',
    publicPath: '/pages/downloads/download-file',
  },
  knowledge: {
    contentNo: 'platform-help',
    publicPath: '/pages/knowledge/platform-help',
  },
  news: {
    contentNo: 'platform-update',
    publicPath: '/pages/news/platform-update',
  },
};

function content(type: ContentType) {
  const definition = definitions[type];
  return {
    body: '后台录入的 English + 中文正文，前台必须原样显示。',
    category:
      type === 'knowledge'
        ? undefined
        : type === 'announcement'
          ? { nameEnUS: 'Must stay hidden', nameZhCN: '公告不应显示的分类' }
          : { nameEnUS: 'Platform updates', nameZhCN: '平台动态' },
    contentNo: definition.contentNo,
    currentFileNo: type === 'download' ? 'current-file-1' : '',
    publicPath: definition.publicPath,
    publishedAt: 1_786_320_000_000,
    seoDescription: '后台填写的 SEO description',
    seoTitle: '后台填写的 SEO title',
    summary: '后台录入的 Mixed language summary。',
    title: '后台录入的 Platform Update 标题',
    type,
    updatedAt: 1_786_320_600_000,
  };
}

function response(body: unknown, status = 200) {
  return { body: JSON.stringify(body), status };
}

function contentTypeForNo(contentNo: string) {
  return (Object.entries(definitions) as Array<
    [ContentType, { contentNo: string }]
  >).find(([, definition]) => definition.contentNo === contentNo)?.[0];
}

export async function mockPublicContent(page: Page) {
  await page.route('**/x/idc-core/api/v1/public/content/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (
      request.method() === 'POST' &&
      url.pathname.endsWith('/public/content/items/download-file/download-tickets')
    ) {
      await route.fulfill({
        body: JSON.stringify({
          code: 0,
          data: { expiresAt: 1_786_320_900_000, token: 'download-token' },
        }),
        contentType: 'application/json',
        status: 200,
      });
      return;
    }
    if (url.pathname.endsWith('/public/content/items')) {
      const type = url.searchParams.get('type') as ContentType;
      const item = definitions[type] ? content(type) : undefined;
      await route.fulfill({
        body: JSON.stringify({
          code: 0,
          data: { items: item ? [item] : [], nextCursor: '' },
        }),
        contentType: 'application/json',
        status: 200,
      });
      return;
    }
    const detailMatch = url.pathname.match(/\/public\/content\/items\/([^/]+)$/u);
    const type = detailMatch?.[1]
      ? contentTypeForNo(decodeURIComponent(detailMatch[1]))
      : undefined;
    if (type) {
      await route.fulfill({
        body: JSON.stringify({ code: 0, data: { item: content(type) } }),
        contentType: 'application/json',
        status: 200,
      });
      return;
    }
    await route.fallback();
  });
}

export async function startContentMockServer() {
  const origin = process.env.NUXT_API_ORIGIN?.trim() || 'http://127.0.0.1:9999';
  const target = new URL(origin);
  const server = createServer((request, reply) => {
    const url = new URL(request.url || '/', origin);
    if (url.pathname.endsWith('/public/content/downloads/download-token')) {
      reply.writeHead(200, {
        'content-disposition': 'attachment; filename="lina-agent.iso"',
        'content-type': 'application/octet-stream',
      });
      reply.end('mock-download');
      return;
    }
    if (url.pathname.endsWith('/missing')) {
      const result = response({ code: 404, reason: 'content_not_found' }, 404);
      reply.writeHead(result.status, { 'content-type': 'application/json' }).end(result.body);
      return;
    }
    if (
      request.method === 'POST' &&
      url.pathname.endsWith('/public/content/items/download-file/download-tickets')
    ) {
      const result = response({
        code: 0,
        data: { expiresAt: 1_786_320_900_000, token: 'download-token' },
      });
      reply.writeHead(result.status, { 'content-type': 'application/json' }).end(result.body);
      return;
    }
    if (url.pathname.endsWith('/public/content/items')) {
      const type = url.searchParams.get('type') as ContentType;
      const item = definitions[type] ? content(type) : undefined;
      const result = response({
        code: 0,
        data: { items: item ? [item] : [], nextCursor: '' },
      });
      reply.writeHead(result.status, { 'content-type': 'application/json' }).end(result.body);
      return;
    }
    const detailMatch = url.pathname.match(/\/public\/content\/items\/([^/]+)$/u);
    if (detailMatch?.[1]) {
      const type = contentTypeForNo(decodeURIComponent(detailMatch[1]));
      if (type) {
        const result = response({ code: 0, data: { item: content(type) } });
        reply.writeHead(result.status, { 'content-type': 'application/json' }).end(result.body);
        return;
      }
    }
    reply.writeHead(404).end();
  });

  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(Number(target.port || 80), target.hostname, () => resolve());
  });
  return {
    address: server.address() as AddressInfo,
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
    server: server as Server,
  };
}

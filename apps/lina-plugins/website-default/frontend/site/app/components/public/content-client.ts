import { publicRequestHeaders, resolvePublicApiURL } from './request-context';

export type PublicContentKind =
  | 'announcements'
  | 'downloads'
  | 'knowledge'
  | 'news';

type PublicContentAPIType = 'announcement' | 'download' | 'knowledge' | 'news';

export interface PublicContentCategory {
  nameEnUS: string;
  nameZhCN: string;
}

interface PublicContentItemDTO {
  body?: string;
  category?: PublicContentCategory;
  contentNo: string;
  currentFileNo?: string;
  publicPath: string;
  publishedAt: number;
  seoDescription?: string;
  seoTitle?: string;
  summary?: string;
  title: string;
  type: PublicContentAPIType;
  updatedAt: number;
}

export interface PublicContentSummary {
  category?: PublicContentCategory;
  contentNo: string;
  publicPath: string;
  publishedAt: number;
  summary?: string;
  title: string;
  type: PublicContentAPIType;
  updatedAt: number;
}

export interface PublicContentDetail extends PublicContentSummary {
  bodyText: string;
  hasDownload: boolean;
  seoDescription?: string;
  seoTitle?: string;
}

export interface PublicContentPage {
  items: PublicContentSummary[];
  nextCursor: string;
}

export interface PublicDownloadTicket {
  expiresAt: number;
  token: string;
}

interface ResponseEnvelope<T> {
  code?: number | string;
  errorCode?: string;
  data?: T;
  message?: string;
  reason?: string;
}

export class PublicContentRequestError extends Error {
  constructor(
    readonly reason: string,
    readonly status: number,
  ) {
    super(reason);
  }
}

const base = '/x/idc-core/api/v1/public/content';

interface AnnouncementDTO { id: number; title: string; content: string; createdAt: number; updatedAt: number; sortOrder: number; }
const announcementBase = '/x/content-notice/api/v1/public/announcements';
function mapAnnouncement(item: AnnouncementDTO): PublicContentSummary {
  return { contentNo: String(item.id), publicPath: `/pages/announcements/${item.id}`, publishedAt: item.createdAt, title: item.title, type: 'announcement', updatedAt: item.updatedAt };
}

const kindToType: Record<PublicContentKind, PublicContentAPIType> = {
  announcements: 'announcement',
  downloads: 'download',
  knowledge: 'knowledge',
  news: 'news',
};

function mapSummary(item: PublicContentItemDTO): PublicContentSummary {
  return {
    category: item.category,
    contentNo: item.contentNo,
    publicPath: item.publicPath,
    publishedAt: item.publishedAt,
    summary: item.summary,
    title: item.title,
    type: item.type,
    updatedAt: item.updatedAt,
  };
}

export async function listPublicContent(
  kind: PublicContentKind,
  filters: { cursor?: string; query?: string } = {},
) {
  if (kind === 'announcements') {
    const pageNum = Number(filters.cursor || '1');
    const result = await requestContent<{ list: AnnouncementDTO[]; total: number }>(`${announcementBase}?pageNum=${pageNum}&pageSize=20`);
    return { items: (result.list || []).map(mapAnnouncement), nextCursor: pageNum * 20 < result.total ? String(pageNum + 1) : '' } satisfies PublicContentPage;
  }
  const query = new URLSearchParams({
    limit: '20',
    type: kindToType[kind],
  });
  if (filters.cursor) query.set('cursor', filters.cursor);
  if (filters.query) query.set('query', filters.query);
  const result = await requestContent<{
    items: PublicContentItemDTO[];
    nextCursor: string;
  }>(`${base}/items?${query}`);
  return {
    items: (result.items || []).map(mapSummary),
    nextCursor: result.nextCursor || '',
  } satisfies PublicContentPage;
}

export async function getPublicContent(
  kind: PublicContentKind,
  contentNo: string,
) {
  if (kind === 'announcements') {
    const item = await requestContent<AnnouncementDTO>(`${announcementBase}/${encodeURIComponent(contentNo)}`);
    return { ...mapAnnouncement(item), bodyText: item.content, hasDownload: false } satisfies PublicContentDetail;
  }
  const result = await requestContent<{ item: PublicContentItemDTO }>(
    `${base}/items/${encodeURIComponent(contentNo)}`,
  );
  if (!result.item || result.item.type !== kindToType[kind]) {
    throw new PublicContentRequestError('content_not_found', 404);
  }
  return {
    ...mapSummary(result.item),
    bodyText: result.item.body || '',
    hasDownload: Boolean(result.item.currentFileNo),
    seoDescription: result.item.seoDescription,
    seoTitle: result.item.seoTitle,
  } satisfies PublicContentDetail;
}

export function createPublicDownloadTicket(contentNo: string) {
  return requestContent<PublicDownloadTicket>(
    `${base}/items/${encodeURIComponent(contentNo)}/download-tickets`,
    { method: 'POST' },
  );
}

export function publicDownloadURL(token: string) {
  return `${base}/downloads/${encodeURIComponent(token)}`;
}

async function requestContent<T>(url: string, init: RequestInit = {}): Promise<T> {
  const controller = new AbortController();
  const timeout = globalThis.setTimeout(() => controller.abort(), 8_000);
  try {
    const response = await fetch(resolvePublicApiURL(url), {
      ...init,
      headers: {
        ...publicRequestHeaders(),
        Accept: 'application/json',
        ...init.headers,
      },
      signal: controller.signal,
    });
    const payload = (await response.json()) as ResponseEnvelope<T> | T;
    if ((payload as ResponseEnvelope<T>)?.errorCode === 'CONTENT_NOTICE_NOT_FOUND') throw new PublicContentRequestError('content_not_found', 404);
    if (!response.ok) {
      const envelope = payload as ResponseEnvelope<T>;
      throw new PublicContentRequestError(
        envelope.reason || 'content_request_failed',
        response.status,
      );
    }
    if (typeof payload === 'object' && payload !== null && 'data' in payload) {
      const envelope = payload as ResponseEnvelope<T>;
      if (
        envelope.data === undefined ||
        (envelope.code !== undefined && `${envelope.code}` !== '0')
      ) {
        throw new PublicContentRequestError(
          envelope.reason || 'content_request_failed',
          response.status,
        );
      }
      return envelope.data;
    }
    return payload as T;
  } catch (error) {
    if (error instanceof PublicContentRequestError) throw error;
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw new PublicContentRequestError('content_request_timeout', 504);
    }
    throw new PublicContentRequestError('content_request_failed', 503);
  } finally {
    globalThis.clearTimeout(timeout);
  }
}

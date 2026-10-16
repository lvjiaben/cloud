import type { Locale } from './public-i18n';

export interface ContentCopy {
  announcements: string;
  announcementsIntro: string;
  back: string;
  category: string;
  contentUnavailable: string;
  download: string;
  downloadChanged: string;
  downloads: string;
  downloadsIntro: string;
  empty: string;
  fileHash: string;
  fileSize: string;
  help: string;
  helpIntro: string;
  home: string;
  knowledge: string;
  knowledgeIntro: string;
  language: string;
  loadFailed: string;
  news: string;
  newsIntro: string;
  nextPage: string;
  pageNumber: string;
  previousPage: string;
  publishedAt: string;
  redownload: string;
  retry: string;
  search: string;
  searchPlaceholder: string;
  uncategorized: string;
}

export const contentCopy: Record<Locale, ContentCopy> = {
  'en-US': {
    announcements: 'Announcements',
    announcementsIntro: 'Important service and platform updates in one place.',
    back: 'Back to list',
    category: 'Category',
    contentUnavailable: 'This content is unavailable.',
    download: 'Download file',
    downloadChanged: 'The file has been updated. Download it again to get the current version.',
    downloads: 'Downloads',
    downloadsIntro: 'Official software, documents, and service resources.',
    empty: 'No published content is available.',
    fileHash: 'SHA-256',
    fileSize: 'File size',
    help: 'Help center',
    helpIntro: 'Search practical answers and step-by-step guidance.',
    home: 'Home',
    knowledge: 'Knowledge base',
    knowledgeIntro: 'Guides for purchasing, delivery, and everyday service use.',
    language: '切换到中文',
    loadFailed: 'Content is temporarily unavailable. Try again later.',
    news: 'News',
    newsIntro: 'Product updates, service improvements, and company news.',
    nextPage: 'Next',
    pageNumber: 'Page {page}',
    previousPage: 'Previous',
    publishedAt: 'Published',
    redownload: 'Download current file',
    retry: 'Try again',
    search: 'Search',
    searchPlaceholder: 'Search published help content',
    uncategorized: 'Uncategorized',
  },
  'zh-CN': {
    announcements: '公告',
    announcementsIntro: '集中查看重要的服务与平台更新。',
    back: '返回列表',
    category: '分类',
    contentUnavailable: '该内容不存在或已下线。',
    download: '下载文件',
    downloadChanged: '文件已更新，需要重新下载。',
    downloads: '下载中心',
    downloadsIntro: '获取官方软件、文档和服务资料。',
    empty: '暂时没有已发布内容。',
    fileHash: 'SHA-256',
    fileSize: '文件大小',
    help: '帮助中心',
    helpIntro: '搜索常见问题和一步一步的操作说明。',
    home: '首页',
    knowledge: '知识库',
    knowledgeIntro: '查看购买、交付和日常服务使用指南。',
    language: 'Switch to English',
    loadFailed: '暂时无法读取内容，请稍后重试。',
    news: '新闻',
    newsIntro: '了解产品更新、服务改进和公司动态。',
    nextPage: '下一页',
    pageNumber: '第 {page} 页',
    previousPage: '上一页',
    publishedAt: '发布时间',
    redownload: '重新下载当前文件',
    retry: '重试',
    search: '搜索',
    searchPlaceholder: '搜索已发布的帮助内容',
    uncategorized: '未分类',
  },
};

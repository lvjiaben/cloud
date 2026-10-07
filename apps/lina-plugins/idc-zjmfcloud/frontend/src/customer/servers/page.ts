import { businessLabel, copy, displayName, formatTime, normalizeLocale, parseServerDetail, parseServerPage, type Locale, type ServerItem } from './model.js';

const protocol = 'linapro.plugin-page-bridge';
const version = '1.0';
const pluginId = 'idc-zjmfcloud';
const surfaceKey = 'servers';
const capability = 'customer-servers.read';

type BridgeMessage = { capability?: unknown; direction?: unknown; ok?: unknown; payload?: any; pluginId?: unknown; protocol?: unknown; requestId?: unknown; result?: unknown; surfaceKey?: unknown; version?: unknown };

export function startCustomerServersPage(win: Window = window, doc: Document = document) {
  let locale: Locale = 'zh-CN';
  let initialized = false;
  let sequence = 0;
  let nextCursor = '';
  let items: ServerItem[] = [];
  let selectedServerNo = '';
  let listRequestId = '';
  let detailRequestId = '';

  const byTestId = (name: string) => doc.querySelector<HTMLElement>(`[data-testid="${name}"]`)!;
  const send = (sentCapability: string, method: string, payload: unknown = {}, requestId?: string) => {
    win.parent.postMessage({ protocol, version, direction: 'plugin-to-host', pluginId, surfaceKey, capability: sentCapability, method, payload, requestId }, '*');
  };
  const resize = () => send('host.view.v1', 'resize', { height: Math.ceil(doc.documentElement.scrollHeight) });
  const setVisible = (name: string, visible: boolean) => { byTestId(name).hidden = !visible; };
  const setText = (selector: string, value: string) => { const node = doc.querySelector<HTMLElement>(selector); if (node) node.textContent = value; };
  const bridgeRequest = (path: string, params: Record<string, unknown>, kind: 'detail' | 'list') => {
    sequence += 1;
    const requestId = `${kind}-${sequence}`;
    send(capability, 'request', { method: 'GET', path, params }, requestId);
    return requestId;
  };

  function applyCopy() {
    const words = copy[locale];
    doc.documentElement.lang = locale;
    doc.title = locale === 'en-US' ? 'Cloud servers' : '云服务器';
    byTestId('server-refresh').textContent = words.refresh;
    byTestId('server-retry').textContent = words.retry;
    byTestId('server-more').textContent = words.loadMore;
    byTestId('server-detail-close').textContent = words.close;
    byTestId('server-detail-retry').textContent = words.retry;
    byTestId('server-detail-title').textContent = words.detailTitle;
    setText('[data-testid="server-error"] [data-role="title"]', words.errorTitle);
    setText('[data-testid="server-error"] [data-role="description"]', words.errorDescription);
    setText('[data-testid="server-empty"] [data-role="title"]', words.emptyTitle);
    setText('[data-testid="server-empty"] [data-role="description"]', words.emptyDescription);
    setText('[data-testid="server-detail-error"] [data-role="description"]', words.errorDescription);
    renderList();
  }

  function renderList() {
    const list = byTestId('server-list');
    list.replaceChildren(...items.map((item) => {
      const row = doc.createElement('li');
      row.className = 'server-row';
      const product = doc.createElement('div'); product.className = 'product';
      const name = doc.createElement('strong'); name.textContent = displayName(item, locale);
      const kind = doc.createElement('span'); kind.textContent = businessLabel(item.kind, locale);
      product.append(name, kind);
      const location = doc.createElement('div'); location.className = 'location'; location.textContent = `${item.channelName} · ${item.poolName}`;
      const statusCell = doc.createElement('div'); statusCell.className = 'status-cell';
      const status = doc.createElement('span'); status.className = 'status'; status.textContent = businessLabel(item.state, locale); statusCell.append(status);
      const button = doc.createElement('button'); button.type = 'button'; button.className = 'button row-action'; button.textContent = copy[locale].view;
      button.dataset.serverNo = item.serverNo;
      button.addEventListener('click', () => loadDetail(item.serverNo));
      row.append(product, location, statusCell, button);
      return row;
    }));
    byTestId('server-summary').textContent = copy[locale].summary(items.length);
    setVisible('server-pager', nextCursor.length > 0);
    resize();
  }

  function renderDetail(item: ServerItem) {
    const words = copy[locale];
    const fields: Array<[string, string]> = [
      [words.product, displayName(item, locale)], [words.kind, businessLabel(item.kind, locale)],
      [words.location, `${item.channelName} · ${item.poolName}`], [words.state, businessLabel(item.state, locale)],
      [words.operation, businessLabel(item.latestOperation, locale)], [words.boundAt, formatTime(item.boundAt, locale)],
      [words.updatedAt, formatTime(item.updatedAt, locale)],
    ];
    const nodes = fields.map(([label, value]) => { const wrapper = doc.createElement('div'); const term = doc.createElement('dt'); const description = doc.createElement('dd'); term.textContent = label; description.textContent = value; wrapper.append(term, description); return wrapper; });
    byTestId('server-detail-grid').replaceChildren(...nodes);
    setVisible('server-detail-skeleton', false); setVisible('server-detail-error', false); setVisible('server-detail-grid', true); resize();
  }

  function loadList(replace = true) {
    if (listRequestId) return;
    if (replace) { setVisible('server-skeleton', true); setVisible('server-content', false); setVisible('server-empty', false); setVisible('server-error', false); }
    listRequestId = bridgeRequest('account/plugin-servers', { cursor: replace ? '' : nextCursor, limit: 20 }, 'list');
  }

  function loadDetail(serverNo: string) {
    if (!/^SVR-[0-9A-F]{32}$/.test(serverNo) || detailRequestId) return;
    selectedServerNo = serverNo;
    setVisible('server-detail', true); setVisible('server-detail-skeleton', true); setVisible('server-detail-grid', false); setVisible('server-detail-error', false);
    detailRequestId = bridgeRequest(`account/plugin-servers/${serverNo}`, {}, 'detail'); resize();
  }

  function handleListResponse(message: BridgeMessage) {
    const append = items.length > 0 && !byTestId('server-content').hidden;
    listRequestId = '';
    const page = message.ok === true ? parseServerPage(message.result) : null;
    if (!page) { if (!append) { setVisible('server-skeleton', false); setVisible('server-content', false); setVisible('server-empty', false); setVisible('server-error', true); } resize(); return; }
    items = append ? [...items, ...page.items] : page.items;
    nextCursor = page.nextCursor; setVisible('server-skeleton', false); setVisible('server-error', false);
    setVisible('server-empty', items.length === 0); setVisible('server-content', items.length > 0); renderList();
  }

  function handleDetailResponse(message: BridgeMessage) {
    detailRequestId = '';
    const item = message.ok === true ? parseServerDetail(message.result) : null;
    if (!item || item.serverNo !== selectedServerNo) { setVisible('server-detail-skeleton', false); setVisible('server-detail-grid', false); setVisible('server-detail-error', true); resize(); return; }
    renderDetail(item);
  }

  const handleMessage = (event: MessageEvent<BridgeMessage>) => {
    const message = event.data;
    if (event.source !== win.parent || message?.protocol !== protocol || message.direction !== 'host-to-plugin' || message.pluginId !== pluginId || message.surfaceKey !== surfaceKey) return;
    if (message.capability === 'host.context.v1') {
      const context = message.payload ?? {}; locale = normalizeLocale(context.locale); const theme = context.theme ?? {};
      doc.documentElement.dataset.theme = String(theme.mode ?? 'light');
      for (const [name, value] of Object.entries(theme.cssVariables ?? {})) if (typeof value === 'string') doc.documentElement.style.setProperty(name, value);
      if (theme.fontFamily) doc.body.style.fontFamily = String(theme.fontFamily);
      if (theme.fontSize) doc.body.style.fontSize = `${Number(theme.fontSize)}px`;
      applyCopy(); send('host.view.v1', 'set-title', { title: locale === 'en-US' ? 'Cloud servers' : '云服务器' });
      if (!initialized) { initialized = true; loadList(true); }
      resize(); return;
    }
    if (message.requestId === listRequestId) handleListResponse(message);
    if (message.requestId === detailRequestId) handleDetailResponse(message);
  };
  win.addEventListener('message', handleMessage);

  byTestId('server-refresh').addEventListener('click', () => { items = []; nextCursor = ''; loadList(true); });
  byTestId('server-retry').addEventListener('click', () => loadList(true));
  byTestId('server-more').addEventListener('click', () => loadList(false));
  byTestId('server-detail-close').addEventListener('click', () => { selectedServerNo = ''; setVisible('server-detail', false); resize(); });
  byTestId('server-detail-retry').addEventListener('click', () => { const value = selectedServerNo; detailRequestId = ''; if (value) loadDetail(value); });
  const resizeObserver = new ResizeObserver(resize);
  resizeObserver.observe(doc.body);
  applyCopy(); send('host.view.v1', 'ready'); resize();
  return () => { win.removeEventListener('message', handleMessage); resizeObserver.disconnect(); };
}

// Shared E2E runtime configuration.
// 共享 E2E 运行时配置。
//
// All hard-coded endpoints (frontend dev server, backend HTTP API) are
// centralized here so that test files only depend on this single source of
// truth. Override via environment variables when running tests against a
// non-default deployment:
//   - E2E_BASE_URL          admin frontend origin for browser tests (default 127.0.0.1:6031)
//   - E2E_BACKEND_BASE_URL  backend HTTP origin (default 127.0.0.1:6030)
//   - E2E_API_BASE_URL      backend `/api/v1/` base URL; overrides backendBaseURL when set
//   - E2E_PUBLIC_BASE_URL   backend public origin (no `/api/v1/` suffix); overrides backendBaseURL when set
//   - E2E_WORKSPACE_BASE_PATH  admin workspace browser entry path (default /admin)
//
// 所有端口（后端公共入口、后端 HTTP 接口）集中到此处，测试文件不应再单独
// 硬编码端口或回退地址。如需在非默认部署下运行测试，可通过上述环境变量覆盖。

const backendBaseURL = process.env.E2E_BACKEND_BASE_URL ?? 'http://127.0.0.1:6030';
const browserBaseURL = process.env.E2E_BASE_URL ?? 'http://127.0.0.1:6031';
const workspaceBasePath = normalizeWorkspaceBasePath(
  process.env.E2E_WORKSPACE_BASE_PATH ?? '/admin',
);

// Backend API base URL with the /api/v1/ prefix; trailing slash is required so
// relative paths like 'auth/login' resolve correctly under playwrightRequest.
// 后端 API 基础 URL，包含 /api/v1/ 前缀；保留末尾斜杠以便相对路径解析。
const apiBaseURL =
  process.env.E2E_API_BASE_URL ?? `${backendBaseURL.replace(/\/$/, '')}/api/v1/`;

// Public origin used when accessing non-/api/v1 routes on the backend (such as
// /x-assets/ or /api.json).
// 直接访问后端非 /api/v1 路径（例如 /x-assets/ 或 /api.json）时使用的源。
const publicBaseURL =
  process.env.E2E_PUBLIC_BASE_URL ?? apiBaseURL.replace(/\/api\/v1\/?$/, '');

// Origin observed by the backend for browser-context requests to backend-owned
// public routes such as /api.json. By default E2E enters through the backend
// public origin; override this only when running tests through a proxy that
// intentionally rewrites the Host header.
// 浏览器上下文请求 /api.json 等后端公共路由时，后端实际看到的 Host。默认
// E2E 通过后端公共入口访问；仅当测试环境存在会改写 Host 的代理时才覆盖。
const frontendProxyBackendOrigin =
  process.env.E2E_FRONTEND_PROXY_BACKEND_ORIGIN ?? backendBaseURL.replace(/\/$/, '');

export const config = {
  adminUser: process.env.E2E_ADMIN_USER ?? 'admin',
  adminPass: process.env.E2E_ADMIN_PASS ?? 'admin123',
  baseURL: browserBaseURL,
  backendBaseURL,
  apiBaseURL,
  publicBaseURL,
  frontendProxyBackendOrigin,
  workspaceBasePath,
};

export function workspacePath(path = '/') {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  if (normalizedPath === '/') {
    return workspaceBasePath;
  }
  if (workspaceBasePath === '/') {
    return normalizedPath;
  }
  return `${workspaceBasePath}${normalizedPath}`;
}

export function pluginApiPath(pluginId: string, path = '') {
  const normalizedPluginId = pluginId.trim().replace(/^\/+|\/+$/g, '');
  const normalizedPath = path.trim().replace(/^\/+/, '');
  const prefix = `/x/${normalizedPluginId}/api/v1`;
  return normalizedPath ? `${prefix}/${normalizedPath}` : prefix;
}

export function isWorkspaceManagedPath(path: string) {
  if (!path.startsWith('/')) {
    return false;
  }
  return [
    '/',
    '/about',
    '/auth',
    '/dashboard',
    '/dev',
    '/developer',
    '/monitor',
    '/platform',
    '/profile',
    '/system',
    '/tenant',
  ].some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}

function normalizeWorkspaceBasePath(value: string) {
  const cleaned = value.trim().replaceAll('\\', '/').replace(/\/+/g, '/');
  if (cleaned === '/') {
    return '/';
  }
  const normalized = cleaned.replace(/\/+$/, '');
  if (!normalized || !normalized.startsWith('/')) {
    return '/admin';
  }
  return normalized;
}

import type { Page, Route } from "@host-tests/support/playwright";

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

export const contentManagementNo = "CNT_DOWNLOAD_SECURITY_GUIDE";

export type ContentUploadPolicyMock = {
  inspectionRequiredAvailable: boolean;
  inspectionRequiredExtensions: string[];
  inspectionRequiredMaxSizeBytes: number;
  ordinaryExtensions: string[];
  ordinaryMaxSizeBytes: number;
};

export interface ContentManagementMockOptions {
  failUploadPolicy?: boolean;
  uploadPolicy?: Partial<ContentUploadPolicyMock>;
}

export interface ContentManagementMockState {
  policyRequests: number;
  uploadSessionRequests: Array<Record<string, unknown>>;
}

const defaultUploadPolicy: ContentUploadPolicyMock = {
  inspectionRequiredAvailable: true,
  inspectionRequiredExtensions: [".zip", ".7z", ".exe", ".msi", ".iso"],
  inspectionRequiredMaxSizeBytes: 1024,
  ordinaryExtensions: [
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".webp",
    ".bmp",
    ".pdf",
    ".doc",
    ".docx",
    ".xls",
    ".xlsx",
    ".ppt",
    ".pptx",
  ],
  ordinaryMaxSizeBytes: 5 * 1024 * 1024 * 1024,
};

function contentSummary() {
  return {
    availableActions: ["edit", "delete"],
    contentNo: contentManagementNo,
    createdAt: 1_786_406_400_000,
    publicPath: "/downloads/security-guide",
    status: "draft",
    summary: "用于验证上传前安全限制",
    title: "安全下载测试内容",
    type: "download",
    updatedAt: 1_786_410_000_000,
    version: 3,
    visibility: "public",
  };
}

function contentDetail() {
  return {
    ...contentSummary(),
    body: "选择文件前必须先读取当前实例的真实上传限制。",
    targetAudiences: [],
    targetCustomerGroupNos: [],
  };
}

export async function installContentManagementMock(
  page: Page,
  options: ContentManagementMockOptions = {},
): Promise<ContentManagementMockState> {
  const state: ContentManagementMockState = {
    policyRequests: 0,
    uploadSessionRequests: [],
  };
  const uploadPolicy = {
    ...defaultUploadPolicy,
    ...options.uploadPolicy,
  };
  let uploadSizeBytes = 0;

  await page.route(
    "**/x/idc-core/api/v1/admin/content/**",
    async (route: Route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;

      if (request.method() === "GET" && path.endsWith("/upload-policy")) {
        state.policyRequests += 1;
        if (options.failUploadPolicy) {
          await route.fulfill({
            status: 503,
            json: { code: 503, data: {}, message: "temporarily unavailable" },
          });
          return;
        }
        await route.fulfill({ json: envelope(uploadPolicy) });
        return;
      }
      if (request.method() === "GET" && path.endsWith("/categories")) {
        await route.fulfill({ json: envelope({ items: [], nextCursor: "" }) });
        return;
      }
      if (request.method() === "GET" && path.endsWith("/items")) {
        await route.fulfill({
          json: envelope({
            items: [contentSummary()],
            nextCursor: "",
            total: 1,
          }),
        });
        return;
      }
      if (
        request.method() === "GET" &&
        path.endsWith(`/items/${contentManagementNo}`)
      ) {
        await route.fulfill({ json: envelope({ item: contentDetail() }) });
        return;
      }
      if (
        request.method() === "POST" &&
        path.endsWith(`/items/${contentManagementNo}/upload-sessions`)
      ) {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.uploadSessionRequests.push(body);
        uploadSizeBytes = Number(body.sizeBytes);
        await route.fulfill({
          json: envelope({
            item: {
              chunkSizeBytes: 1024,
              confirmedBytes: 0,
              expiresAt: 1_786_413_600_000,
              fileName: String(body.fileName),
              sizeBytes: uploadSizeBytes,
              uploadNo: "UPL_CONTENT_0001",
              version: 1,
            },
          }),
        });
        return;
      }
      if (
        request.method() === "PUT" &&
        path.endsWith("/upload-sessions/UPL_CONTENT_0001/chunks/0")
      ) {
        await route.fulfill({
          json: envelope({
            complete: true,
            confirmedBytes: uploadSizeBytes,
            nextOffset: uploadSizeBytes,
            uploadNo: "UPL_CONTENT_0001",
            version: 2,
          }),
        });
        return;
      }
      if (
        request.method() === "POST" &&
        path.endsWith("/upload-sessions/UPL_CONTENT_0001/completion")
      ) {
        await route.fulfill({
          json: envelope({
            item: {
              canCancel: true,
              canEnable: false,
              canRescan: false,
              fileName: "ordinary.pdf",
              fileNo: "FIL_CONTENT_0001",
              gateStatus: "pending",
              scanStatus: "pending",
              sizeBytes: uploadSizeBytes,
              version: 1,
            },
          }),
        });
        return;
      }

      await route.fulfill({ status: 404, json: envelope({}) });
    },
  );
  return state;
}

// Mutable API fixture proves persisted CRUD results and failure recovery through the real page.
export async function installContentCRUDMock(page: Page) {
  const state = {
    failSave: false,
    authenticated: false,
    failDetailNo: "",
    deletedNos: [] as string[],
    actions: [] as Record<string, unknown>[],
    writes: [] as Record<string, unknown>[],
    queries: [] as URLSearchParams[],
    items: Array.from({ length: 12 }, (_, index) => ({
      ...contentDetail(),
      contentNo: `CNT_NEWS_${index + 1}`,
      type: "news",
      title: `新闻 ${index + 1}`,
      categoryNo: "CAT_NEWS",
      version: 1,
    })),
    categories: [
      {
        categoryNo: "CAT_NEWS",
        name: "公司新闻",
        type: "news",
        version: 1,
        synthetic: false,
      },
    ],
  };
  await page.route("**/x/idc-core/api/v1/admin/content/**", async (route) => {
    const req = route.request();
    state.authenticated = Boolean(
      req.headers().authorization?.startsWith("Bearer "),
    );
    const url = new URL(req.url());
    const path = url.pathname;
    const respond = (data: unknown) => route.fulfill({ json: envelope(data) });
    if (path.endsWith("/upload-policy")) return respond(defaultUploadPolicy);
    if (path.endsWith("/categories")) {
      if (req.method() === "POST") {
        const body = req.postDataJSON();
        const item = {
          ...body,
          categoryNo: "CAT_NEW",
          version: 1,
          synthetic: false,
        };
        state.categories.push(item);
        return respond({ item });
      }
      return respond({ items: state.categories, nextCursor: "" });
    }
    if (path.includes("/categories/") && req.method() === "DELETE") {
      const no = path.split("/").pop();
      state.categories = state.categories.filter(
        (item) => item.categoryNo !== no,
      );
      state.items.forEach((item) => {
        if (item.categoryNo === no) item.categoryNo = "";
      });
      return respond({});
    }
    if (path.endsWith("/items") && req.method() === "GET") {
      state.queries.push(url.searchParams);
      const rows = state.items.filter(
        (item) =>
          (!url.searchParams.get("type") ||
            item.type === url.searchParams.get("type")) &&
          (!url.searchParams.get("query") ||
            item.title.startsWith(url.searchParams.get("query")!)) &&
          (!url.searchParams.get("categoryNo") ||
            item.categoryNo === url.searchParams.get("categoryNo")),
      );
      const limit = Number(url.searchParams.get("limit") || 10),
        offset = (Number(url.searchParams.get("page") || 1) - 1) * limit;
      return respond({
        items: rows.slice(offset, offset + limit),
        total: rows.length,
        nextCursor: "",
      });
    }
    if (
      (req.method() === "POST" && path.endsWith("/items")) ||
      req.method() === "PUT"
    ) {
      if (state.failSave)
        return route.fulfill({
          status: 503,
          json: { code: 503, message: "Unavailable" },
        });
      const body = req.postDataJSON();
      state.writes.push(body);
      const existing = state.items.find((item) =>
        path.endsWith(`/items/${item.contentNo}`),
      );
      const item = {
        ...contentDetail(),
        ...existing,
        ...body,
        contentNo: existing?.contentNo || "CNT_CREATED",
        version: (existing?.version || 0) + 1,
        status: existing?.status || body.status,
      };
      if (existing) Object.assign(existing, item);
      else state.items.unshift(item);
      return respond({ item });
    }
    const no = path.split("/items/")[1]?.split("/")[0];
    const item = state.items.find((item) => item.contentNo === no);
    if (item && req.method() === "GET" && state.failDetailNo === no)
      return route.fulfill({ status: 503, json: { code: 503 } });
    if (item && req.method() === "DELETE") {
      state.deletedNos.push(item.contentNo);
      state.items = state.items.filter((row) => row !== item);
      return respond({ item });
    }
    if (item && req.method() === "POST") {
      state.actions.push({
        action: path.split("/").pop(),
        ...req.postDataJSON(),
      });
      item.status = path.endsWith("/unpublications") ? "offline" : "published";
      item.version++;
      return respond({ item });
    }
    if (item) return respond({ item });
    return route.fulfill({ status: 404, json: envelope({}) });
  });
  return state;
}

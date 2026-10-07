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
    targetCustomerNos: [],
    targetServiceNos: [],
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
          json: envelope({ items: [contentSummary()], nextCursor: "" }),
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

import type { Page } from "@playwright/test";
export async function mockChannelWorkspace(page: Page) {
  const state = {
    failTokens: false,
    failPage: "",
    largeLists: false,
    delayTokenWrite: undefined as Promise<void> | undefined,
    delayNextTokenRead: undefined as Promise<void> | undefined,
    startedDelayedTokenReads: 0,
    completedTokenWrites: 0,
    created: undefined as Record<string, unknown> | undefined,
    publication: undefined as Record<string, unknown> | undefined,
    token: undefined as Record<string, unknown> | undefined,
    revoked: false,
    status: "",
    reads: [] as string[],
    pageReads: [] as Array<{ path: string; page: number; pageSize: number }>,
  };
  const now = Date.parse("2026-09-04T00:00:00Z");
  const accounts = Array.from({ length: 26 }, (_, i) => ({
    channelId: i ? "channel_" + i : "channel_primary",
    customerNo: "PUBLIC-" + i,
    displayName: i ? "渠道" + i : "主渠道",
    statusCode: i % 2 ? "paused" : "approved",
    newSalesEnabled: i % 2 === 0,
    updatedAt: now,
  }));
  await page.route(
    "**/x/idc-channel/api/v1/admin/channel/**",
    async (route) => {
      const url = new URL(route.request().url());
      const path = url.pathname.split("/admin/channel/")[1];
      const method = route.request().method();
      const pageNumber = Number(url.searchParams.get("page") || 1);
      const pageSize = Number(url.searchParams.get("pageSize") || 10);
      const success = (raw: unknown) => {
        const data = raw as Record<string, unknown>;
        if (method === "GET" && Array.isArray(data.list)) {
          let rows = data.list;
          if (
            state.largeLists &&
            (path === "publications" || path === "requests")
          )
            rows = Array.from({ length: 27 }, (_, index) => ({
              ...rows[0],
              ...(path === "publications"
                ? {
                    productVersionId: 101 + index,
                    publicSlug: "server-basic-" + (index + 1),
                  }
                : { requestId: "req-" + (index + 1) }),
            }));
          if (path === "accounts" && url.searchParams.get("keyword"))
            rows = rows.filter((row) =>
              String(row.displayName + row.channelId).includes(
                url.searchParams.get("keyword")!,
              ),
            );
          data.total = rows.length;
          data.list = rows.slice(
            (pageNumber - 1) * pageSize,
            pageNumber * pageSize,
          );
          data.page = pageNumber;
          data.pageSize = pageSize;
        }
        if (path === "finance") {
          for (const [field, stem, id] of [
            ["invoices", "invoice", "invoiceNo"],
            ["creditStatements", "statement", "statementNo"],
            ["creditRepayments", "repayment", "repaymentNo"],
          ]) {
            const first = (data[field!] as Record<string, unknown>[])[0]!;
            const rows = Array.from({ length: 27 }, (_, index) => ({
              ...first,
              [id!]: index
                ? String(first[id!]) + "-" + (index + 1)
                : first[id!],
            }));
            const number = Number(url.searchParams.get(stem + "Page") || 1);
            const size = Number(url.searchParams.get(stem + "PageSize") || 10);
            data[field!] = rows.slice((number - 1) * size, number * size);
            data[stem + "Total"] = rows.length;
          }
        }
        return route.fulfill({ json: { code: 0, data, message: "success" } });
      };
      if (method === "GET") state.reads.push(path || "");
      if (method === "GET")
        state.pageReads.push({ path: path || "", page: pageNumber, pageSize });
      if (method === "GET" && state.failPage === path)
        return route.fulfill({
          status: 503,
          json: { code: 503, message: "Read failed" },
        });
      if (path === "accounts") {
        if (method === "POST") {
          state.created = route.request().postDataJSON();
          accounts.unshift({
            ...(state.created as (typeof accounts)[number]),
            statusCode: "pending_approval",
            newSalesEnabled: false,
            updatedAt: now,
          });
        }
        return success({ list: accounts });
      }
      if (path?.endsWith("/status")) {
        state.status = route.request().postDataJSON().action;
        accounts[0]!.statusCode =
          state.status === "pause" ? "paused" : "approved";
        return success({ channel: accounts[0] });
      }
      if (path === "tokens") {
        if (method === "POST") {
          state.token = route.request().postDataJSON();
          await state.delayTokenWrite;
          state.completedTokenWrites++;
          return success({ id: 42, token: "test-one-time-secret" });
        }
        if (state.failTokens)
          return route.fulfill({
            status: 403,
            json: { code: 403, message: "Permission denied" },
          });
        if (state.delayNextTokenRead) {
          const delayed = state.delayNextTokenRead;
          state.delayNextTokenRead = undefined;
          state.startedDelayedTokenReads++;
          await delayed;
        }
        return success({
          list: Array.from({ length: 24 }, (_, i) => ({
            id: 42 + i,
            scopes:
              url.searchParams.get("channelId") === "channel_1"
                ? ["channel-one.read"]
                : ["catalog.read", "order.read"],
            sourceCidrs: ["192.0.2.0/24"],
            expiresAt: now + 86400000,
            revokedAt: i === 0 && state.revoked ? now : null,
          })),
        });
      }
      if (path === "tokens/42" && method === "DELETE") {
        await state.delayTokenWrite;
        state.completedTokenWrites++;
        state.revoked = true;
        return success({ revoked: true });
      }
      if (path === "publications") {
        if (method === "POST")
          state.publication = route.request().postDataJSON();
        return success({
          list: [
            {
              productVersionId: 101,
              currency: "CNY",
              priceMinor: 8800,
              stock: 4,
              status: "published",
              publicSlug: "server-basic",
            },
          ],
        });
      }
      if (path === "catalog")
        return success({
          list: [{ currency: "CNY", price: "88.00", stock: 4 }],
        });
      if (path === "finance")
        return success({
          currencies: [
            {
              currencyCode: "CNY",
              availableBalanceMinor: 120000,
              creditAvailableMinor: 500000,
              receivableMinor: 8800,
              paidMinor: 17600,
              refundedMinor: 0,
              serviceCostMinor: 9000,
            },
          ],
          invoices: [
            {
              invoiceNo: "INV-001",
              orderNo: "ORD-001",
              currencyCode: "CNY",
              totalAmountMinor: 8800,
              paidAmountMinor: 8800,
              creditAmountMinor: 0,
              refundedAmountMinor: 0,
              status: "paid",
              createdAt: now,
            },
          ],
          creditStatements: [
            {
              statementNo: "CST-001",
              currencyCode: "CNY",
              totalMinor: 9000,
              outstandingMinor: 0,
              status: "settled",
              paymentDueAt: now,
            },
          ],
          creditRepayments: [
            {
              repaymentNo: "CRP-001",
              statementNo: "CST-001",
              amountMinor: 9000,
              currencyCode: "CNY",
              status: "succeeded",
              createdAt: now,
            },
          ],
        });
      if (path === "health")
        return success({
          statusCode: "approved",
          newSalesEnabled: true,
          activeTokenCount: 24,
          publicationCount: 1,
          pendingWebhookCount: 0,
          lastRequestAt: now,
        });
      if (path === "requests")
        return success({
          list: [
            {
              requestId: "req-001",
              operation: "quote",
              status: "accepted",
              resultCode: "ok",
              createdAt: now,
              completedAt: now + 1000,
            },
          ],
        });
      if (path === "capabilities")
        return success({
          operations: {
            catalog: "enabled",
            reconfigure: "enabled_core_guarded",
          },
          adapters: { v10: "disabled_unverified" },
        });
      return success({});
    },
  );
  return state;
}

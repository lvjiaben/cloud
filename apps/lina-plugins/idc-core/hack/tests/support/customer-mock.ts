import type { Page, Route } from "@host-tests/support/playwright";

const firstCustomer = {
  customerCode: "10000",
  primaryPhone: "13800138000",
  primaryEmail: "customer@example.test",
  countryCode: "CN",
  createdAt: 1_786_161_600_000,
  customerNo: "CUS_0123456789abcdef0123456789abcdef",
  displayName: "上海云舟科技",
  level: {
    code: "gold",
    levelNo: "LVL_0123456789abcdef0123456789abcdef",
    nameEnUs: "Gold",
    nameZhCn: "金牌客户",
    priority: 100,
  },
  preferredLocale: "zh-CN",
  status: "active",
  tagCount: 2,
  type: "enterprise",
  updatedAt: 1_786_161_660_000,
  verification: "verified",
  version: 3,
};

const secondCustomer = {
  ...firstCustomer,
  customerNo: "CUS_abcdef0123456789abcdef0123456789",
  displayName: "林辰",
  level: null,
  status: "restricted",
  tagCount: 0,
  type: "individual",
  verification: "pending",
};

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

export interface CustomerMockState {
 overviewExtras:Record<string,unknown>;
 listExtras:Record<string,unknown>;
  activationRequests: Array<Record<string, unknown>>;
  directRequests: Array<Record<string, unknown>>;
  batchRequests: Array<Record<string, unknown>>;
  batchFailures: number;
  listRequests: URL[];
  passwordResetRequests: Array<Record<string, unknown>>;
  resendRequests: Array<Record<string, unknown>>;
  segmentRequests: URL[];
  statusRequests: Array<Record<string, unknown>>;
  segmentMutations: Array<Record<string, unknown>>;
  noteRequests: Array<Record<string, unknown>>;
  auditRequests: URL[];
  directFailures: number;
}

export async function installCustomerManagementMock(
  page: Page,
): Promise<CustomerMockState> {
  const state: CustomerMockState = {
 overviewExtras:{},listExtras:{},
    activationRequests: [],
    directRequests: [],
    batchRequests: [],
    batchFailures: 0,
    listRequests: [],
    passwordResetRequests: [],
    resendRequests: [],
    segmentRequests: [],
    statusRequests: [], segmentMutations: [], noteRequests: [], auditRequests: [], directFailures: 0,
  };
  const currentCustomer = {...firstCustomer};
  const notes: Array<Record<string, unknown>> = [];
  const addedTags: Array<Record<string, unknown>> = [];
  const addedGroups: Array<Record<string, unknown>> = [];
  const pendingCustomers: Array<Record<string, unknown>> = [];
  const activationReplays = new Map<string, Record<string, unknown>>();
  await page.route(
    "**/x/idc-core/api/v1/admin/customer-segments**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.segmentRequests.push(url);
      const isGroup = url.searchParams.get("kind") === "group";
      await route.fulfill({
        json: apiEnvelope({
          items: [
            ...(isGroup ? addedGroups : addedTags),
            isGroup
              ? {
                  code: "managed",
                  colorToken: "",
                  nameEnUs: "Managed",
                  nameZhCn: "重点维护",
                  number: "GRP_0123456789abcdef0123456789abcdef",
                  status: "active",
                }
              : {
                  code: "priority",
                  colorToken: "warning",
                  nameEnUs: "Priority",
                  nameZhCn: "优先客户",
                  number: "TAG_0123456789abcdef0123456789abcdef",
                  status: "active",
                },
          ],
          nextCursor: "",
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/sales-channel/customers/*/referrer",
    async (route: Route) => {
      await route.fulfill({
        json: apiEnvelope({
          customerCode: "",
          customerNo: "",
          displayName: "",
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/customers**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      const method = route.request().method();
      if (method === "PUT" && url.pathname.endsWith("/account-status")) {
        const body=route.request().postDataJSON(); state.statusRequests.push(body); currentCustomer.status=body.toStatus; currentCustomer.version+=1;
        await route.fulfill({json:apiEnvelope({customerNo:currentCustomer.customerNo,status:currentCustomer.status,version:currentCustomer.version})});return;
      }
      if (url.pathname.endsWith("/audit-records")) {state.auditRequests.push(url);await route.fulfill({json:apiEnvelope({items:[{auditNo:"AUD_0123456789abcdef0123456789abcdef",occurredAt:1780000000000,operationCode:"customer.password.reset",result:"succeeded"}]})});return;}
      if (url.pathname.endsWith("/activities")) {await route.fulfill({json:apiEnvelope({items:notes,nextCursor:""})});return;}
      if (method==="POST" && url.pathname.endsWith("/notes")) {const body=route.request().postDataJSON();state.noteRequests.push(body);notes.unshift({number:"NTE_0123456789abcdef0123456789abcdef",kind:"note",title:body.content,detail:body.content,status:"active",category:body.category,createdAt:Date.now(),updatedAt:Date.now()});await route.fulfill({json:apiEnvelope({})});return;}
      if (method === "POST" && url.pathname.endsWith("/segments")) {
        const body = route.request().postDataJSON(); state.segmentMutations.push(body);
        const number = body.segmentNo || `${body.kind === "tag" ? "TAG" : "GRP"}_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`;
        const items = body.kind === "tag" ? addedTags : addedGroups;
        if (!items.some(item => item.number === number)) items.push({number,code:"test",nameZhCn:body.name,nameEnUs:body.name,status:"active"});
        await route.fulfill({json:apiEnvelope({segmentNo:number})}); return;
      }
      if (method === "DELETE" && url.pathname.includes("/segments/")) {
        const body = route.request().postDataJSON(); const number = url.pathname.split("/").at(-1);
        state.segmentMutations.push({...body, segmentNo:number, removed:true});
        const items = body.kind === "tag" ? addedTags : addedGroups;
        const index = items.findIndex(item => item.number === number); if (index >= 0) items.splice(index,1);
        await route.fulfill({json:apiEnvelope({})}); return;
      }
      if (method === "PUT" && url.pathname.endsWith("/password")) {
        state.passwordResetRequests.push(
          route.request().postDataJSON() as Record<string, unknown>,
        );
        await route.fulfill({
          json: apiEnvelope({ customerNo: url.pathname.split("/").at(-2) }),
        });
        return;
      }
      if (
        method === "POST" &&
        url.pathname.endsWith("/admin/customers/direct-accounts")
      ) {
        const body = route.request().postDataJSON() as Record<string, unknown>;
        state.directRequests.push(body);
        if(state.directFailures>0){state.directFailures-=1;await route.fulfill({status:409,json:{code:409,message:"账号已存在，请换一个"}});return;}
        const index = pendingCustomers.length + 10;
        const customerNo = `CUS_${String(index).padStart(32, "0")}`;
        pendingCustomers.push({
          ...firstCustomer,
          customerNo,
          primaryEmail: body.email || "",
          primaryPhone: body.phone || "",
          displayName: body.displayName,
          preferredLocale: body.preferredLocale,
          status: "active",
          type: body.customerType,
          verification: "unverified",
          level: null,
          tagCount: 0,
        });
        await route.fulfill({
          json: apiEnvelope({
            customerNo,
            mustChangePassword: false,
            reused: false,
            status: "active",
            userId: 99,
          }),
        });
        return;
      }
      if (
        method === "POST" &&
        url.pathname.endsWith("/activation-invitations/resend")
      ) {
        const body = route.request().postDataJSON() as Record<string, unknown>;
        state.resendRequests.push(body);
        await route.fulfill({
          json: apiEnvelope({
            invitationNo: "INV_resend_000000000000000000000001",
            customerNo: url.pathname.split("/").at(-3),
            destinationHint: "b***@example.com",
            status: "active",
            expiresAt: Date.now() + 86_400_000,
            reused: false,
          }),
        });
        return;
      }
      if (
        method === "POST" &&
        url.pathname.endsWith("/admin/customers/activation-invitations")
      ) {
        const body = route.request().postDataJSON() as Record<string, unknown>;
        state.activationRequests.push(body);
        if (body.email === "staff@example.com") {
          await route.fulfill({
            status: 409,
            json: {
              code: 409,
              message: "员工账号不能绑定为客户，请使用独立客户账号",
            },
          });
          return;
        }
        const key = String(body.idempotencyKey ?? "");
        const replay = activationReplays.get(key);
        if (replay) {
          await route.fulfill({
            json: apiEnvelope({ ...replay, reused: true }),
          });
          return;
        }
        const index = pendingCustomers.length + 1;
        const customerNo = `CUS_${String(index).padStart(32, "0")}`;
        const result = {
          invitationNo: `INV_${String(index).padStart(32, "0")}`,
          customerNo,
          customerType: body.customerType,
          primaryEmail: body.email || "",
          primaryPhone: body.phone || "",
          displayName: body.displayName,
          destinationHint: body.email ? "b***@example.com" : "+86 ***8000",
          targetUserId: body.bindExistingIdentity ? 88 : 0,
          existingIdentityBound: Boolean(body.bindExistingIdentity),
          status: "active",
          expiresAt: Date.now() + 86_400_000,
          reused: false,
        };
        activationReplays.set(key, result);
        pendingCustomers.push({
          ...firstCustomer,
          customerNo,
          primaryEmail: body.email || "",
          primaryPhone: body.phone || "",
          displayName: body.displayName,
          preferredLocale: body.preferredLocale,
          status: "pending_activation",
          type: body.customerType,
          verification: "unverified",
          level: null,
          tagCount: 0,
        });
        await route.fulfill({ json: apiEnvelope(result) });
        return;
      }
      if (url.pathname.endsWith(`/${firstCustomer.customerNo}`)) {
        await route.fulfill({
          json: apiEnvelope({
            ...state.overviewExtras,
            balances: (state.overviewExtras.balances||[]),
            business: {
              audits: 8,
              bills: 3,
              contracts: 2,
              notifications: 4,
              orders: 5,
              servers: 2,
              taxInvoices: 1,
              tickets: 3,
              transactions: 6,
            },
            counts: {
              activeAddresses: 1,
              activeBlacklist: 0,
              activeContacts: 2,
              activeCustomFields: 3,
              activeGroups: 1,
              activeNotes: 4,
              activeTags: 2,
              availableAttachments: 2,
              openRisks: 1,
              plannedFollowUps: 1,
            },
            customer: currentCustomer,
            groups: {
              hasMore: false,
              items: [...addedGroups,
                {
                  code: "managed",
                  colorToken: "",
                  nameEnUs: "Managed",
                  nameZhCn: "重点维护",
                  number: "GRP_0123456789abcdef0123456789abcdef",
                  status: "active",
                },
              ],
            },
            tags: {
              hasMore: false,
              items: [...addedTags,
                {
                  code: "priority",
                  colorToken: "warning",
                  nameEnUs: "Priority",
                  nameZhCn: "优先客户",
                  number: "TAG_0123456789abcdef0123456789abcdef",
                  status: "active",
                },
              ],
            },
          }),
        });
        return;
      }

      state.listRequests.push(url);
      const pageNumber = Number(url.searchParams.get("page") || 1);
      const size = Number(url.searchParams.get("limit") || 10);
      const prefix = url.searchParams.get("namePrefix");
      if (
        prefix ||
        url.searchParams.get("groupNo") ||
        url.searchParams.get("tagNo")
      ) {
        await route.fulfill({
          json: apiEnvelope({ customers: pageNumber === 1 ? [firstCustomer] : [], nextCursor: "", total: 1 }),
        });
        return;
      }
      const directory = [firstCustomer, secondCustomer, ...pendingCustomers,
        ...Array.from({ length: 18 }, (_, index) => ({ ...secondCustomer,
          customerNo: `CUS_${(index + 1).toString(16).padStart(32, "0")}`,
          displayName: `分页客户 ${index + 1}`,
        }))];
      const offset = (pageNumber - 1) * size;
      await route.fulfill({ json: apiEnvelope({ customers: directory.slice(offset, offset + size).map(item=>({...item,...state.listExtras})), nextCursor: "", total: directory.length }) });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/customers/tags/batch",
    async (route: Route) => {
      state.batchRequests.push(
        route.request().postDataJSON() as Record<string, unknown>,
      );
      if (state.batchFailures > 0) {
        state.batchFailures--;
        await route.fulfill({status:500,json:{code:51,message:"保存失败，请重试"}});return;
      }
      await route.fulfill({
        json: apiEnvelope({
          auditNo: "AUD_0123456789abcdef0123456789abcdef",
          customerCount: 2,
          replayed: false,
          tagCount: 1,
        }),
      });
    },
  );
  return state;
}

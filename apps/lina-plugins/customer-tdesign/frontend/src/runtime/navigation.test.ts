import { describe, it, expect, vi } from "vitest";
import { assembleNavigation, customerNavigation, productNavigation } from "./navigation";
import type { CustomerProductDirectory } from "#/api/idc/product-directories";
import { listCustomerProductDirectories } from "#/api/idc/product-directories";
vi.mock("#/api/idc/product-directories", () => ({
  listCustomerProductDirectories: vi.fn(),
}));
vi.mock("#/api/idc/external-menu", () => ({
  listCustomerExternalMenus: vi.fn(async () => ({ items: [] })),
}));
vi.mock("#/api/request", async (importOriginal) => ({
  ...(await importOriginal<typeof import("#/api/request")>()),
  requestClient: { get: vi.fn(async () => ({ list: [] })) },
}));
const directory = (
  id: string,
  order: number,
  type: CustomerProductDirectory["navigationType"] = "product",
  targetUrl = "",
): CustomerProductDirectory => ({
  directoryNo: id,
  nameZhCn: id,
  nameEnUs: id,
  sortOrder: order,
  serviceCount: 0,
  navigationType: type,
  targetUrl,
});
describe("merchant navigation", () => {
  it("only includes purchased-service menus across both hierarchy levels", () => {
    const groups = assembleNavigation([
      directory("产品与服务", 1, "system", "/account/servers"),
      {...directory("云服务器", 2), parentDirectoryNo: "产品与服务"},
      {...directory("购买商品", 3, "system", "/account/products/catalog"), parentDirectoryNo: "产品与服务"},
      directory("订单", 4, "system", "/account/orders"),
      directory("外链", 5, "custom", "https://example.com"),
      directory("插件", 6, "plugin", "/account/support"),
      directory("物理机", 7),
    ], new Set(["idc-resource"]));
    expect(productNavigation(groups).map(item => item.title)).toEqual(["云服务器", "物理机"]);
    expect(groups.find(item => item.title === "订单")?.directoryNo).toBe("订单");
    expect(productNavigation([])).toEqual([]);
  });
  it("uses only merchant product names and sort order", () => {
    const result=assembleNavigation([
      directory("产品与服务",10,"system","/account/servers"),
      {...directory("证书",20),parentDirectoryNo:"产品与服务",icon:"lucide:shield-check"},
      {...directory("主机",10),parentDirectoryNo:"产品与服务"},
      directory("费用",20,"system","/account/billing"),
      {...directory("我的账单",5,"system","/account/billing/invoices"),parentDirectoryNo:"费用"},
    ],new Set(["idc-resource"]));
    expect(result.map(row=>row.title)).toEqual(["产品与服务","费用"]);
    expect(result[0].items?.map(row=>row.title)).toEqual(["主机","证书"]);
    expect(result[0].path).toBe("/account/servers");
    expect(result[0].items?.[1].icon).toBe("lucide:shield-check");
    expect(result[1].items?.[0].title).toBe("我的账单");
  });
  it("rejects unsafe targets and hides disabled optional features", () => {
    const result = assembleNavigation(
      [
        directory("认证", 1, "system", "/account/verification"),
        directory("已退役合同", 1, "system", "/account/contracts"),
        directory("恶意", 2, "custom", "javascript:alert(1)"),
        directory("越界", 3, "system", "/accounting"),
        directory("外部", 4, "custom", "https://help.example.com/"),
      ],
      new Set(),
    );
    expect(result.flatMap((v) => v.items || [v]).map((v) => v.title)).toEqual([
      "外部",
    ]);
  });
  it("successful empty snapshot does not invent product menus", () => {
    expect(assembleNavigation([], new Set())).toEqual([]);
  });
  it("loads all pages before publishing a navigation snapshot", async () => {
    vi.mocked(listCustomerProductDirectories).mockImplementation(
      async (page = 1) => ({
        total: 205,
        items: Array.from({ length: page === 3 ? 5 : 100 }, (_, i) =>
          directory(`产品${(page - 1) * 100 + i}`, i),
        ),
      }),
    );
    const result = await customerNavigation();
    expect(result).toHaveLength(205);
    expect(listCustomerProductDirectories).toHaveBeenCalledTimes(3);
  });
  it("does not invent grouping or promote children with missing parents",()=>{
    const result=assembleNavigation([
      directory("商家一级",5,"system","/account/profile"),
      {...directory("孤立子菜单",1),parentDirectoryNo:"missing"},
    ],new Set());
    expect(result.map(row=>row.title)).toEqual(["商家一级"]);
    expect(result[0].items).toBeUndefined();
  });

  it("keeps a data-owned heading for available children when its optional page is unavailable",()=>{
    const result=assembleNavigation([
      directory("产品与服务",1,"system","/account/servers"),
      {...directory("普通服务",2),parentDirectoryNo:"产品与服务"},
    ],new Set());
    expect(result).toHaveLength(1);expect(result[0].items?.[0].title).toBe("普通服务");
  });

});

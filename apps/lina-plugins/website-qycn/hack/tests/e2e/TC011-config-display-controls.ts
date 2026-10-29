import { test, expect } from "../support/fixtures";

const fields = [
  {
    fieldToken: "cpu",
    type: "integer",
    labelZhCN: "处理器核心数",
    labelEnUS: "CPU cores",
    displayControl: "radio",
    visible: true,
    required: false,
    minimumInteger: "2",
    maximumInteger: "4",
    numericChoices: ["2", "4"],
    numericLabels: {
      "2": { zhCN: "双核", enUS: "Dual core" },
      "4": { zhCN: "四核", enUS: "Quad core" },
    },
  },
  {
    fieldToken: "memory",
    type: "integer",
    labelZhCN: "内存",
    labelEnUS: "Memory",
    displayControl: "select",
    visible: true,
    required: false,
    minimumInteger: "2",
    maximumInteger: "8",
    numericChoices: ["8", "2"],
    numericLabels: {
      "2": { zhCN: "2 GiB", enUS: "2 GiB" },
      "8": { zhCN: "8 GiB", enUS: "8 GiB" },
    },
  },
  {
    fieldToken: "bandwidth",
    type: "integer",
    labelZhCN: "带宽",
    labelEnUS: "Bandwidth",
    displayControl: "slider",
    visible: true,
    required: false,
    minimumInteger: "2",
    maximumInteger: "10",
    quantityStep: "2",
  },
  {
    fieldToken: "region",
    type: "single_select",
    labelZhCN: "地区",
    labelEnUS: "Region",
    displayControl: "select",
    visible: true,
    required: true,
    options: [
      { optionToken: "xa", labelZhCN: "西安", labelEnUS: "Xi’an" },
      { optionToken: "bj", labelZhCN: "北京", labelEnUS: "Beijing" },
    ],
  },
  {
    fieldToken: "line",
    type: "single_select",
    labelZhCN: "线路",
    labelEnUS: "Network",
    displayControl: "radio",
    visible: true,
    required: true,
    options: [
      { optionToken: "ct", labelZhCN: "电信", labelEnUS: "Telecom" },
      { optionToken: "cu", labelZhCN: "联通", labelEnUS: "Unicom" },
    ],
  },
  {
    fieldToken: "fixed",
    type: "integer",
    labelZhCN: "固定规格",
    labelEnUS: "Fixed specification",
    displayControl: "radio",
    visible: true,
    required: true,
    disabled: true,
    numericChoices: ["1", "2"],
  },
  {
    fieldToken: "ports",
    type: "integer",
    labelZhCN: "端口数量",
    labelEnUS: "Port count",
    displayControl: "input",
    visible: true,
    required: false,
    minimumInteger: "0",
    maximumInteger: "10",
  },
  {
    fieldToken: "hidden",
    type: "integer",
    labelZhCN: "隐藏规格",
    labelEnUS: "Hidden specification",
    visible: false,
    required: true,
  },
];
const initial = [
  { fieldToken: "line", type: "single_select", optionTokens: ["cu"] },
  { fieldToken: "fixed", type: "integer", integerValue: "2" },
  { fieldToken: "hidden", type: "integer", integerValue: "9" },
];

test.describe("TC-11 商家配置展示方式", () => {
  for (const language of ["zh-CN", "en-US"]) {
    test(`TC-11${language === "zh-CN" ? "a" : "b"}: ${language} 展示控件保留原数值与权威核价`, async ({
      site,
      page,
    }) => {
      const requests: any[] = [];
      let rejectPrice = false;
      await page.route("**/__website/session", (route) =>
        route.fulfill({ json: { checked: true } }),
      );
      await page.route("**/catalog/products/controls**", async (route) => {
        const path = new URL(route.request().url()).pathname;
        let data: any;
        if (path.endsWith("/controls"))
          data = {
            product: {
              nameZhCN: "展示配置测试",
              nameEnUS: "Configuration controls",
              currencies: ["CNY"],
            },
          };
        else if (path.endsWith("/purchase-configuration"))
          data = {
            ready: true,
            valid: true,
            configurationRevision: "R",
            fields,
            values: initial,
          };
        else {
          const body = route.request().postDataJSON();
          requests.push({ path, ...body });
          if (path.endsWith("/quote")) {
            if (rejectPrice)
              return route.fulfill({ status: 409, json: { code: 409 } });
            const cpu = Number(
              body.values.find((value: any) => value.fieldToken === "cpu")
                ?.integerValue,
            );
            const bandwidth = Number(
              body.values.find((value: any) => value.fieldToken === "bandwidth")
                ?.integerValue || 0,
            );
            data = {
              configurationRevision: "R",
              currencyCode: "CNY",
              terms: [
                {
                  billingTermToken: "M",
                  labelZhCN: "月付",
                  labelEnUS: "Monthly",
                  minimumPurchaseCycles: 1,
                  status: "final",
                  totalAmount: String(cpu * 1000 + bandwidth * 100),
                },
              ],
            };
          } else
            data = {
              ready: true,
              valid: true,
              configurationRevision: "R",
              fields,
              values: body.values.some(
                (value: any) => value.fieldToken === "bandwidth",
              )
                ? body.values
                : [
                    ...body.values,
                    {
                      fieldToken: "bandwidth",
                      type: "integer",
                      integerValue: "2",
                    },
                  ],
            };
        }
        await route.fulfill({ json: { code: 0, data } });
      });
      await site.open(`/configure.html?product=controls&lang=${language}`);
      await expect(page.locator("#purchase-price")).toHaveText("CNY 22.00");
      const zh = language === "zh-CN";
      await expect(
        page.getByRole("radio", {
          name: zh ? "双核" : "Dual core",
          exact: true,
        }),
      ).toBeChecked();
      await expect(
        page.getByLabel(zh ? "内存" : "Memory", { exact: false }),
      ).toHaveJSProperty("tagName", "SELECT");
      await expect(
        page.getByLabel(zh ? "地区" : "Region", { exact: false }),
      ).toHaveJSProperty("tagName", "SELECT");
      await expect(
        page
          .getByRole("group", {
            name: zh ? "固定规格" : "Fixed specification",
            exact: true,
          })
          .getByRole("radio")
          .last(),
      ).toBeDisabled();
      await expect(
        page.getByRole("radio", { name: "2", exact: true }),
      ).toBeChecked();
      await expect(
        page.getByText(zh ? "隐藏规格" : "Hidden specification", {
          exact: true,
        }),
      ).toHaveCount(0);
      await expect(
        page.locator(".purchase-slider input[type=number]"),
      ).toHaveValue("2");
      await expect(
        page.getByLabel(zh ? "内存" : "Memory", { exact: false }),
      ).toHaveValue("8");
      await expect(
        page.getByLabel(zh ? "端口数量" : "Port count", { exact: false }),
      ).toHaveValue("0");
      await expect(
        page.getByRole("radio", { name: zh ? "联通" : "Unicom", exact: true }),
      ).toBeChecked();
      await expect(page.locator(".purchase-form")).not.toContainText(
        zh ? "套餐默认" : "Package default",
      );
      const initialQuote = requests.find((body) =>
        body.path.endsWith("/quote"),
      );
      expect(initialQuote.values).toEqual(
        expect.arrayContaining([
          { fieldToken: "cpu", type: "integer", integerValue: "2" },
          { fieldToken: "memory", type: "integer", integerValue: "8" },
          { fieldToken: "bandwidth", type: "integer", integerValue: "2" },
          { fieldToken: "ports", type: "integer", integerValue: "0" },
          { fieldToken: "region", type: "single_select", optionTokens: ["xa"] },
          { fieldToken: "line", type: "single_select", optionTokens: ["cu"] },
          { fieldToken: "fixed", type: "integer", integerValue: "2" },
        ]),
      );
      expect(
        requests.every(
          (body) =>
            !body.values.some((value: any) => value.fieldToken === "hidden"),
        ),
      ).toBe(true);
      await expect(page.locator(".purchase-selection")).toContainText(
        zh ? "双核" : "Dual core",
      );
      await expect(page.locator(".purchase-terms")).toContainText(
        zh ? "月付" : "Monthly",
      );
      await expect(page.locator(".purchase-terms")).not.toContainText(
        zh ? "日付" : "Daily",
      );
      await site.screenshot(`display-controls-${language}-initial`);
      await site.chooseConfigurationRadio(
        zh ? "处理器核心数" : "CPU cores",
        zh ? "四核" : "Quad core",
      );
      await expect(page.locator("#purchase-price")).toHaveText("CNY 42.00");
      await site.chooseConfiguration(zh ? "内存" : "Memory", "8");
      await site.chooseConfiguration(zh ? "地区" : "Region", "bj");
      await site.chooseConfigurationRadio(
        zh ? "线路" : "Network",
        zh ? "联通" : "Unicom",
      );
      await site.slideConfiguration(
        zh ? "带宽 · 调整数值" : "Bandwidth · Adjust value",
        "6",
      );
      await expect(page.locator("#purchase-price")).toHaveText("CNY 46.00");
      const quote = requests
        .filter((body) => body.path.endsWith("/quote"))
        .at(-1);
      expect(quote.values).toEqual(
        expect.arrayContaining([
          { fieldToken: "cpu", type: "integer", integerValue: "4" },
          { fieldToken: "memory", type: "integer", integerValue: "8" },
          { fieldToken: "bandwidth", type: "integer", integerValue: "6" },
          { fieldToken: "region", type: "single_select", optionTokens: ["bj"] },
          { fieldToken: "line", type: "single_select", optionTokens: ["cu"] },
        ]),
      );
      await expect(page.locator(".purchase-selection")).toContainText(
        zh ? "四核" : "Quad core",
      );
      await site.mobile();
      await site.screenshot(`display-controls-${language}-selected`);
      const quotedCount = requests.filter((body) =>
        body.path.endsWith("/quote"),
      ).length;
      await site.configureNumber(zh ? "带宽" : "Bandwidth", "5");
      expect(
        await page
          .locator(".purchase-slider input[type=number]")
          .evaluate((input: HTMLInputElement) => input.validity.stepMismatch),
      ).toBe(true);
      await expect(page.locator(".price-status")).toContainText(
        zh ? "请选择必填配置" : "Complete the required options",
      );
      await expect(page.locator(".purchase-add")).toBeDisabled();
      expect(
        requests.filter((body) => body.path.endsWith("/quote")).length,
      ).toBe(quotedCount);
      await site.configureNumber(zh ? "带宽" : "Bandwidth", "");
      await expect(
        page.locator(".purchase-slider input[type=number]"),
      ).toHaveValue("2");
      await expect(page.locator("#purchase-price")).toHaveText("CNY 42.00");
      expect(
        requests
          .filter((body) => body.path.endsWith("/evaluate"))
          .some(
            (body) =>
              !body.values.some(
                (value: any) => value.fieldToken === "bandwidth",
              ),
          ),
      ).toBe(true);
      expect(
        requests.filter((body) => body.path.endsWith("/quote")).at(-1).values,
      ).toContainEqual({
        fieldToken: "bandwidth",
        type: "integer",
        integerValue: "2",
      });
      await expect(page.locator(".purchase-selection")).not.toContainText(
        zh ? "套餐默认" : "Package default",
      );
      rejectPrice = true;
      await site.chooseConfigurationRadio(
        zh ? "处理器核心数" : "CPU cores",
        zh ? "双核" : "Dual core",
      );
      await expect(page.locator(".price-status")).toContainText(
        zh ? "价格暂时无法更新" : "Could not update the price",
      );
      await expect(page.locator(".purchase-add")).toBeDisabled();
      await expect(page.locator("#purchase-price")).toHaveText("—");
      await site.screenshot(`display-controls-${language}-error`);
      rejectPrice = false;
      await site.retryConfigurationPrice(zh ? "重试" : "Retry");
      await expect(page.locator("#purchase-price")).toHaveText("CNY 22.00");
      await expect(page.locator(".purchase-add")).toBeEnabled();
      const restored = {
        configurationRevision: "R",
        billingTermToken: "MISSING-DAY",
        currencyCode: "CNY",
        values: [
          { fieldToken: "cpu", type: "integer", integerValue: "4" },
          { fieldToken: "memory", type: "integer", integerValue: "2" },
          { fieldToken: "bandwidth", type: "integer", integerValue: "6" },
          { fieldToken: "region", type: "single_select", optionTokens: ["bj"] },
          { fieldToken: "line", type: "single_select", optionTokens: ["cu"] },
          { fieldToken: "ports", type: "integer", integerValue: "0" },
          { fieldToken: "fixed", type: "integer", integerValue: "2" },
        ],
      };
      await page.addInitScript(
        (draft) =>
          sessionStorage.setItem(
            "website-qycn-configuration-controls",
            JSON.stringify(draft),
          ),
        restored,
      );
      await site.open(`/configure.html?product=controls&lang=${language}`);
      await expect(page.locator("#purchase-price")).toHaveText("CNY 46.00");
      await expect(
        page.getByRole("radio", {
          name: zh ? "四核" : "Quad core",
          exact: true,
        }),
      ).toBeChecked();
      await expect(
        page.getByLabel(zh ? "内存" : "Memory", { exact: false }),
      ).toHaveValue("2");
      await expect(
        page.getByLabel(zh ? "地区" : "Region", { exact: false }),
      ).toHaveValue("bj");
      await expect(
        page.locator(".purchase-slider input[type=number]"),
      ).toHaveValue("6");
      await expect(page.locator(".purchase-term input")).toBeChecked();
      await expect(page.locator(".purchase-terms")).toContainText(zh ? "月付" : "Monthly");
      await site.screenshot(`display-controls-${language}-restored`);
    });
  }
});

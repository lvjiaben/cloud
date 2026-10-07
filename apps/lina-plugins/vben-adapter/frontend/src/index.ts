export interface VbenPluginAdapter {
  Button: unknown;
  Page: unknown;
  useVbenDrawer: Function;
  useVbenForm: Function;
  useVbenModal: Function;
  useVbenVxeGrid: Function;
}

const requiredFunctions = [
  "useVbenDrawer",
  "useVbenForm",
  "useVbenModal",
  "useVbenVxeGrid",
] as const;

export function createVbenPluginAdapter(
  adapter: VbenPluginAdapter,
): Readonly<VbenPluginAdapter> {
  if (!adapter || !adapter.Page || !adapter.Button) {
    throw new Error("official Vben Page and button adapters are required");
  }
  for (const key of requiredFunctions) {
    if (typeof adapter[key] !== "function") {
      throw new Error(`official Vben adapter ${key} is required`);
    }
  }
  return Object.freeze({ ...adapter });
}

export function fixedRightActionColumn<T extends Record<string, unknown>>(
  column: T,
): T & { fixed: "right" } {
  return { ...column, fixed: "right" };
}

export const vbenPluginPageRules = Object.freeze({
  actionColumn: "fixed-right",
  buttonPriority: ["vben", "ant-design-vue", "custom"],
  form: "useVbenForm",
  grid: "useVbenVxeGrid",
  page: "Page+Card",
} as const);

import { describe, expect, it } from "vitest";

import {
  interfaceFieldLabel,
  normalizeInterfaceConfigurationSchema,
  secretConfigurationComplete,
  splitInterfaceConfigurationValues,
} from "./provider-interface-configuration";

const label = { enUS: "API password", zhCN: "接口密码" };

describe("provider interface configuration", () => {
  it("keeps machine keys out of localized labels and separates secret values", () => {
    const schema = normalizeInterfaceConfigurationSchema({
      fields: [
        { key: "region", label, required: true, type: "single_select" },
        { key: "password", label, required: true, type: "secret" },
      ],
      version: "v1",
    });

    expect(interfaceFieldLabel(schema.fields[0]!, "zh-CN")).toBe("接口密码");
    expect(
      splitInterfaceConfigurationValues(schema, {
        password: "never-return-this",
        region: "east",
      }),
    ).toEqual({
      nonSecretSettings: { region: "east" },
      secrets: { password: "never-return-this" },
    });
  });

  it("requires every declared required secret without needing its value", () => {
    const schema = normalizeInterfaceConfigurationSchema({
      fields: [
        { key: "access_user", label, required: true, type: "secret" },
        { key: "access_token", label, required: true, type: "secret" },
        { key: "optional", label, type: "secret" },
      ],
      version: "v1",
    });

    expect(
      secretConfigurationComplete(schema, {
        access_token: true,
        access_user: true,
      }),
    ).toBe(true);
    expect(secretConfigurationComplete(schema, { access_user: true })).toBe(
      false,
    );
  });

  it("drops undeclared data and converts address paths to structured values", () => {
    const schema = normalizeInterfaceConfigurationSchema({
      fields: [
        { key: "area", label, type: "address" },
        { key: "enabled", label, type: "boolean" },
      ],
      version: "v1",
    });

    expect(
      splitInterfaceConfigurationValues(schema, {
        area: ["cn", "zhejiang", "hangzhou"],
        enabled: false,
        rawJson: "must-not-pass",
      }).nonSecretSettings,
    ).toEqual({
      area: { path: ["cn", "zhejiang", "hangzhou"] },
      enabled: false,
    });
  });
});

import { describe, expect, it } from "vitest";

import {
  catalogConfigurationCandidates,
  catalogConfigurationFileTokensWithout,
  catalogConfigurationFieldState,
  catalogConfigurationPresentation,
  catalogConfigurationRowsToValues,
  catalogConfigurationValuesToRows,
  evaluateCatalogConfigurationCondition,
  sanitizeCatalogConfigurationValues,
  type CatalogConfigurationDeclaration,
  validateCatalogConfigurationDraftValue,
} from "./catalog-configuration-state";

const label = { enUS: "Region", zhCN: "地区" };
const declaration: CatalogConfigurationDeclaration = {
  conditions: [
    {
      actions: [
        { action: "disable", field: "note" },
        { action: "filter_candidates", field: "region", values: ["north"] },
      ],
      when: { field: "quantity", operator: "greater_than", value: "1" },
    },
  ],
  fields: [
    {
      key: "region",
      label,
      staticCandidates: [
        { label: { enUS: "North", zhCN: "北区" }, value: "north" },
        { label: { enUS: "South", zhCN: "南区" }, value: "south" },
      ],
      type: "single_select",
    },
    { key: "quantity", label, type: "decimal" },
    { key: "note", label, type: "single_line" },
    { key: "credential", label, type: "secret" },
  ],
  version: "v1",
};

describe("catalog configuration governance", () => {
  it("removes a bound file only from the next interface or product revision", () => {
    const bound = ["MCF_00000000000000000000000000000001"];
    for (const surface of ["interface", "product"] as const) {
      const next = catalogConfigurationFileTokensWithout(bound, bound[0]!);
      expect(next, surface).toEqual([]);
      expect(bound, surface).toEqual(["MCF_00000000000000000000000000000001"]);
      expect([...next, "MCF_00000000000000000000000000000002"]).toEqual([
        "MCF_00000000000000000000000000000002",
      ]);
    }
  });

  it("round-trips governed file tokens on interface and product surfaces", () => {
    const fileDeclaration: CatalogConfigurationDeclaration = {
      fields: [
        { key: "document", label, type: "file", uploadPolicy: "document-v1" },
      ],
      uploadPolicies: [
        {
          id: "document-v1",
          maximumFiles: 2,
          maximumFileBytes: 1024,
          maximumTotalBytes: 2048,
          allowedMimeTypes: ["application/pdf"],
          allowedExtensions: [".pdf"],
          safetyScanRequired: true,
        },
      ],
      version: "v1",
    };
    const values = { document: ["MCF_0123456789ABCDEF0123456789ABCDEF"] };
    for (const surface of ["interface", "product"] as const) {
      const rows = catalogConfigurationValuesToRows(
        fileDeclaration,
        surface,
        values,
      );
      expect(rows[0]?.fileTokens).toEqual(values.document);
      expect(catalogConfigurationRowsToValues(rows, fileDeclaration)).toEqual(
        values,
      );
    }
  });
  it("evaluates bounded conditions with exact decimal comparison", () => {
    expect(
      evaluateCatalogConfigurationCondition(
        {
          children: [
            { field: "region", operator: "equal", value: "north" },
            {
              field: "quantity",
              operator: "greater_than",
              value: "9007199254740993.01",
            },
          ],
          operator: "and",
        },
        { quantity: "9007199254740993.02", region: "north" },
      ),
    ).toBe(true);
    expect(
      evaluateCatalogConfigurationCondition(
        {
          children: Array.from({ length: 65 }, () => ({
            field: "region",
            operator: "not_empty" as const,
          })),
          operator: "or",
        },
        { region: "north" },
      ),
    ).toBe(false);
  });

  it("applies declared field actions without executable expressions", () => {
    expect(
      catalogConfigurationCandidates(declaration, declaration.fields[0]!, {
        quantity: "2",
      }).map((item) => item.value),
    ).toEqual(["north"]);
    expect(
      catalogConfigurationFieldState(declaration, declaration.fields[2]!, {
        quantity: "2",
      }).disabled,
    ).toBe(true);
  });

  it("uses a drawer for conditional or complex declarations", () => {
    expect(catalogConfigurationPresentation(declaration)).toBe("drawer");
    expect(
      catalogConfigurationPresentation({
        fields: declaration.fields.slice(0, 1),
        version: "v1",
      }),
    ).toBe("modal");
  });

  it("serializes every field through one explicit typed arm", () => {
    const allTypes: CatalogConfigurationDeclaration = {
      fields: [
        { key: "line", label, type: "single_line" },
        { key: "lines", label, type: "multi_line" },
        { key: "integer", label, type: "integer" },
        { key: "decimal", label, type: "decimal" },
        { key: "boolean", label, type: "boolean" },
        {
          key: "single",
          label,
          staticCandidates: [{ label, value: "a" }],
          type: "single_select",
        },
        {
          key: "multi",
          label,
          staticCandidates: [{ label, value: "a" }],
          type: "multi_select",
        },
        { key: "date", label, type: "date" },
        { key: "datetime", label, type: "datetime" },
        { key: "secret", label, type: "secret" },
        {
          key: "tree",
          label,
          staticCandidates: [{ label, value: "leaf" }],
          type: "tree_select",
        },
        {
          key: "address",
          label,
          staticCandidates: [{ label, value: "city" }],
          type: "address",
        },
        { key: "file", label, type: "file" },
      ],
      version: "v1",
    };
    const rows = catalogConfigurationValuesToRows(allTypes, "interface", {
      address: ["city"],
      boolean: false,
      date: "2026-08-26",
      datetime: "2026-08-26T08:00:00Z",
      decimal: "9007199254740993.02",
      file: ["opaque-file-token"],
      integer: "42",
      line: "text",
      lines: "first\nsecond",
      multi: ["a"],
      secret: "replace-me",
      single: "a",
      tree: "leaf",
    });
    expect(rows).toHaveLength(13);
    expect(rows.find((row) => row.fieldKey === "decimal")).toEqual({
      decimalValue: "9007199254740993.02",
      fieldKey: "decimal",
    });
    const restored = catalogConfigurationRowsToValues(rows, allTypes);
    expect(restored.decimal).toBe("9007199254740993.02");
    expect(restored.single).toBe("a");
    expect(restored.tree).toBe("leaf");
    expect(restored.multi).toEqual(["a"]);
  });

  it("never accepts secrets on product or customer surfaces", () => {
    expect(
      sanitizeCatalogConfigurationValues(declaration, "customer", {
        credential: "visible secret",
        region: "north",
      }),
    ).toEqual({ region: "north" });
    expect(
      catalogConfigurationValuesToRows(declaration, "product", {
        credential: "visible secret",
      }),
    ).toEqual([]);
  });

  it("preserves server-provided dynamic selections while their governed loader is unavailable", () => {
    const schema: CatalogConfigurationDeclaration = {
      fields: [
        {
          dynamicCandidates: {
            method: "configuration.candidates",
            version: "v1",
          },
          key: "plan",
          label,
          type: "single_select",
        },
      ],
      version: "v1",
    };
    expect(
      catalogConfigurationValuesToRows(schema, "product", {
        plan: "opaque-existing-choice",
      }),
    ).toEqual([{ fieldKey: "plan", optionTokens: ["opaque-existing-choice"] }]);
  });

  it("keeps 64-bit integer input as exact text", () => {
    const field = { key: "count", label, type: "integer" as const };
    const schema = { fields: [field], version: "v1" };
    expect(
      validateCatalogConfigurationDraftValue(schema, field, {
        count: "9223372036854775807",
      }),
    ).toBe("");
    expect(
      catalogConfigurationValuesToRows(schema, "product", {
        count: "9223372036854775807",
      }),
    ).toEqual([{ fieldKey: "count", integerValue: "9223372036854775807" }]);
    expect(
      validateCatalogConfigurationDraftValue(schema, field, {
        count: "9223372036854775808",
      }),
    ).toBe("integer");
    expect(
      catalogConfigurationValuesToRows(schema, "product", {
        count: "9223372036854775808",
      }),
    ).toEqual([{ fieldKey: "count", integerValue: "9223372036854775808" }]);
  });
});

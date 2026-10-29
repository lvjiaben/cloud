import { describe, expect, it } from "vitest";
import type {
  CatalogConfigurationDeclaration,
  CatalogConfigurationField,
} from "./catalog-configuration-state";
import {
  type OfferPreviewField,
  previewRelationshipConditions,
  resolveOfferPreview,
} from "./catalog-offer-preview-state";

it("keeps hidden defaults in dependent selections and rejects caller overrides", () => {
  const fields: OfferPreviewField[] = [
    { ...choice("mode", ["basic", "advanced"]), customerVisible: false, defaultValue: "advanced" },
    { key: "units", type: "integer", label: { zhCN: "份数", enUS: "Units" }, minimum: "0", maximum: "10", customerVisible: false, defaultValue: 0 },
    { key: "enabled", type: "boolean", label: { zhCN: "启用", enUS: "Enabled" }, customerVisible: false, defaultValue: false },
    choice("plan", ["basic", "fast"]),
  ];
  const declaration: CatalogConfigurationDeclaration = { version: "v1", fields, conditions: [{
    when: { operator: "equal", field: "mode", value: "advanced" },
    actions: [{ action: "filter_candidates", field: "plan", values: ["fast"] }],
  }] };
  const supplied = { mode: "basic", units: 9, enabled: true, plan: "basic" };
  const result = resolveOfferPreview(declaration, supplied);
  expect(result.fields.map(field => field.key)).toEqual(["plan"]);
  expect(result.values).toEqual({ mode: "advanced", units: 0, enabled: false, plan: "fast" });
  expect(result.fields[0]!.staticCandidates!.map(row => row.value)).toEqual(["fast"]);
  expect(supplied).toEqual({ mode: "basic", units: 9, enabled: true, plan: "basic" });
});

it("maps provider relationship values back to stable commercial options", () => {
  const fields: OfferPreviewField[] = [
    {
      ...choice("commercial_family", ["dv", "ev"]),
      productFieldKey: "supplier_family",
      productChoiceBindings: { dv: "vendor-1", ev: "vendor-2" },
    },
    {
      ...choice("commercial_validation", ["file", "dns"]),
      productFieldKey: "supplier_validation",
      productChoiceBindings: { file: "http-01", dns: "dns-01" },
    },
  ];
  const before = JSON.stringify(fields);
  const conditions = previewRelationshipConditions(fields, [], {}, [
    {
      parentFieldKey: "supplier_family",
      childFieldKey: "supplier_validation",
      links: [
        { parentValue: "vendor-1", childValue: "http-01" },
        { parentValue: "vendor-1", childValue: "dns-01" },
        { parentValue: "vendor-2", childValue: "dns-01" },
      ],
    },
  ]);
  const result = resolveOfferPreview(
    { version: "v1", fields, conditions },
    { commercial_family: "ev", commercial_validation: "file" },
  );
  expect(result.values.commercial_family).toBe("ev");
  expect(result.values.commercial_validation).toBe("dns");
  expect(
    result.fields
      .find((field) => field.key === "commercial_validation")
      ?.staticCandidates?.map((value) => value.value),
  ).toEqual(["dns"]);
  expect(JSON.stringify(fields)).toBe(before);
});

const choice = (key: string, values: string[]): CatalogConfigurationField => ({
  key,
  type: "single_select",
  label: { zhCN: key, enUS: key },
  staticCandidates: values.map((value) => ({
    value,
    label: { zhCN: value, enUS: value },
  })),
});

describe("declarative local offer preview", () => {
  it("rebuilds mapped directory restrictions for unsaved choices while retaining provider rules", () => {
    const fields = [
      {
        ...choice("commercial_family", ["ev", "dv"]),
        productFieldKey: "certificate_family",
      },
      {
        ...choice("commercial_validation", ["file", "dns", "email"]),
        productFieldKey: "validation_method",
      },
    ];
    const old = {
      when: {
        operator: "equal" as const,
        field: "commercial_family",
        value: "dv",
      },
      actions: [
        {
          action: "filter_candidates" as const,
          field: "commercial_validation",
          values: ["dns"],
        },
      ],
    };
    const provider = {
      when: {
        operator: "equal" as const,
        field: "commercial_family",
        value: "ev",
      },
      actions: [{ action: "require" as const, field: "commercial_validation" }],
    };
    const relationships = [
      {
        parentFieldKey: "certificate_family",
        childFieldKey: "validation_method",
        links: [
          { parentValue: "dv", childValue: "dns" },
          { parentValue: "dv", childValue: "email" },
          { parentValue: "ev", childValue: "email" },
        ],
      },
    ];
    const before = JSON.stringify({ fields, old, provider, relationships });
    const conditions = previewRelationshipConditions(
      fields,
      [old, provider],
      { commercial_validation: [old] },
      relationships,
    );
    expect(conditions).toHaveLength(3);
    expect(conditions[0]).toEqual(provider);
    const preview = resolveOfferPreview(
      { version: "v1", fields, conditions },
      { commercial_family: "dv", commercial_validation: "email" },
    );
    expect(preview.values.commercial_validation).toBe("email");
    expect(
      preview.fields[1]!.staticCandidates!.map((row) => row.value),
    ).toEqual(["dns", "email"]);
    const changed = fields.map((field) =>
      field.key === "commercial_validation"
        ? {
            ...field,
            staticCandidates: field.staticCandidates!.filter(
              (row) => row.value !== "email",
            ),
          }
        : field,
    );
    const empty = resolveOfferPreview(
      {
        version: "v1",
        fields: changed,
        conditions: previewRelationshipConditions(
          changed,
          [old, provider],
          { commercial_validation: [old] },
          relationships,
        ),
      },
      {},
    );
    expect(empty.values.commercial_validation).toBeUndefined();
    expect(JSON.stringify({ fields, old, provider, relationships })).toBe(
      before,
    );
  });

  it("never guesses missing or ambiguous mappings, and removes only the owned count of identical rules", () => {
    const fields = [choice("parent", ["a"]), choice("child", ["x"])];
    const rule = {
      when: { operator: "equal" as const, field: "parent", value: "a" },
      actions: [
        { action: "filter_candidates" as const, field: "child", values: ["x"] },
      ],
    };
    const relationships = [
      {
        parentFieldKey: "parent",
        childFieldKey: "child",
        links: [{ parentValue: "a", childValue: "x" }],
      },
    ];
    expect(
      previewRelationshipConditions(fields, [rule], {}, relationships),
    ).toEqual([rule]);
    const mapped = fields.map((field) => ({
      ...field,
      productFieldKey: field.key,
    }));
    const changed = [
      {
        ...mapped[0]!,
        staticCandidates: choice("parent", ["b"]).staticCandidates,
      },
      mapped[1]!,
    ];
    const conditions = previewRelationshipConditions(
      changed,
      [rule, rule],
      { child: [rule] },
      relationships,
    );
    expect(conditions).toHaveLength(2);
    expect(conditions[0]).toEqual(rule);
    expect(conditions[1]!.actions[0]!.values).toEqual([]);
    expect(
      previewRelationshipConditions(
        [
          ...mapped,
          { ...choice("duplicate", ["a"]), productFieldKey: "parent" },
        ],
        [rule],
        { child: [rule] },
        relationships,
      ),
    ).toEqual([rule]);
  });

  it("keeps legal children and resets only invalid choices independently of field order", () => {
    const declaration: CatalogConfigurationDeclaration = {
      version: "v1",
      fields: [
        choice("route", ["a", "shared", "b"]),
        choice("region", ["north", "south"]),
      ],
      conditions: [
        {
          when: { operator: "equal", field: "region", value: "north" },
          actions: [
            {
              action: "filter_candidates",
              field: "route",
              values: ["a", "shared"],
            },
          ],
        },
        {
          when: { operator: "equal", field: "region", value: "south" },
          actions: [
            {
              action: "filter_candidates",
              field: "route",
              values: ["shared", "b"],
            },
          ],
        },
      ],
    };
    expect(resolveOfferPreview(declaration, {}).values).toEqual({
      region: "north",
      route: "a",
    });
    const supplied = { region: "south", route: "shared" };
    const retained = resolveOfferPreview(declaration, supplied);
    expect(retained.values.route).toBe("shared");
    expect(
      retained.fields[0]!.staticCandidates!.map((row) => row.value),
    ).toEqual(["shared", "b"]);
    expect(
      resolveOfferPreview(declaration, { region: "south", route: "a" }).values
        .route,
    ).toBe("shared");
    expect(supplied).toEqual({ region: "south", route: "shared" });
    expect(declaration.fields[0]!.staticCandidates).toHaveLength(3);
  });

  it("applies show, require and disable to typed numbers without exposing secrets", () => {
    const declaration: CatalogConfigurationDeclaration = {
      version: "v1",
      fields: [
        choice("mode", ["basic", "advanced"]),
        {
          key: "units",
          type: "integer",
          label: { zhCN: "份数", enUS: "Units" },
          minimum: "2",
          maximum: "10",
        },
        {
          key: "note",
          type: "single_line",
          label: { zhCN: "交付说明", enUS: "Delivery note" },
        },
        {
          key: "auth",
          type: "secret",
          label: { zhCN: "凭据", enUS: "Credential" },
        },
      ],
      conditions: [
        {
          when: { operator: "equal", field: "mode", value: "advanced" },
          actions: [{ action: "show", field: "note" }],
        },
        {
          when: { operator: "greater_or_equal", field: "units", value: "4" },
          actions: [{ action: "require", field: "note" }],
        },
        {
          when: { operator: "equal", field: "mode", value: "basic" },
          actions: [{ action: "disable", field: "units" }],
        },
      ],
    };
    const hidden = resolveOfferPreview(declaration, {
      mode: "basic",
      units: 4,
      note: "Old",
      auth: "not-for-preview",
    });
    expect(hidden.fields.map((field) => field.key)).toEqual(["mode", "units"]);
    expect(hidden.fields[1]!.disabled).toBe(true);
    expect(hidden.values).toEqual({ mode: "basic" });
    const shown = resolveOfferPreview(declaration, {
      mode: "advanced",
      units: 4,
    });
    expect(shown.fields.find((field) => field.key === "note")!.required).toBe(
      true,
    );
    expect(shown.values.units).toBe(4);
    expect(JSON.stringify(shown)).not.toContain("not-for-preview");
  });

  it("does not invent a numeric choice when conditional choices are empty and keeps a valid quantity step", () => {
    const declaration: CatalogConfigurationDeclaration = {
      version: "v1",
      fields: [
        choice("mode", ["none"]),
        { ...choice("size", ["2", "4"]), type: "integer" },
        {
          key: "quantity",
          type: "integer",
          minimum: "2",
          maximum: "10",
          quantityStep: "2",
          label: { zhCN: "数量", enUS: "Quantity" },
        } as CatalogConfigurationField,
      ],
      conditions: [
        {
          when: { operator: "equal", field: "mode", value: "none" },
          actions: [{ action: "filter_candidates", field: "size", values: [] }],
        },
      ],
    };
    expect(
      resolveOfferPreview(declaration, { size: "4", quantity: 6 }).values,
    ).toEqual({ mode: "none", size: undefined, quantity: 6 });
    expect(
      resolveOfferPreview(declaration, { quantity: 5 }).values.quantity,
    ).toBe(2);
  });
});

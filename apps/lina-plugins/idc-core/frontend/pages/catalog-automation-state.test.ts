import { describe, expect, it } from "vitest";

import { canWriteCatalogFulfillmentBinding } from "./catalog-automation-state";

describe("catalog automation permission gate", () => {
  it("requires both interface visibility and binding write access", () => {
    expect(canWriteCatalogFulfillmentBinding("draft", true, true)).toBe(true);
    expect(canWriteCatalogFulfillmentBinding("draft", false, true)).toBe(false);
    expect(canWriteCatalogFulfillmentBinding("draft", true, false)).toBe(false);
  });

  it("never changes a frozen published version", () => {
    expect(canWriteCatalogFulfillmentBinding("published", true, true)).toBe(
      false,
    );
    expect(canWriteCatalogFulfillmentBinding(undefined, true, true)).toBe(
      false,
    );
  });
});

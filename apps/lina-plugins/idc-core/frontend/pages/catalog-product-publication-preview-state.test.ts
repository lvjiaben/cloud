// This file verifies publication confirmation remains disabled for failed automation admission.

import { describe, expect, it } from "vitest";

import {
  publicationPreviewCanConfirm,
  type CatalogProductPublicationPreviewState,
} from "./catalog-product-publication-preview-state";

function preview(
  automationPassed: boolean,
): CatalogProductPublicationPreviewState {
  return {
    checks: [
      { code: "lifecycle", passed: true },
      { code: "automation", passed: automationPassed },
    ],
    ready: automationPassed,
  };
}

describe("catalog publication preview confirmation", () => {
  it("disables confirmation when automation admission fails", () => {
    expect(publicationPreviewCanConfirm(false, preview(false))).toBe(false);
  });

  it("allows confirmation only after all authoritative checks pass", () => {
    expect(publicationPreviewCanConfirm(false, preview(true))).toBe(true);
    expect(publicationPreviewCanConfirm(true, preview(true))).toBe(false);
    expect(publicationPreviewCanConfirm(false)).toBe(false);
  });
});

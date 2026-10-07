import { describe, expect, it } from "vitest";

import { parseCertificateItem, parseCertificatePage } from "./model";

const certificate = {
  availableActions: ["sync", "download", "unknown-action"],
  entitlementNo: "ENT_SSL_00000001",
  customerName: "示例客户",
  expiresAt: 1_810_000_000_000,
  issuedAt: 1_778_000_000_000,
  primaryDomain: "example.test",
  productNameEnUs: "Domain validated certificate",
  productNameZhCn: "域名型证书",
  statusCode: "issued",
  updatedAt: 1_778_000_100_000,
  validation: [{ domain: "example.test", host: "_dnsauth.example.test", method: "dns", recordType: "CNAME", value: "validation.example.test" }],
  validationMethod: "dns",
};

describe("certificate view model", () => {
  it("keeps only declared user-facing actions and structured validation data", () => {
    const item = parseCertificateItem(certificate);
    expect(item?.availableActions).toEqual(["sync", "download"]);
    expect(item?.validation[0]).toMatchObject({ method: "dns", recordType: "CNAME" });
  });

  it("fails closed for malformed status, validation method, or oversized pages", () => {
    expect(parseCertificateItem({ ...certificate, validationMethod: "shell" })).toBeNull();
    expect(parseCertificatePage({ items: Array.from({ length: 101 }, () => certificate), nextCursor: "" })).toBeNull();
  });
});

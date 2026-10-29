import { describe, it, expect } from "vitest";
import { expiry, systemIcon } from "./format";
describe("resource-owned server display", () => {
  it("uses exact expiry bands and hides countdown outside activation", () => {
    const now = new Date(2026, 9, 25, 12).getTime(),
      day = 86400000;
    for (const [days, tone] of [
      [31, "success"],
      [30, "warning"],
      [15, "warning"],
      [14, "danger"],
      [-1, "danger"],
    ] as const)
      expect(expiry(now + days * day, "active", now)).toMatchObject({
        days,
        tone,
      });
    expect(expiry(now, "active", now).date).toBe("2026-10-25");
    expect(expiry(now + day, "suspended", now)).toEqual({ date: "2026-10-26" });
    expect(expiry(null, "active", now)).toEqual({ date: "—" });
  });
  it("resolves operating system icons without guessing unknown systems", () => {
    expect(systemIcon("CentOS-9x64")).toBe("centos");
    expect(systemIcon("Debian 13")).toBe("debian");
    expect(systemIcon("Unknown")).toBeUndefined();
  });
});

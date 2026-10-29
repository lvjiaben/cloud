import { describe, expect, it } from "vitest";
import { expiryDisplay, operatingSystemIcon } from "./service-display";
describe("service display", () => {
  const now = new Date(2026, 9, 25, 12).getTime(),
    day = 86_400_000;
  it("formats date and remaining-day thresholds only for activated services", () => {
    for (const [days, tone] of [
      [31, "success"],
      [30, "warning"],
      [15, "warning"],
      [14, "danger"],
    ] as const) {
      expect(expiryDisplay(now + days * day, "active", now)).toMatchObject({
        remaining: `[${days}天后过期]`,
        tone,
      });
    }
    expect(expiryDisplay(now, "active", now)).toMatchObject({
      date: "2026-10-25",
      remaining: "[今日到期]",
      tone: "danger",
    });
    expect(expiryDisplay(now - day, "active", now).remaining).toBe(
      "[已过期1天]",
    );
    expect(expiryDisplay(now + day, "suspended", now)).toEqual({
      date: "2026-10-26",
    });
    expect(expiryDisplay(null, "active", now)).toEqual({ date: "—" });
  });
  it("uses OS icons and a neutral unknown-system icon", () => {
    expect(operatingSystemIcon("CentOS-9x64")).toBe("simple-icons:centos");
    expect(operatingSystemIcon("Debian 13")).toBe("simple-icons:debian");
    expect(operatingSystemIcon("other")).toBe("lucide:monitor");
  });
});

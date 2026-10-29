// Customer-facing service lifecycle and expiry formatting, independent of power state.
export function expiryDisplay(
  expiresAt: number | null | undefined,
  status: string,
  now = Date.now(),
) {
  if (!expiresAt || !Number.isFinite(expiresAt)) return { date: "—" };
  const value = new Date(expiresAt);
  if (!Number.isFinite(value.getTime())) return { date: "—" };
  const date = `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, "0")}-${String(value.getDate()).padStart(2, "0")}`;
  if (status !== "active") return { date };
  const days = Math.ceil((expiresAt - now) / 86_400_000);
  return {
    date,
    remaining:
      days > 0
        ? `[${days}天后过期]`
        : days === 0
          ? "[今日到期]"
          : `[已过期${Math.abs(days)}天]`,
    tone: days > 30 ? "success" : days >= 15 ? "warning" : "danger",
  };
}

export function operatingSystemIcon(name: string) {
  const icons: [RegExp, string][] = [
    [/windows/i, "simple-icons:windows"],
    [/centos/i, "simple-icons:centos"],
    [/ubuntu/i, "simple-icons:ubuntu"],
    [/debian/i, "simple-icons:debian"],
    [/rocky/i, "simple-icons:rockylinux"],
    [/alma/i, "simple-icons:almalinux"],
    [/fedora/i, "simple-icons:fedora"],
    [/freebsd/i, "simple-icons:freebsd"],
    [/linux/i, "simple-icons:linux"],
  ];
  return icons.find(([match]) => match.test(name))?.[1] || "lucide:monitor";
}

// Resource-owned display rules; server lifecycle and runtime power state stay distinct.
export function expiry(value: number | null, status: string, now = Date.now()) {
  if (!value || !Number.isFinite(value)) return { date: "—" };
  if (!Number.isFinite(new Date(value).getTime())) return { date: "—" };
  const d = new Date(value),
    date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  if (status !== "active") return { date };
  const days = Math.ceil((value - now) / 86400000);
  return {
    date,
    days,
    tone: days > 30 ? "success" : days >= 15 ? "warning" : "danger",
  };
}
export function systemIcon(name: string) {
  return [
    [/windows/i, "windows"],
    [/centos/i, "centos"],
    [/ubuntu/i, "ubuntu"],
    [/debian/i, "debian"],
    [/rocky/i, "rockylinux"],
    [/alma/i, "almalinux"],
    [/fedora/i, "fedora"],
    [/freebsd/i, "freebsd"],
    [/linux/i, "linux"],
  ].find(([match]) => (match as RegExp).test(name))?.[1] as string | undefined;
}

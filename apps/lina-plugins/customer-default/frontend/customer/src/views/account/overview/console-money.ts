// Format integer minor units without converting financial values to Number.
export function consoleMoney(
  minor: string,
  currency: string,
  locale: string,
): string {
  try {
    const formatter = new Intl.NumberFormat(locale, {
      style: "currency",
      currency,
    });
    const digits = formatter.resolvedOptions().maximumFractionDigits ?? 2;
    const value = BigInt(minor);
    const absolute = value < 0n ? -value : value;
    const scale = BigInt(`1${"0".repeat(digits)}`);
    const whole = absolute / scale;
    const fraction = (absolute % scale).toString().padStart(digits, "0");
    const signed = value < 0n ? -(whole || 1n) : whole;
    return formatter
      .formatToParts(signed)
      .map((part) => {
        if (part.type === "fraction") return fraction;
        if (part.type === "integer" && whole === 0n) return "0";
        return part.value;
      })
      .join("");
  } catch {
    return "—";
  }
}

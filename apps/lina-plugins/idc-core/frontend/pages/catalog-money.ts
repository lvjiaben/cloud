const maximumMinorAmount = 9_223_372_036_854_775_807n;

function fractionDigits(currencyCode: string) {
  return new Intl.NumberFormat("en-US", {
    currency: currencyCode.trim().toUpperCase(),
    style: "currency",
  }).resolvedOptions().maximumFractionDigits ?? 2;
}

export function minorToMajorAmount(amountMinor: string, currencyCode: string) {
  const value = amountMinor.trim();
  if (!/^\d{1,19}$/u.test(value)) throw new Error("invalid_minor_amount");
  const digits = fractionDigits(currencyCode);
  if (digits === 0) return BigInt(value).toString();
  const padded = value.padStart(digits + 1, "0");
  const integer = padded.slice(0, -digits).replace(/^0+(?=\d)/u, "");
  const fraction = padded.slice(-digits);
  return `${integer}.${fraction}`;
}

export function majorToMinorAmount(amountMajor: string, currencyCode: string) {
  const value = amountMajor.trim();
  if (!/^\d{1,19}(?:\.\d{1,4})?$/u.test(value))
    throw new Error("invalid_major_amount");
  const digits = fractionDigits(currencyCode);
  const [integer, fraction = ""] = value.split(".");
  if (fraction.length > digits) throw new Error("invalid_major_amount");
  const minor = BigInt(`${integer}${fraction.padEnd(digits, "0")}`);
  if (minor > maximumMinorAmount) throw new Error("invalid_major_amount");
  return minor.toString();
}

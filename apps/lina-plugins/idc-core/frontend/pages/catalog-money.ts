const maximumMinorAmount = "9223372036854775807";

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
  if (digits === 0) return value.replace(/^0+(?=\d)/u, "");
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
  const minor = `${integer}${fraction.padEnd(digits, "0")}`.replace(/^0+(?=\d)/u, "");
  if (compareMinorAmounts(minor, maximumMinorAmount) > 0) throw new Error("invalid_major_amount");
  return minor;
}

// Compare canonical nonnegative minor-unit strings without floating-point conversion.
export function compareMinorAmounts(left: string, right: string) {
 const a = left.replace(/^0+(?=\d)/u, ''), b = right.replace(/^0+(?=\d)/u, '');
 if (!/^\d+$/u.test(a) || !/^\d+$/u.test(b)) throw new Error('invalid_minor_amount');
 return a.length === b.length ? (a < b ? -1 : a > b ? 1 : 0) : a.length < b.length ? -1 : 1;
}

// Coefficients are decimal strings. Round half up once, at the currency's minor
// unit; never multiply money with JavaScript floating point.
export function multiplyMajorAmount(base: string, coefficient: string, currencyCode: string) {
  if (!/^(?:0|[1-9]\d{0,5})(?:\.\d{1,6})?$/u.test(coefficient.trim())) throw new Error('invalid_coefficient');
  const [whole, fraction = ''] = coefficient.trim().split('.');
  const numerator = BigInt(`${whole}${fraction}`);
  const denominator = BigInt(`1${"0".repeat(fraction.length)}`);
  const product = BigInt(majorToMinorAmount(base, currencyCode)) * numerator;
  const rounded = (product + denominator / BigInt(2)) / denominator;
  if (rounded > BigInt(maximumMinorAmount)) throw new Error('invalid_major_amount');
  return minorToMajorAmount(String(rounded), currencyCode);
}

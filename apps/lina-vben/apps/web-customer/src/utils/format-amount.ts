export function currencyFractionDigits(currencyCode: string) {
  try {
    return new Intl.NumberFormat('en', { currency: currencyCode, style: 'currency' })
      .resolvedOptions().maximumFractionDigits ?? 2;
  } catch {
    return 2;
  }
}

/** Format an integer minor-unit string without passing it through JS Number. */
export function formatMinorAmount(amountMinor: string, currencyCode: string) {
  const sign = amountMinor.startsWith('-') ? '-' : '';
  const digits = amountMinor.replace(/^-/, '').replace(/^0+(?=\d)/, '');
  const scale = currencyFractionDigits(currencyCode);
  if (scale === 0) return `${currencyCode} ${sign}${digits || '0'}`;
  const padded = (digits || '0').padStart(scale + 1, '0');
  return `${currencyCode} ${sign}${padded.slice(0, -scale)}.${padded.slice(-scale)}`;
}

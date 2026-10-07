export interface GovernedQuoteConfigurationState {
  configurationFrozen: boolean;
  configurationRequired: boolean;
}

export function hasPendingQuoteConfiguration(
  items: GovernedQuoteConfigurationState[],
) {
  return items.some(
    (item) => item.configurationRequired && !item.configurationFrozen,
  );
}

export function quoteConfigurationAction(
  item: GovernedQuoteConfigurationState,
) {
  if (!item.configurationRequired) return 'none' as const;
  return item.configurationFrozen
    ? ('reconfigure' as const)
    : ('configure' as const);
}

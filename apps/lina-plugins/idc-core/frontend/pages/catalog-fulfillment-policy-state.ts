export type CatalogFulfillmentStrategy =
  | "approval_then_plugin"
  | "manual"
  | "payment_confirmed_plugin";

export const catalogFulfillmentStrategies: readonly CatalogFulfillmentStrategy[] =
  ["manual", "approval_then_plugin", "payment_confirmed_plugin"];

export function fulfillmentStrategyRequiresBinding(
  strategy: CatalogFulfillmentStrategy,
) {
  return strategy !== "manual";
}

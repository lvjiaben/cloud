import { pluginApiPath, requestClient } from '#/api/request';

import type { PurchaseValue } from './purchase-configuration';

const corePluginID = 'idc-core';

export interface CartItem {
  cartItemNo: string;
  issueCode: string;
  productNameEnUS: string;
  productNameZhCN: string;
  publicSlug: string;
  configurationRequired: boolean;
  configurationFrozen: boolean;
  configurationRevision?: string;
  purchaseCycles: number;
  repairAction: string;
  quantity: number;
  statusCode: string;
  version: number;
}

export interface Cart {
  cartNo: string;
  currencyCode: string;
  items: CartItem[];
  latestQuote?: FinalQuote;
  version: number;
}

export interface QuoteItem {
  adjustments: Array<{
    amountMinor: string;
    sourceReference: string;
    typeCode: string;
  }>;
  cartItemNo: string;
  discountMinor: string;
  productNameEnUS: string;
  productNameZhCN: string;
  publicSlug: string;
  configurationRequired: boolean;
  configurationFrozen: boolean;
  configurationRevision?: string;
  purchaseCycles: number;
  quoteItemNo: string;
  quantity: number;
  subtotalMinor: string;
  taxAmountMinor: string;
  totalAmountMinor: string;
}

export interface FinalQuote {
  cartNo: string;
  cartVersion: number;
  currencyCode: string;
  discountMinor: string;
  expiresAt: number;
  issuedAt: number;
  items: QuoteItem[];
  netAmountMinor: string;
  quoteNo: string;
  subtotalMinor: string;
  taxAmountMinor: string;
  totalAmountMinor: string;
}

export interface Checkout {
  currencyCode: string;
  invoiceNo: string;
  itemCount: number;
  orderNo: string;
  paymentDueAt: number;
  quoteNo: string;
  requiresContract: boolean;
  reused: boolean;
  statusCode: string;
  totalAmountMinor: string;
}

export interface PaymentPreparation {
  amountDueMinor: string;
  contextNo: string;
  currencyCode: string;
  customerTypeCode: string;
  identityStatusCode: string;
  invoiceNo: string;
  lastEvaluatedAt: number;
  orderNo: string;
  paymentDueAt: number;
  ready: boolean;
  reasonCode: string;
  reused: boolean;
  statusCode: string;
}

export interface OrderSummary {
  canCancel: boolean;
  canPreparePayment: boolean;
  createdAt: number;
  currencyCode: string;
  itemCount: number;
  orderNo: string;
  paymentDueAt?: number;
  requiresContract: boolean;
  riskDecisionCode: string;
  statusCode: string;
  totalAmountMinor: string;
}

export interface OrderDetail {
  cancellation?: Record<string, number | string>;
  deliveries: Delivery[];
  invoice?: Record<string, number | string>;
  items: Array<Record<string, boolean | number | string>>;
  paymentPreparation?: {
    contextNo: string;
    expiresAt?: number;
    identityStatusCode: string;
    lastEvaluatedAt?: number;
    reasonCode: string;
    statusCode: string;
  };
  reviews: Array<Record<string, number | string>>;
  serviceEntitlements: ServiceEntitlement[];
  summary: OrderSummary;
}

export interface Delivery {
  completedAt?: number;
  ongoingService: boolean;
  productNameEnUS: string;
  productNameZhCN: string;
  quantity: number;
  startedAt?: number;
  statusCode:
    | 'awaiting_payment'
    | 'awaiting_review'
    | 'cancelled'
    | 'completed'
    | 'not_completed'
    | 'processing';
  updatedAt: number;
}

export interface ServiceEntitlement {
  entitlementNo: string;
  expiresAt?: number;
  orderItemNo: string;
  productNameEnUS: string;
  productNameZhCN: string;
  startedAt: number;
  statusCode: 'active' | 'expired' | 'suspended' | 'terminated';
}

export interface OrderPage {
  nextCursor: string;
  orders: OrderSummary[];
}

export interface AddCartItemInput {
  billingTermToken: string;
  configurationRevision: string;
  currencyCode: string;
  expectedCartVersion: number;
  publicSlug: string;
  purchaseCycles: number;
  quantity: number;
  values: PurchaseValue[];
}

function path(pathName: string) {
  return pluginApiPath(corePluginID, `account/commerce/${pathName}`);
}

export function getCart() {
  return requestClient.get<{ cart: Cart }>(path('cart'));
}

export function mergeGuestCart() {
  return requestClient.post<{ cart?: Cart; merged: boolean }>(
    path('guest-cart-merge'),
  );
}

export function openCart(currencyCode: string) {
  return requestClient.post<{ cart: Cart }>(path('carts'), { currencyCode });
}

export function addCartItem(input: AddCartItemInput) {
  return requestClient.post<{ cart: Cart }>(path('cart-items'), input);
}

export function removeCartItem(
  cartItemNo: string,
  expectedCartVersion: number,
  expectedItemVersion: number,
) {
  return requestClient.delete<{ cart: Cart }>(
    path(`cart-items/${encodeURIComponent(cartItemNo)}`),
    { params: { expectedCartVersion, expectedItemVersion } },
  );
}

export function createFinalQuote(expectedCartVersion: number) {
  return requestClient.post<{ quote: FinalQuote }>(path('quotes'), {
    assetNumbers: [],
    expectedCartVersion,
  });
}

export function checkoutQuote(quoteNo: string, idempotencyKey: string) {
  return requestClient.post<{ checkout: Checkout }>(path('orders'), {
    idempotencyKey,
    quoteNo,
  });
}

export function preparePayment(orderNo: string) {
  return requestClient.post<{ preparation: PaymentPreparation }>(
    path(`orders/${encodeURIComponent(orderNo)}/payment-preparations`),
  );
}

export function resumePaymentPreparation(contextNo: string) {
  return requestClient.put<{ preparation: PaymentPreparation }>(
    path(`payment-preparations/${encodeURIComponent(contextNo)}`),
  );
}

export function cancelOrder(orderNo: string, idempotencyKey: string) {
  return requestClient.put<{
    cancellation: Record<string, number | string>;
  }>(path(`orders/${encodeURIComponent(orderNo)}/cancellation`), {
    idempotencyKey,
  });
}

export function listOrders(
  params: {
    cursor?: string;
    limit?: number;
    statusCode?: string;
  } = {},
) {
  return requestClient.get<OrderPage>(path('orders'), {
    params: { limit: 20, ...params },
  });
}

export function getOrder(orderNo: string) {
  return requestClient.get<{ order: OrderDetail }>(
    path(`orders/${encodeURIComponent(orderNo)}`),
  );
}

export function persistentCommerceActionKey(scope: string) {
  const storageKey = `lina-web-customer-commerce.action.${scope}`;
  const existing = window.sessionStorage.getItem(storageKey);
  if (existing) return existing;
  const value = `customer-${crypto.randomUUID()}`;
  window.sessionStorage.setItem(storageKey, value);
  return value;
}

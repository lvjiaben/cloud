import { requestClient, type RequestOptions } from "../request";
// Fetch errors retain HTTP status directly; no framework transport transforms needed.
export function requestWithHttpStatus<T>(
  client: typeof requestClient,
  url: string,
  config: RequestOptions = {},
) {
  return client.request<T>(url, config);
}

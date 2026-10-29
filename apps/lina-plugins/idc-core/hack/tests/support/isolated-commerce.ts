// Dedicated-database integration guards and clients shared by commerce tests.
// No fixture secret or customer password is written to Playwright output.
import { readFile, realpath } from "node:fs/promises";
import { isAbsolute, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { APIRequestContext } from "@host-tests/support/playwright";
import { expect, request } from "@host-tests/support/playwright";

export interface Fixture {
  protocol: string;
  databaseName: string;
  root: string;
  baseURL: string;
  mockOrigin: string;
  orderNo: string;
  attemptNo: string;
  serverNo: string;
  channelNo: string;
  resourceChannelNo: string;
  expectedAddress: string;
  expectedHostname: string;
  expectedTaskId: number;
  expectedCloudId: number;
  adminAuth: string;
  customerAuth: string;
  foreignAuth: string;
  checkout: string;
  callback: string;
  mockState: string;
}
export const descriptor = process.env.IDC_ISOLATED_COMMERCE_FIXTURE;
const tempRoot = fileURLToPath(
  new URL("../../../../../../temp/", import.meta.url),
);
async function confined(path: string, root: string) {
  const actual = await realpath(path);
  const rel = relative(await realpath(root), actual);
  expect(
    !isAbsolute(rel) &&
      rel !== ".." &&
      !rel.startsWith(".." + (process.platform === "win32" ? "\\" : "/")),
  ).toBe(true);
  return actual;
}
export async function fixture() {
  const file = await confined(resolve(descriptor!), tempRoot);
  const f: Fixture = JSON.parse(await readFile(file, "utf8"));
  expect(f.protocol).toBe("idc-isolated-commerce-v1");
  expect(f.databaseName).toMatch(/^cloud_nightly_[0-9a-f]{12}$/);
  await confined(f.root, tempRoot);
  const origin = new URL(f.baseURL);
  expect(origin.protocol).toBe("http:");
  expect(origin.hostname).toBe("127.0.0.1");
  expect(["6030", "6031", "6032", "6680"]).not.toContain(origin.port);
  expect(origin.pathname).toBe("/");
  const mock = new URL(f.mockOrigin);
  expect(mock.protocol).toBe("https:");
  expect(mock.hostname).toBe("127.0.0.1");
  expect(f.orderNo).toMatch(/^ORD_[0-9a-f]{32}$/);
  expect(f.channelNo).toBe("EPAY_NIGHTLY");
  expect(f.resourceChannelNo).toMatch(/^CHN_[0-9a-f]{20}$/);
  return f;
}
export async function saved<T = any>(f: Fixture, name: string): Promise<T> {
  const file = await confined(resolve(f.root, name), f.root);
  return JSON.parse(await readFile(file, "utf8"));
}
export async function api(f: Fixture, authFile: string) {
  const auth = await saved<{ accessToken: string }>(f, authFile);
  return request.newContext({
    baseURL: f.baseURL,
    extraHTTPHeaders: {
      Authorization: `Bearer ${auth.accessToken}`,
      "Accept-Language": "zh-CN",
    },
  });
}
export async function get<T = any>(
  client: APIRequestContext,
  path: string,
): Promise<T> {
  expect(path.startsWith("/x/") || path.startsWith("/api/v1/")).toBe(true);
  const response = await client.get(path, { maxRedirects: 0 });
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body.code).toBe(0);
  return body.data;
}
export const orderPath = (f: Fixture) =>
  `/x/idc-core/api/v1/account/commerce/orders/${f.orderNo}`;
export const serversPath = "/x/idc-resource/api/v1/account/native-servers";

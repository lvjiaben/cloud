import type { RequestClient } from "@vben/request";

export declare function pluginApiPath(pluginID: string, path: string): string;

interface TestRequestClient {
  instance: RequestClient["instance"];
  request: RequestClient["request"];
  delete<T = unknown>(url: string, ...args: unknown[]): Promise<T>;
  download<T = Blob>(url: string, ...args: unknown[]): Promise<T>;
  get<T = unknown>(url: string, ...args: unknown[]): Promise<T>;
  post<T = unknown>(url: string, ...args: unknown[]): Promise<T>;
  put<T = unknown>(url: string, ...args: unknown[]): Promise<T>;
}

export declare const requestClient: TestRequestClient;

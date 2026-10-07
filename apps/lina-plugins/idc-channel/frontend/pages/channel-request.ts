import type { RequestClient, RequestClientConfig } from "@vben/request";

/** Keep transport status through the shared client's body-only error handling. */
export async function requestPreservingHttpStatus<T>(
  client: Pick<RequestClient, "instance" | "request">,
  url: string,
  config: RequestClientConfig,
): Promise<T> {
  let responseStatus: number | undefined;
  const requestTransforms =
    config.transformRequest ?? client.instance.defaults.transformRequest ?? [];
  const responseTransforms =
    config.transformResponse ??
    client.instance.defaults.transformResponse ??
    [];

  try {
    return await client.request<T>(url, {
      ...config,
      transformRequest: [
        (data) => {
          // A refresh replay is a new attempt. Its network failure must not
          // inherit the previous response's 401 status.
          responseStatus = undefined;
          return data;
        },
        ...[requestTransforms].flat(),
      ],
      transformResponse: [
        (data, _headers, status) => {
          responseStatus = status;
          return data;
        },
        ...[responseTransforms].flat(),
      ],
    });
  } catch (failure) {
    if (
      typeof responseStatus !== "number" ||
      !Number.isInteger(responseStatus) ||
      responseStatus < 100 ||
      responseStatus > 599
    ) {
      throw failure;
    }
    // Do not mutate a refresh failure that other queued requests may share.
    throw Object.assign(
      new Error(failure instanceof Error ? failure.message : "", {
        cause: failure,
      }),
      { response: { status: responseStatus, data: failure } },
    );
  }
}

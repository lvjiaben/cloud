import type { RequestClient, RequestClientConfig } from '@vben/request';

/** Preserve transport status across the existing client's token-refresh replay. */
export async function requestWithHttpStatus<T>(
  client: RequestClient,
  url: string,
  config: RequestClientConfig = {},
): Promise<T> {
  let responseStatus: number | undefined;
  const requestTransforms =
    config.transformRequest ?? client.instance.defaults.transformRequest;
  const responseTransforms =
    config.transformResponse ?? client.instance.defaults.transformResponse;

  try {
    return await client.request<T>(url, {
      ...config,
      transformRequest: [
        (data: unknown) => {
          // The official auth interceptor replays this same config. A new
          // attempt without a response must not inherit an earlier 401.
          responseStatus = undefined;
          return data;
        },
        ...(Array.isArray(requestTransforms)
          ? requestTransforms
          : requestTransforms
            ? [requestTransforms]
            : []),
      ],
      transformResponse: [
        (data: unknown, _headers: unknown, status?: number) => {
          responseStatus = status;
          return data;
        },
        ...(Array.isArray(responseTransforms)
          ? responseTransforms
          : responseTransforms
            ? [responseTransforms]
            : []),
      ],
    });
  } catch (failure) {
    if (
      responseStatus === undefined ||
      !Number.isInteger(responseStatus) ||
      responseStatus < 100 ||
      responseStatus > 599
    ) {
      throw failure;
    }
    // Never infer HTTP status from business codes or mutate a shared error.
    throw Object.assign(
      new Error(failure instanceof Error ? failure.message : '', {
        cause: failure,
      }),
      { response: { data: failure, status: responseStatus } },
    );
  }
}

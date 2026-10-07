import type { RequestClient, RequestClientConfig } from "@vben/request";

// Keep the shared client's authentication, encoding and response interceptors.
// Only transport metadata lost by its body-only error wrapper is restored.
export async function requestPreservingHttpStatus<T>(
  client: Pick<RequestClient, "instance" | "request">,
  url: string,
  config: RequestClientConfig,
): Promise<T> {
  let responseStatus: number | undefined;
  const requestTransforms =
    config.transformRequest ?? client.instance.defaults.transformRequest ?? [];
  const responseTransforms =
    config.transformResponse ?? client.instance.defaults.transformResponse ?? [];
  try {
    return await client.request<T>(url, {
      ...config,
      transformRequest: [
        (data) => {
          // A replay after token refresh starts a new attempt. A subsequent
          // network error must not be mistaken for the preceding HTTP 401.
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
      !Number.isInteger(responseStatus) ||
      responseStatus! < 100 ||
      responseStatus! > 599
    ) {
      throw failure;
    }
    // Never interpret a business code as HTTP status or mutate a refresh error
    // that may be shared with another queued request.
    throw Object.assign(
      new Error(failure instanceof Error ? failure.message : "", {
        cause: failure,
      }),
      { response: { status: responseStatus, data: failure } },
    );
  }
}

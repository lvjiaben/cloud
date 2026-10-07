import {
  authenticateResponseInterceptor,
  defaultResponseInterceptor,
  RequestClient,
} from "@vben/request";
import MockAdapter from "axios-mock-adapter";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { requestPreservingHttpStatus } from "./provider-request";

describe("provider directory transport status", () => {
  let client: RequestClient;
  let mock: MockAdapter;

  beforeEach(() => {
    client = new RequestClient({ responseReturn: "data" });
    client.addResponseInterceptor(
      defaultResponseInterceptor({
        codeField: "code",
        dataField: "data",
        successCode: 0,
      }),
    );
    mock = new MockAdapter(client.instance);
  });

  afterEach(() => {
    mock.restore();
    vi.restoreAllMocks();
  });

  function enableRefresh() {
    const refresh = vi.fn().mockResolvedValue("test-refreshed-token");
    const reauthenticate = vi.fn().mockResolvedValue(undefined);
    client.addResponseInterceptor(
      authenticateResponseInterceptor({
        client,
        doReAuthenticate: reauthenticate,
        doRefreshToken: refresh,
        enableRefreshToken: true,
        formatToken: (token) => `Bearer ${token}`,
      }),
    );
    return { refresh, reauthenticate };
  }

  it.each([401, 403, 503])(
    "preserves actual HTTP %i after body unwrapping",
    async (status) => {
      const body = { code: 999, message: "Test transport failure" };
      mock.onGet("/channel").reply(status, body);
      await expect(
        requestPreservingHttpStatus(client, "/channel", { method: "GET" }),
      ).rejects.toMatchObject({
        response: { status, data: body },
        cause: body,
      });
      expect(mock.history.get).toHaveLength(1);
    },
  );

  it("does not mistake a business code for a permission response", async () => {
    mock
      .onGet("/channel")
      .reply(200, { code: 403, message: "Business refusal" });
    await expect(
      requestPreservingHttpStatus(client, "/channel", { method: "GET" }),
    ).rejects.toMatchObject({ response: { status: 200 } });
    mock
      .onGet("/channel")
      .reply(503, { code: 401, message: "Service unavailable" });
    await expect(
      requestPreservingHttpStatus(client, "/channel", { method: "GET" }),
    ).rejects.toMatchObject({ response: { status: 503 } });
  });

  it.each([401, 403, 503])(
    "keeps final HTTP %i from an authenticated replay",
    async (status) => {
      const { refresh, reauthenticate } = enableRefresh();
      mock.onGet("/channel").replyOnce(401, { code: 999 });
      mock.onGet("/channel").reply(status, { code: 777 });
      await expect(
        requestPreservingHttpStatus(client, "/channel", { method: "GET" }),
      ).rejects.toMatchObject({ response: { status, data: { code: 777 } } });
      expect(refresh).toHaveBeenCalledTimes(1);
      expect(mock.history.get).toHaveLength(2);
      expect(reauthenticate).toHaveBeenCalledTimes(status === 401 ? 1 : 0);
    },
  );

  it("preserves the final successful data and original request encoding", async () => {
    const { refresh } = enableRefresh();
    const defaults = client.instance.defaults;
    const originalRequestTransforms = defaults.transformRequest;
    const originalResponseTransforms = defaults.transformResponse;
    const config = { method: "POST", data: { channelId: "test-channel" } };
    mock.onPost("/channel").replyOnce(401, { code: 999 });
    mock.onPost("/channel").reply(200, { code: 0, data: { saved: true } });
    await expect(
      requestPreservingHttpStatus(client, "/channel", config),
    ).resolves.toEqual({ saved: true });
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(mock.history.post.map((item) => item.data)).toEqual([
      '{"channelId":"test-channel"}',
      '{"channelId":"test-channel"}',
    ]);
    expect(config).toEqual({
      method: "POST",
      data: { channelId: "test-channel" },
    });
    expect(defaults.transformRequest).toBe(originalRequestTransforms);
    expect(defaults.transformResponse).toBe(originalResponseTransforms);
  });

  it.each(["network", "timeout"])(
    "does not reuse 401 after replay %s failure",
    async (kind) => {
      enableRefresh();
      mock.onGet("/channel").replyOnce(401, { code: 999 });
      if (kind === "network") mock.onGet("/channel").networkError();
      else mock.onGet("/channel").timeout();
      const failure = await requestPreservingHttpStatus(client, "/channel", {
        method: "GET",
      }).catch((error) => error);
      expect(failure).toBeInstanceOf(Error);
      expect(failure).not.toHaveProperty("response");
      expect(failure).toMatchObject({
        message: expect.stringMatching(
          kind === "network" ? /Network Error/ : /timeout/,
        ),
      });
      expect(mock.history.get).toHaveLength(2);
    },
  );

  it("keeps failed refresh tied to the real original 401 without mutating its error", async () => {
    const { refresh, reauthenticate } = enableRefresh();
    const refreshError = new Error("Test refresh failure");
    refresh.mockRejectedValue(refreshError);
    vi.spyOn(console, "error").mockImplementation(() => {});
    mock.onGet("/channel").reply(401, { code: 999 });
    await expect(
      requestPreservingHttpStatus(client, "/channel", { method: "GET" }),
    ).rejects.toMatchObject({ response: { status: 401 }, cause: refreshError });
    expect(refreshError).not.toHaveProperty("response");
    expect(reauthenticate).toHaveBeenCalledTimes(1);
    expect(mock.history.get).toHaveLength(1);
  });

  it("isolates status between concurrent calls", async () => {
    let resolveSlow!: (reply: [number, object]) => void;
    mock.onGet("/slow").reply(
      () =>
        new Promise((resolve) => {
          resolveSlow = resolve;
        }),
    );
    mock.onGet("/fast").reply(503, { code: 403 });
    const slow = requestPreservingHttpStatus(client, "/slow", {
      method: "GET",
    }).catch((error) => error);
    const fast = requestPreservingHttpStatus(client, "/fast", {
      method: "GET",
    }).catch((error) => error);
    await vi.waitFor(() => expect(resolveSlow).toBeTypeOf("function"));
    expect(await fast).toMatchObject({ response: { status: 503 } });
    resolveSlow([403, { code: 999 }]);
    expect(await slow).toMatchObject({ response: { status: 403 } });
  });

  it("runs caller transforms and captures status before a parser failure", async () => {
    const parserError = new Error("Test parser failure");
    const requestTransform = vi.fn((data) => JSON.stringify(data));
    const responseTransform = vi.fn(() => {
      throw parserError;
    });
    const config = {
      method: "POST",
      data: { page: 2 },
      transformRequest: [requestTransform],
      transformResponse: [responseTransform],
    };
    mock.onPost("/channel").reply(503, "not-json");
    await expect(
      requestPreservingHttpStatus(client, "/channel", config),
    ).rejects.toMatchObject({ response: { status: 503 }, cause: parserError });
    expect(requestTransform).toHaveBeenCalledTimes(1);
    expect(responseTransform).toHaveBeenCalledTimes(1);
    expect(mock.history.post[0]?.data).toBe('{"page":2}');
    expect(config.transformRequest).toEqual([requestTransform]);
    expect(config.transformResponse).toEqual([responseTransform]);
  });
});

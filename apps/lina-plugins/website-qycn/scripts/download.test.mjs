// Advance elapsed time without waiting minutes or imposing file-size limits.
import { test } from "node:test";
import assert from "node:assert/strict";
import { setImmediate } from "node:timers/promises";
import { requestDownload, safeURL } from "../frontend/api.js";

function transfer(t, { noHeaders = false } = {}) {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  // Make a regression to a fixed AbortSignal deadline observable by this clock.
  t.mock.method(AbortSignal, "timeout", (milliseconds) => {
    const controller = new AbortController();
    setTimeout(
      () => controller.abort(new DOMException("Timeout", "TimeoutError")),
      milliseconds,
    );
    return controller.signal;
  });
  let stream,
    signal,
    finished = false;
  t.mock.method(globalThis, "fetch", async (_, options) => {
    signal = options.signal;
    if (noHeaders)
      return new Promise((_, reject) =>
        signal.addEventListener("abort", () => reject(signal.reason), {
          once: true,
        }),
      );
    const body = new ReadableStream({
      start(controller) {
        stream = controller;
      },
    });
    signal.addEventListener(
      "abort",
      () => {
        if (!finished) stream.error(signal.reason);
      },
      { once: true },
    );
    return new Response(body, {
      headers: {
        "content-type": "application/octet-stream",
        "content-disposition": "attachment; filename=guide.bin",
      },
    });
  });
  return {
    get signal() {
      return signal;
    },
    async send() {
      stream.enqueue(new Uint8Array(1024));
      await setImmediate();
    },
    finish() {
      finished = true;
      stream.close();
    },
  };
}

test("continuous download can exceed two minutes without a total deadline", async (t) => {
  const source = transfer(t);
  const pending = requestDownload("review-token");
  // Attach rejection immediately, so a deadline regression is a normal assertion.
  const outcome = pending.then(
    (value) => ({ value }),
    (error) => ({ error }),
  );
  await setImmediate();
  for (let i = 0; i < 18; i++) {
    t.mock.timers.tick(10000);
    assert.equal(
      source.signal.aborted,
      false,
      `interrupted at ${(i + 1) * 10}s`,
    );
    await source.send();
  }
  source.finish();
  const result = await outcome;
  assert.equal(result.error, undefined);
  assert.equal(result.value.blob.size, 18 * 1024);
  assert.equal(result.value.filename, "guide.bin");
});

test("stalled headers and stalled body fail after inactivity", async (t) => {
  await t.test("headers", async (t) => {
    transfer(t, { noHeaders: true });
    const pending = assert.rejects(requestDownload("review-token"), {
      name: "TimeoutError",
    });
    await setImmediate();
    t.mock.timers.tick(30001);
    await pending;
  });
  await t.test("body", async (t) => {
    const source = transfer(t);
    const pending = assert.rejects(requestDownload("review-token"), {
      name: "TimeoutError",
    });
    await setImmediate();
    await source.send();
    t.mock.timers.tick(30001);
    await pending;
  });
});

test("untrusted navigation URLs cannot escape into executable schemes or relative authorities", () => {
  for (const value of [
    "//evil.test",
    "/\\evil.test",
    "/%2f%2fevil.test",
    "/%5cevil",
    "javascript:alert(1)",
    "data:text/html,hi",
    "https://name:secret@example.test/",
    "/x\n",
  ]) {
    assert.equal(safeURL(value, { external: true }), "", value);
  }
  assert.equal(safeURL("/products.html?q=cloud"), "/products.html?q=cloud");
  assert.equal(
    safeURL("https://example.test/help", { external: true }),
    "https://example.test/help",
  );
});

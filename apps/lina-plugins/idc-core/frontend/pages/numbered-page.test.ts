import { describe, expect, it, vi } from "vitest";
import { createNumberedPage } from "./numbered-page";

describe("authoritative numbered pagination", () => {
  it("clears the prior owner before another customer loads or fails", async () => {
    const publish = vi.fn();
    const reader = createNumberedPage<string>(publish);
    await reader.query({ currentPage: 1, pageSize: 10 }, async () => ({
      items: ["customer A"],
      total: 1,
    }));
    let finish!: (page: { items: string[]; total: number }) => void;
    const old = reader.query(
      { currentPage: 2, pageSize: 10 },
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    reader.reset();
    await reader.query({ currentPage: 1, pageSize: 10 }, async () => {
      throw { response: { status: 503 } };
    });
    finish({ items: ["customer A late"], total: 2 });
    expect(await old).toEqual({ items: [], total: 0 });
    expect(publish.mock.lastCall?.[0]).toEqual({
      currentPage: 1,
      pageSize: 10,
      items: [],
      total: 0,
    });
  });
  it("keeps exact totals, direct pages and resets page size changes", async () => {
    const publish = vi.fn();
    const reader = createNumberedPage<number>(publish);
    const read = vi.fn(async () => ({ items: [11], total: 125 }));
    await reader.query({ currentPage: 2, pageSize: 10 }, read);
    expect(read).toHaveBeenLastCalledWith(2, 10);
    expect(publish).toHaveBeenLastCalledWith({
      currentPage: 2,
      pageSize: 10,
      total: 125,
      items: [11],
    });
    await reader.query({ currentPage: 2, pageSize: 100 }, read);
    expect(read).toHaveBeenLastCalledWith(1, 100);
  });

  it.each([401, 403, 503])(
    "handles HTTP %i without inventing totals",
    async (status) => {
      const publish = vi.fn();
      const denied = vi.fn();
      const reader = createNumberedPage<number>(publish, denied);
      await reader.query({ currentPage: 2, pageSize: 10 }, async () => ({
        items: [11],
        total: 125,
      }));
      const result = await reader.query(
        { currentPage: 3, pageSize: 10 },
        async () => {
          throw { response: { status } };
        },
      );
      expect(result).toEqual(
        status === 503 ? { items: [11], total: 125 } : { items: [], total: 0 },
      );
      expect(denied).toHaveBeenCalledTimes(status === 503 ? 0 : 1);
      expect(publish.mock.lastCall?.[0].currentPage).toBe(
        status === 503 ? 2 : 1,
      );
    },
  );

  it("does not restore a slow authorized page after a newer denial", async () => {
    const publish = vi.fn();
    const reader = createNumberedPage<number>(publish);
    let resolve!: (value: { items: number[]; total: number }) => void;
    const old = reader.query(
      { currentPage: 2, pageSize: 10 },
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    await reader.query({ currentPage: 3, pageSize: 10 }, async () => {
      throw { response: { status: 403 } };
    });
    resolve({ items: [11], total: 125 });
    expect(await old).toEqual({ items: [], total: 0 });
    expect(publish).toHaveBeenCalledTimes(1);
  });

  it("never derives a total from the current page or publishes after disposal", async () => {
    const publish = vi.fn();
    const reader = createNumberedPage<number>(publish);
    await reader.query({ currentPage: 1, pageSize: 10 }, async () => ({
      items: [1],
      total: Number.NaN,
    }));
    expect(publish.mock.lastCall?.[0].total).toBe(0);
    let resolve!: (value: { items: number[]; total: number }) => void;
    const old = reader.query(
      { currentPage: 1, pageSize: 10 },
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    reader.invalidate();
    resolve({ items: [1], total: 1 });
    await old;
    expect(publish).toHaveBeenCalledTimes(1);
  });
});

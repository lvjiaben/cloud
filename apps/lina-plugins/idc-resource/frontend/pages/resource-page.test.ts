import { describe, expect, it, vi } from "vitest";

import { createResourcePage, isResourceAccessDenied } from "./resource-page";

function fixture() {
  const onDenied = vi.fn();
  const onFailure = vi.fn();
  const onPage = vi.fn();
  return { directory: createResourcePage<string>({ onDenied, onFailure, onPage }), onDenied, onFailure, onPage };
}

describe("confirmed resource numeric page", () => {
  it("keeps the authoritative total on random and empty pages", async () => {
    const { directory } = fixture();
    const load = vi.fn(async () => ({items: ["row-21"], total: 121}));
    await directory.query({currentPage: 3, pageSize: 10}, "", load);
    expect(load).toHaveBeenCalledWith(3, 10);
    expect(directory.current).toMatchObject({currentPage: 3, total: 121});
    await directory.query({currentPage: 99, pageSize: 10}, "", async () => ({items: [], total: 121}));
    expect(directory.current).toMatchObject({currentPage: 99, total: 121, items: []});
  });

  it.each([20, 50, 100])("resets size %s and changed filters to page one", async (pageSize) => {
    const { directory } = fixture();
    await directory.query({currentPage: 3, pageSize: 10}, "", async () => ({items: ["old"], total: 121}));
    const load = vi.fn(async () => ({items: ["new"], total: 81}));
    await directory.query({currentPage: 3, pageSize}, "", load);
    expect(load).toHaveBeenLastCalledWith(1, pageSize);
    await directory.query({currentPage: 4, pageSize}, "changed", load);
    expect(load).toHaveBeenLastCalledWith(1, pageSize);
  });

  it.each([401, 403])("clears the directory and selected details on real HTTP %s", async (status) => {
    const { directory, onDenied } = fixture();
    await directory.query({currentPage: 3, pageSize: 10}, "", async () => ({items: ["authorized"], total: 121}));
    await directory.query({currentPage: 4, pageSize: 10}, "", async () => { throw {response: {status}}; });
    expect(directory.current).toMatchObject({items: [], total: 0, currentPage: 1});
    expect(onDenied).toHaveBeenCalledOnce();
  });

  it.each([{response: {status: 503}}, new Error("network"), {code: 403}])("preserves confirmed data on a non-authorization failure", async (failure) => {
    const { directory, onDenied, onFailure } = fixture();
    await directory.query({currentPage: 3, pageSize: 10}, "", async () => ({items: ["authorized"], total: 121}));
    await directory.query({currentPage: 4, pageSize: 10}, "", async () => { throw failure; });
    expect(directory.current).toMatchObject({items: ["authorized"], total: 121, currentPage: 3});
    expect(onDenied).not.toHaveBeenCalled();
    expect(onFailure).toHaveBeenCalledOnce();
  });

  it("does not resurrect a late successful page after access is denied", async () => {
    const { directory } = fixture();
    let finish!: (result: {items: string[]; total: number}) => void;
    const stale = directory.query({currentPage: 2, pageSize: 10}, "", () => new Promise((resolve) => { finish = resolve; }));
    await directory.query({currentPage: 3, pageSize: 10}, "", async () => { throw {response: {status: 403}}; });
    finish({items: ["secret"], total: 121});
    expect(await stale).toMatchObject({items: [], total: 0});
    expect(directory.current.items).toEqual([]);
  });

  it("can retry after denial and rejects an invented total", async () => {
    const { directory, onFailure } = fixture();
    await directory.query({currentPage: 1, pageSize: 10}, "", async () => ({items: ["x"], total: 0}));
    expect(directory.current.items).toEqual([]);
    expect(onFailure).toHaveBeenCalledOnce();
    await directory.query({currentPage: 1, pageSize: 10}, "", async () => ({items: ["recovered"], total: 121}));
    expect(directory.current.total).toBe(121);
    expect(isResourceAccessDenied({code: 401})).toBe(false);
  });
});

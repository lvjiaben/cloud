import { afterEach, describe, expect, it, vi } from "vitest";
import { initialLoad, forgetInitialLoad } from "./initial-load";

afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); });
describe("页面初次加载隔离", () => {
  it("同次未提交渲染复用请求，提交后不复用业务数据", async () => {
    vi.useFakeTimers();
    const load=vi.fn(async()=>({value:1}));
    const first=initialLoad('resource',load,'navigation-1');
    expect(initialLoad('resource',load,'navigation-1')).toBe(first);
    await first.promise;
    expect(first.data).toEqual({value:1});
    expect(load).toHaveBeenCalledTimes(1);
    forgetInitialLoad('resource',first);
    await initialLoad('resource',load,'navigation-1').promise;
    expect(load).toHaveBeenCalledTimes(2);
  });
  it("取消切页后再进入使用新请求，不重用遗留结果", async()=>{
    vi.useFakeTimers();
    const first=initialLoad('same',async()=>1,'old-navigation');await first.promise;
    const next=initialLoad('same',async()=>2,'new-navigation');await next.promise;
    expect(next.data).toBe(2);
    const back=initialLoad('same',async()=>3,'old-navigation');await back.promise;
    expect(back.data).toBe(3);
  });
  it("身份变更后不能读取上一账户的初次结果",async()=>{
    vi.useFakeTimers();let token='first';
    vi.stubGlobal('sessionStorage',{getItem:()=>JSON.stringify({accessToken:token})});
    const first=initialLoad('profile',async()=>({name:'first'}),'identity-navigation');await first.promise;
    token='second';
    const second=initialLoad('profile',async()=>({name:'second'}),'identity-navigation');await second.promise;
    expect(second.data).toEqual({name:'second'});
  });
  it("失败被展示为可恢复错误，空响应也结束加载",async()=>{
    vi.useFakeTimers();
    const failure=initialLoad('error',async()=>{throw new Error('加载失败')},'error-navigation');await failure.promise;
    expect(failure.error).toBe('加载失败');expect(failure.settled).toBe(true);
    const empty=initialLoad('empty',async()=>undefined,'error-navigation');await empty.promise;
    expect(empty.settled).toBe(true);expect(empty.error).toBe('');
  });
});

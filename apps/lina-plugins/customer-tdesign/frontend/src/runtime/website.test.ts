import { describe, expect, it } from "vitest";
import { allowedWebsiteReturn, websiteOrigin, websiteTarget } from "./website";
const settings = {websiteurl:"https://shop.example", websitecatalogpath:"/services.html", websiteconfigurepath:"/configure.html?product={product}", websitecartpath:"/cart.html"};
describe("trusted website handoff", () => {
  it("preserves product, currency and quote item without forwarding arbitrary query arguments", () => {
    const target = websiteTarget(settings, "configure", "cloud/one", "?currency=USD&quoteItemNo=QI1&accessToken=bad&returnTo=https://evil.example");
    expect(target.origin).toBe("https://shop.example");
    expect(Object.fromEntries(target.searchParams)).toEqual({product:"cloud/one",currency:"USD",quoteItemNo:"QI1"});
  });
  it("rejects missing, credential-bearing, cross-origin and unconfigured destinations", () => {
    for(const value of ["", "javascript:alert(1)", "https://user:secret@shop.example", "https://shop.example/path"]) expect(() => websiteOrigin({websiteurl:value})).toThrow();
    for(const path of ["//evil.example/cart.html", "/\\evil.example", "/auth/logout", "https://evil.example/cart.html"]) expect(() => allowedWebsiteReturn(settings,path)).toThrow();
    expect(() => websiteTarget({...settings,websitecartpath:"//evil.example"},"cart")).toThrow();
    expect(allowedWebsiteReturn(settings,"/cart.html?coupon=test").origin).toBe("https://shop.example");
  });
});

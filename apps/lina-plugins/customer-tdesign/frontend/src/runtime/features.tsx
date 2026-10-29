import { createContext, useContext } from "react";
export const FeaturesContext = createContext<Set<string>>(new Set());
export function useFeature(id: string) {
  return useContext(FeaturesContext).has(id);
}
export function requiredFeature(path: string) {
  if (
    [
      "/account/verification",
      "/account/identity",
      "/account/enterprise-verification",
      "/account/tax-invoices",
    ].some((p) => path === p || path.startsWith(p + "/"))
  )
    return "idc-compliance";
  if (
    path === "/account/servers" ||
    path.startsWith("/account/native-servers/")
  )
    return "idc-resource";
  return "";
}

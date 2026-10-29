import { createContext } from "react";
import { servicePages, type ServicePagePlugin, type ServicePageBinding } from "./service-pages";
import {
  listCustomerProductDirectories,
  type CustomerProductDirectory,
} from "#/api/idc/product-directories";
import { listCustomerExternalMenus } from "#/api/idc/external-menu";
import { requestClient, externalURL, safePath } from "#/api/request";
export type NavItem = {
  title: string;
  path: string;
  icon?: string;
  external?: boolean;
  directoryNo?: string;
  navigationType?: CustomerProductDirectory["navigationType"];
  serviceCount?: number;
};
export type NavGroup = NavItem & {
  items?: NavItem[];
  icon: string;
};
export const NavigationContext = createContext<NavGroup[]>([]);
export function productNavigation(groups: NavGroup[]): NavItem[] {
  return groups
    .flatMap((group) => [group, ...(group.items || [])])
    .filter((item) => item.navigationType === "product" && item.directoryNo);
}
const compliancePaths = [
  "/account/verification",
  "/account/identity",
  "/account/enterprise-verification",
  "/account/tax-invoices",
];
export function assembleNavigation(
  directories: CustomerProductDirectory[],
  enabled: Set<string>,
  external: NavItem[] = [],
): NavGroup[] {
  const items = new Map<string, NavItem>();
  const unavailable = new Set<string>();
  const ordered = [...directories].sort((a,b)=>a.sortOrder-b.sortOrder||a.directoryNo.localeCompare(b.directoryNo));
  for (const directory of ordered) {
    const type=directory.navigationType || "product";
    const target=type==="product" ? `/account/product-directories/${encodeURIComponent(directory.directoryNo)}` : directory.targetUrl || "";
    const path=type==="custom" ? externalURL(target) : safePath(target);
    if (!path || /^\/account\/(?:contracts|cart|notifications)(?:[/?#]|$)/.test(path) ||
      /^\/account\/product-directories(?:\/all)?\/?$/.test(path) ||
      (type!=="custom" && !(path==="/account"||path.startsWith("/account/")))) continue;
    if ((compliancePaths.includes(path)&&!enabled.has("idc-compliance")) ||
      (path==="/account/servers"&&!enabled.has("idc-resource"))) unavailable.add(directory.directoryNo);
    items.set(directory.directoryNo,{
      title:directory.nameZhCn||directory.nameEnUs,path,icon:directory.icon||"lucide:folder",
      directoryNo:directory.directoryNo,navigationType:type,external:type==="custom",
      ...(type==="product"?{serviceCount:directory.serviceCount}:{}),
    });
  }
  // The shared data owns hierarchy; this template keeps its existing section headings.
  const childMenus=new Map<string,NavItem[]>();
  for(const directory of ordered){
    const item=items.get(directory.directoryNo);
    if(item&&directory.parentDirectoryNo&&!unavailable.has(directory.directoryNo)){const list=childMenus.get(directory.parentDirectoryNo)||[];list.push(item);childMenus.set(directory.parentDirectoryNo,list);}
  }
  const result: NavGroup[]=[];
  for(const directory of ordered){
    if(directory.parentDirectoryNo) continue;
    const item=items.get(directory.directoryNo);if(!item) continue;
    const children=childMenus.get(directory.directoryNo)||[];
    if(!children.length&&unavailable.has(directory.directoryNo)) continue;
    result.push({...item,icon:item.icon||"lucide:folder",...(children.length?{items:children}:{})});
  }
  const paths=new Set([...items.values()].map(item=>item.path));
  for(const item of external){const path=externalURL(item.path);if(path&&!paths.has(path)){
    paths.add(path);result.push({...item,path,external:true,icon:item.icon||"lucide:external-link"});
  }}
  return result;
}
export async function customerRuntime(): Promise<{
  groups: NavGroup[];
  enabled: Set<string>;
  servicePages: ServicePageBinding[];
}> {
  const [plugins, first, external] = await Promise.all([
    requestClient.get<{
      list: ServicePagePlugin[];
    }>("/plugins/dynamic"),
    listCustomerProductDirectories(1, 100),
    listCustomerExternalMenus(),
  ]);
  const all = [...first.items];
  for (let page = 2; page <= Math.ceil(first.total / 100); page += 4) {
    const pages = await Promise.all(
      Array.from(
        { length: Math.min(4, Math.ceil(first.total / 100) - page + 1) },
        (_, i) => listCustomerProductDirectories(page + i, 100),
      ),
    );
    all.push(...pages.flatMap((v) => v.items));
  }
  const enabled = new Set(
    plugins.list
      .filter((v) => v.installed === 1 && v.enabled === 1)
      .map((v) => v.id),
  );
  return {
    enabled,
    servicePages: servicePages(plugins.list),
    groups: assembleNavigation(
      all,
      enabled,
      external.items
        .filter((v) => v.enabled)
        .sort((a, b) => a.sortOrder - b.sortOrder)
        .map((v) => ({
          title: v.nameZh || v.nameEn,
          path: v.url,
          external: true,
        })),
    ),
  };
}

export async function customerNavigation() {
  return (await customerRuntime()).groups;
}

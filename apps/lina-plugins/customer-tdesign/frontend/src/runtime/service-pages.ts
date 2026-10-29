// Governed plugin-owned service pages; templates never define their business columns.
import { createContext } from "react";
export type ServicePageSurface = {
  pluginId: string;
  pluginType: "source" | "dynamic";
  key: string;
  version: string;
  title: string;
  icon?: string;
  slot: string;
  status?: string;
  entry: string;
  entryUrl?: string;
  bridgeCapabilities?: string[];
};
export type ServicePagePlugin = {
  id: string;
  version: string;
  installed: number;
  enabled: number;
  runtimeState?: string;
  customerSurfaces?: ServicePageSurface[];
};
export type ServicePageBinding = ServicePageSurface & { source: string };
export const ServicePagesContext = createContext<{
  pages: ServicePageBinding[];
  reload: () => void;
}>({ pages: [], reload: () => {} });
export function servicePages(
  plugins: ServicePagePlugin[],
): ServicePageBinding[] {
  return plugins.flatMap((plugin) => {
    if (
      plugin.installed !== 1 ||
      plugin.enabled !== 1 ||
      (plugin.runtimeState && plugin.runtimeState !== "normal")
    )
      return [];
    const bindings = (plugin.customerSurfaces || []).flatMap((surface) => {
      if (
        surface.pluginId !== plugin.id ||
        surface.slot !== "customer.products" ||
        (surface.status && surface.status !== "active") ||
        !["source", "dynamic"].includes(surface.pluginType) ||
        (surface.pluginType === "dynamic" && !surface.entryUrl) ||
        surface.entry.startsWith("/") ||
        !surface.bridgeCapabilities?.includes("purchased-services.customer.v1") ||
        !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(plugin.id) ||
        !/^v[0-9][A-Za-z0-9.+-]*$/.test(plugin.version) ||
        !/^[A-Za-z0-9/_-]+\.html$/.test(surface.entry) ||
        surface.entry.includes("..")
      )
        return [];
      const expected = `/x-assets/${plugin.id}/${plugin.version}/${surface.entry}`;
      if (surface.entryUrl && surface.entryUrl !== expected) return [];
      return [{ ...surface, source: expected }];
    });
    return bindings.length === 1 ? bindings : [];
  });
}

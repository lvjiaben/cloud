import { localizeData } from "../i18n.js";
// Static presentation content never becomes a purchasable catalog product.
export const slides = localizeData([
  {
    title: "Professional protection",
    accent: "Security for your cloud",
    sub: "Network protection · Elastic compute · A dependable foundation",
    tag: "CLOUD SECURITY",
    theme: "security",
    href: "/products.html",
    cta: "Explore security",
  },
  {
    title: "Cloud made simple",
    accent: "Give your business room to grow",
    sub: "From lightweight applications to enterprise workloads",
    tag: "ELASTIC CLOUD",
    theme: "compute",
    href: "/products.html",
    cta: "Shop cloud products",
  },
  {
    title: "Connected worldwide",
    accent: "Bring your users closer",
    sub: "Multi-region deployment · Flexible networks · Connected growth",
    tag: "GLOBAL NETWORK",
    theme: "network",
    href: "/datacenters.html",
    cta: "Explore data centers",
  },
  {
    title: "Dedicated performance",
    accent: "Built for your ambitions",
    sub: "Dedicated resources and expert support for your workloads",
    tag: "DEDICATED COMPUTING",
    theme: "compute",
    href: "/products.html",
    cta: "Explore hosting",
  },
]);
export const advantages = localizeData([
  [
    "shield",
    "Security",
    "Layered protection from network access to applications helps keep your business online.",
  ],
  [
    "globe",
    "Quality networks",
    "Flexible locations and network connections for a reliable experience across regions.",
  ],
  [
    "user",
    "Dedicated support",
    "Technical support for deployment, operation and the next stage of growth.",
  ],
  [
    "clock",
    "Simple and efficient",
    "Manage products and services in one customer center with clear status information.",
  ],
]);
export const locations = localizeData([
  {
    name: "Guangzhou",
    region: "China",
    detail: "A location for southern China workloads with multiple network options.",
  },
  {
    name: "Shanghai",
    region: "China",
    detail: "Connect to eastern China for websites, commerce and business applications.",
  },
  {
    name: "Hong Kong",
    region: "Asia Pacific",
    detail: "An illustrative location for cross-border connectivity and flexible deployment.",
  },
  {
    name: "Singapore",
    region: "Asia Pacific",
    detail: "A regional option for applications serving Southeast Asia.",
  },
  {
    name: "Los Angeles",
    region: "International",
    detail: "An illustrative North American location for global applications.",
  },
  {
    name: "Frankfurt",
    region: "International",
    detail: "An illustrative European location for planning distributed applications.",
  },
]);
export const contentKinds = localizeData({
  news: "News",
  announcements: "Announcements",
  knowledge: "Knowledge",
  downloads: "Downloads",
});
export const apiTypes = {
  news: "news",
  announcements: "announcement",
  knowledge: "knowledge",
  downloads: "download",
};
export const defaultNavigation = localizeData([
  {
    label: { zhCN: "Products" },
    group: true,
    children: [
      { label: { zhCN: "All products" }, href: "/products.html" },
    ],
  },
  { label: { zhCN: "Data centers" }, href: "/datacenters.html" },
  { label: { zhCN: "Knowledge" }, href: "/knowledge.html" },
  { label: { zhCN: "News" }, href: "/news.html" },
  {
    label: { zhCN: "About us" },
    group: true,
    children: [
      { label: { zhCN: "Company profile" }, href: "/about.html" },
      { label: { zhCN: "Contact us" }, href: "/contact.html" },
    ],
  },
]);

import { startCertificatePage } from "../shared/page.js";
import { installCertificateShell } from "../shared/shell.js";
import "./appearance.css";

installCertificateShell("customer");
startCertificatePage({ audience: "customer", capability: "service-entitlements.customer.v1", surfaceKey: document.querySelector<HTMLMetaElement>('meta[name="linapro-surface-key"]')?.content || "certificates" });

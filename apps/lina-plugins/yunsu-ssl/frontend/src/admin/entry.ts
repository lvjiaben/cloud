import { startCertificatePage } from "../shared/page.js";
import { installCertificateShell } from "../shared/shell.js";

installCertificateShell("admin");
startCertificatePage({ audience: "admin", capability: "service-entitlements.admin.v1", surfaceKey: "certificates" });

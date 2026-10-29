import { access, readFile } from "node:fs/promises";

const entryPath = new URL("../dist/mount.js", import.meta.url);
await access(entryPath);

const bundleSource = await readFile(entryPath, "utf8");
if (/\bprocess(?:\.|\[)/u.test(bundleSource)) {
  throw new Error(
    "The browser plugin bundle must not depend on the Node.js process global.",
  );
}

const builtModule = await import(`${entryPath.href}?verify=${Date.now()}`);
if (typeof builtModule.mount !== "function") {
  throw new Error("The plugin bundle must export mount(context).");
}

const appSource = await readFile(
  new URL("../src/App.vue", import.meta.url),
  "utf8",
);
const apiSource = await readFile(
  new URL("../src/notification/api.ts", import.meta.url),
  "utf8",
);
if (appSource.includes("watch(() => editor.channelCode")) {
  throw new Error(
    "Programmatic template editing must not trigger a channel watcher that clears provider mappings.",
  );
}
if (!appSource.includes('@change="resetProviderMapping"')) {
  throw new Error(
    "User channel changes must explicitly clear provider mappings.",
  );
}
const resendButton =
  appSource.match(
    /<button\s+[^>]*class="button primary"[^>]*@click="resend"[^>]*>/u,
  )?.[0] ?? "";
if (
  !resendButton ||
  resendButton.includes("v-if=") ||
  !resendButton.includes("!eligibility?.allowed")
) {
  throw new Error(
    "The resend button must remain visible and disabled while resend eligibility is blocked.",
  );
}
if (/wechat-work-binding|startWeComBinding|removeWeComBinding/u.test(appSource + apiSource)) {
  throw new Error("The removed WeCom self-binding page must not remain in the browser bundle.");
}

# Website rendering

This plugin owns its independent Nuxt source, lockfile, SSR renderer, and static build under `site/`. It renders title, description, image, color, and navigation. Admin fields are declared in `plugin.yaml` and displayed by `idc-portal`. The retired `apps/lina-websites` workspace has been removed.

Build and package using the plugin-root instructions in [README](../README.md). The renderer entry is `site/.output/server/index.mjs`; the plugin gateway is `../gateway/server.mjs`. Root `make dev` builds and starts the application services together.

Third-party templates may declare `entry` and `public_assets` to deliver HTML, JavaScript, and CSS without a framework restriction. See `website-html` and Admin → Website configuration → Development guide for the contract.

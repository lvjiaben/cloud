# Website rendering

The shared `SSR` implementation in `apps/lina-websites/sites/default` renders this template through `public-minimal-v1`: title, description, and independent navigation without a home image. Admin fields are declared in `plugin.yaml` and displayed by `idc-portal`.

Third-party templates can declare `entry` and `public_assets` to serve their own HTML, JS and styles after installation and activation, with no framework restriction. See `website-html` and Admin → Website configuration → Development guide for the complete contract.

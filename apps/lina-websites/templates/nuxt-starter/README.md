# Nuxt website starter

This template is only for public websites, never the customer center. After copying it to a separate repository, replace the `workspace:*` references for `@lina/site-contracts` and `@lina/site-sdk` with published pinned versions.

Set `NUXT_API_ORIGIN` to a local LinaPro backend and `NUXT_PUBLIC_SITE_URL` to the public website origin. Build `.output/server` and `.output/public`, then let the official packer calculate the real file list and digests. Never ship the zero digest from the example manifest.

A website must not own `/admin`, `/api`, `/x`, `/x-assets`, `/auth`, `/account`, `/cart`, or `/checkout`, and must not persist authoritative price, cart, customer, or payment snapshots.

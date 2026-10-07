# Website contracts

This package contains only stable public types and the website-package manifest format needed by third-party websites. It contains no merchant secrets, customer-private data, administration APIs, or provider APIs.

`site-package.schema.json` is a development-time aid. The production installer must still revalidate archive paths, digests, sizes, compatibility, routes, and capabilities on the server.

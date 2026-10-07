# Website archive format

The archive root contains `site-package.json`; all other files are Nuxt production output. The manifest must pass `@lina/site-contracts/site-package.schema.json` and list every file except itself with byte size and lowercase SHA-256.

Production packages use Ed25519. The signed bytes are the UTF-8 RFC 8785 canonical form of the manifest after removing `publisher.signature`. `publisher.keyId` must resolve to a publisher public key trusted by the local installation. Identity or key material carried by an archive never grants trust, and no software-vendor online service participates in verification. The zero signature in the example is illustrative and cannot pass production verification.

The server installer must recompute everything. It must reject traversal, absolute paths, backslash bypasses, symbolic or hard links, device files, duplicate or case-colliding names, undeclared files, excessive counts and sizes, and decompression bombs.

Packages use the `nuxt-node-1` runtime with `server/index.mjs`, `public`, and `/__site/health`. They contain no install scripts and perform no online dependency installation during install or startup.

This document freezes the manifest fields and signature input. The production packer, local trust management, and administration installer are still unimplemented, so the example must not be uploaded to production.

# Website SDK

This package wraps versioned public APIs available to website packages. It exposes the anonymous catalog contract plus the server-authoritative guest-cart open and add-item handoff. The cart ownership credential remains an HttpOnly, SameSite cookie and is never accepted as an SDK argument or returned to website code. Brand and content methods will be added only when their backend contracts exist.

Nuxt server code may provide a custom `fetch` and the local installation's API origin. The SDK never accepts administration tokens, merchant third-party secrets, or provider credentials.

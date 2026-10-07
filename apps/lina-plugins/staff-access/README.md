# LinaPro Staff Access

`staff-access` is a platform-only source plugin that owns reusable,
scoped business qualifications for target staff members. It does not replace
host authentication, role permissions, or data permissions.

Consumer plugins use `backend/cap/staffaccesscap` to check whether a visible,
enabled employee may perform a named business duty in a global or department
scope. Platform administrators remain eligible. Every grant change requires a
recent full login and creates immutable audit evidence. A grant may have an
optional activation time and expiry time; consumers evaluate both against the
database clock on every check, so an expired or not-yet-active grant fails
closed without relying on a worker or cache refresh.

The plugin frontend is built independently and mounted through the standard
plugin dynamic-page mechanism. No host frontend source is required.

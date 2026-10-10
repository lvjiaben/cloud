## ADDED Requirements

### Requirement: Required IDC database regressions
Continuous integration SHALL provide explicit isolated database connection settings to IDC test modules and SHALL fail when mandatory database regression suites silently skip.

#### Scenario: Mandatory database test has no connection
- **WHEN** CI runs a mandatory IDC database regression without a usable database configuration
- **THEN** the validation job fails rather than reporting a successful skipped test

### Requirement: Dynamic plugin asset completeness
Dynamic plugin development and packaging SHALL build plugin-owned frontends before packaging and validate declared public assets and surface entrypoints before publishing a WASM artifact.

#### Scenario: Declared surface asset is absent
- **WHEN** a plugin declares a frontend surface without its built entrypoint
- **THEN** packaging fails before writing an unusable artifact into the runtime discovery directory

#### Scenario: Clean checkout builds SSL
- **WHEN** the standard development build encounters the SSL dynamic plugin in a clean checkout
- **THEN** it builds the plugin-owned frontend and packages its valid declared surfaces

### Requirement: Reachable governed plugin surfaces
Removing a sidebar container SHALL preserve its authorized plugin sandbox routes and bridge metadata so configuration entrypoints remain accessible.

#### Scenario: Migration surface leaves the extension sidebar
- **WHEN** the administration menu hides the former extension container
- **THEN** the authorized migration sandbox route remains registered and can load its declared isolated entrypoint

### Requirement: Consistent dynamic upgrade discovery
Dynamic plugin upgrade execution SHALL compare the database-effective release against the discovered artifact using the same version semantics as preview, including while holding the lifecycle lock.

#### Scenario: Confirm a pending dynamic upgrade
- **WHEN** a newer dynamic artifact is discovered and its confirmed upgrade satisfies dependency and authorization checks
- **THEN** execution upgrades that artifact and reports its new effective version instead of reclassifying the pending release as normal before execution

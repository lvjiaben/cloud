## ADDED Requirements

### Requirement: Independent customer template plugins
The system SHALL support installed, enabled customer-template providers owned by idc-portal. A provider SHALL publish a bounded relative HTML entry and optional SPA flag through governed public assets. Templates MAY use any frontend framework. The default implementation SHALL be copied from web-customer under the user's explicit authorization and maintained exclusively in its plugin directory; required public component and customer integration dependencies SHALL be bundled with their licenses. The entire plugin SHALL build and package outside the repository using standard Node.js, pnpm and Go, without host sources or external source symlinks. The old web-customer directory MAY be deleted only after the independent build, installation and runtime have been verified under the user’s precise authorization; all other host sources SHALL NOT be modified.

#### Scenario: Build outside the source repository
- **WHEN** the complete customer-default directory is copied outside the repository and its locked packages are installed
- **THEN** standard Node.js, pnpm and Go SHALL produce a standalone installable WASM containing the client assets and language resources
- **AND** no symlink or build input SHALL resolve to another repository source directory

#### Scenario: Select an installed template
- **WHEN** an administrator saves a currently available customer template
- **THEN** the public configuration identifies its release-bound entry and the independent customer gateway serves it, including supported SPA deep links

#### Scenario: Template becomes unavailable
- **WHEN** the selected provider is disabled or uninstalled
- **THEN** the public configuration is inactive and the gateway reports a retryable unavailable state without deleting customer business data

### Requirement: Shared brand and independent template configuration
The system SHALL persist customer name, title, company, logo, contact and description as shared public configuration. Each provider SHALL declare and independently retain its public-specific configuration. Neither surface SHALL expose secret credentials.

#### Scenario: Switch customer templates
- **WHEN** an administrator selects another available template
- **THEN** shared brand remains and each template retains its own saved settings

### Requirement: Authoritative configurable customer navigation
The customer application SHALL consume the enabled member navigation snapshot. Product entries SHALL open services associated with the selected menu; system, custom and plugin entries SHALL use their validated target. An authorized open-platform entry SHALL remain available without requiring a legacy hardcoded group.

#### Scenario: All navigation is disabled
- **WHEN** the member navigation request successfully returns an empty list
- **THEN** the application SHALL respect the empty snapshot instead of restoring hardcoded entries

#### Scenario: Navigation request fails
- **WHEN** the navigation service fails
- **THEN** the application SHALL retain its last working state and SHALL NOT treat the failed request as an empty successful snapshot

### Requirement: In-page developer reference
The customer configuration page SHALL provide framework-neutral development, build, installation, upgrade and removal instructions plus a searchable live OpenAPI reference with request and response schemas and a downloadable definition.

#### Scenario: Inspect actual API contracts
- **WHEN** the developer searches an installed site's API reference
- **THEN** matching methods and paths can be expanded with their referenced schema definitions
- **AND** a failed reference load SHALL offer retry instead of displaying fabricated API contracts

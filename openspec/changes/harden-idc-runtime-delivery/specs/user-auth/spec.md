## ADDED Requirements

### Requirement: Reject publicly known signing secrets
The system SHALL reject publicly distributed placeholder JWT signing secrets before starting an authenticated HTTP service. Deployment examples SHALL require a private signing secret instead of providing a usable shared default.

#### Scenario: Deployment retains example signing secret
- **WHEN** authenticated HTTP startup reads a publicly known placeholder signing secret
- **THEN** startup fails with a configuration error before accepting authentication requests

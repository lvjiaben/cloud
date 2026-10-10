## ADDED Requirements

### Requirement: Merchant-owned customer product directories
The system SHALL provide merchant-managed bilingual product directories independently from sales categories and staff service classifications. Directories SHALL associate products without modifying their price, fulfillment, or access rights.

#### Scenario: Merchant maintains a directory
- **WHEN** an authorized merchant creates or updates a directory and its associated products
- **THEN** the customer directory list reflects the saved associations and stable ordering
- **AND** concurrent edits cannot silently overwrite another saved revision

#### Scenario: Disabled directory preserves services
- **WHEN** a merchant disables a directory
- **THEN** existing customer services and billing remain accessible through the all-services view

### Requirement: Merchant-configured customer service menus
The system SHALL render enabled merchant directories as customer-center menu entries linked to the shared purchased-service view. Customers SHALL NOT manage visibility preferences. Selecting an entry SHALL select its corresponding category tab and filter purchased services by the associated products.

#### Scenario: Customer opens a merchant service menu
- **WHEN** a customer clicks an enabled merchant menu or opens its URL directly
- **THEN** its category tab and owned service list are selected automatically
- **AND** reload and browser history preserve the selection without customer visibility settings

#### Scenario: Directory navigation is unavailable
- **WHEN** directory navigation cannot load
- **THEN** all owned services remain accessible and a retry is offered

### Requirement: Purchase sales-category navigation
The purchase catalog SHALL expose merchant sales categories in its side navigation, independently from customer service directories. Category selection SHALL filter purchasable products and survive reload through URL state.

#### Scenario: Customer chooses a sales category
- **WHEN** a customer chooses a merchant sales category
- **THEN** only published purchasable products belonging to that category are listed
- **AND** selecting a user-center service menu does not modify sales classifications

### Requirement: Scoped purchased-service listing
The system SHALL list purchased services using merchant product associations and the current authenticated customer boundary in database queries. Listings SHALL have bounded pagination with accurate totals and set-based association loading.

#### Scenario: Directory contains another customer's services
- **WHEN** a customer requests a directory's services
- **THEN** only their own service entitlements are returned
- **AND** service identifiers or menu selection cannot bypass ownership checks

#### Scenario: Products have no active directory
- **WHEN** a purchased product has no active directory association
- **THEN** its service remains reachable from the all-services view

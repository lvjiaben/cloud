// These read-only directory facts let the merchant preview draft mappings.
// The configuration writer obtains its own authorized supplier observations.
package v1

// CatalogDirectoryRelationship links connector parameters without introducing
// a cloud category or exposing credentials, costs or private response bodies.
type CatalogDirectoryRelationship struct {
	ParentFieldKey string                        `json:"parentFieldKey" dc:"Interface-declared parent parameter key" eg:"location"`
	ChildFieldKey  string                        `json:"childFieldKey" dc:"Interface-declared child parameter key" eg:"zone"`
	Links          []*CatalogDirectoryChoiceLink `json:"links" dc:"Allowed parent and child option value pairs" eg:"[]"`
}

// CatalogDirectoryChoiceLink is one eligible pair from the bounded directory.
type CatalogDirectoryChoiceLink struct {
	ParentValue string `json:"parentValue" dc:"Parent option value from the connector directory" eg:"north"`
	ChildValue  string `json:"childValue" dc:"Child option value from the connector directory" eg:"zone-a"`
}

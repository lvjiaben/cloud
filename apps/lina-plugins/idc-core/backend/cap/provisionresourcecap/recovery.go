// This file defines operational corrections for a paid server delivery. Hardware
// sizes and financial terms are deliberately absent from editable selections.
package provisionresourcecap

// RecoveryConfiguration selects a compatible upstream and native routing IDs.
// The resource owner resolves and pins its connection revision before approval.
type RecoveryConfiguration struct {
	PasswordLength                 int    `json:"generationLength,omitempty" dc:"Replacement generated password length from 6 to 20; zero retains the original generation rules" eg:"12"`
	PasswordCharacters             string `json:"generationCharacters,omitempty" dc:"Comma-separated replacement character classes: lowercase,uppercase,digits,symbols; empty retains original rules when length is zero" eg:"lowercase,uppercase,digits"`
	InterfaceLabel                 string `json:"interfaceLabel" dc:"Validated upstream display name" eg:"Primary cloud"`
	RegionLabel                    string `json:"regionLabel" dc:"Validated region display name" eg:"Shaanxi"`
	LineLabel                      string `json:"lineLabel" dc:"Validated node display name" eg:"Node A"`
	OperatingSystemLabel           string `json:"operatingSystemLabel" dc:"Validated image display name" eg:"Debian"`
	InterfaceInstanceNo            string `json:"interfaceInstanceNo" dc:"Selected compatible upstream interface" eg:"pfi_0123456789abcdef0123456789abcdef"`
	InterfaceConfigurationRevision string `json:"interfaceConfigurationRevision" dc:"Resource-owner connection revision pinned by validation" eg:"rci_0123456789abcdef0123456789abcdef"`
	Region                         string `json:"region" dc:"Selected upstream region value" eg:"1"`
	Line                           string `json:"line" dc:"Selected upstream node value" eg:"5"`
	OperatingSystem                string `json:"operatingSystem" dc:"Selected upstream image value" eg:"97"`
}

// RecoveryOption is a safe label and value; it contains no channel credentials.
type RecoveryOption struct {
	Value     string `json:"value" dc:"Opaque selection value" eg:"1"`
	LabelZhCN string `json:"labelZhCN" dc:"Chinese selection label" eg:"Shaanxi"`
	LabelEnUS string `json:"labelEnUS" dc:"English selection label" eg:"Shaanxi"`
}

// RecoverySpecification is an original, immutable purchased configuration fact.
type RecoverySpecification struct {
	Key   string `json:"key" dc:"Specification kind" eg:"cpu"`
	Value string `json:"value" dc:"Original purchased value" eg:"2"`
}

// RecoveryEditor exposes original terms and bounded, read-only upstream choices.
type RecoveryEditor struct {
	Original         RecoveryConfiguration   `json:"original" dc:"Original paid operational configuration" eg:"{}"`
	Selection        RecoveryConfiguration   `json:"selection" dc:"Currently inspected operational configuration" eg:"{}"`
	Specifications   []RecoverySpecification `json:"specifications" dc:"Immutable purchased specifications" eg:"[]"`
	Interfaces       []RecoveryOption        `json:"interfaces" dc:"At most 100 compatible enabled interfaces" eg:"[]"`
	Regions          []RecoveryOption        `json:"regions" dc:"Bounded upstream regions" eg:"[]"`
	Lines            []RecoveryOption        `json:"lines" dc:"Bounded upstream nodes for the selected region" eg:"[]"`
	OperatingSystems []RecoveryOption        `json:"operatingSystems" dc:"Bounded global upstream images; node readiness still belongs to the provider" eg:"[]"`
}

// RecoverySelectionRequest is an authorized inspection of one paid request.
type RecoverySelectionRequest struct {
	Request   TaskRequest
	Selection *RecoveryConfiguration
}

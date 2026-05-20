// Package secret defines provider-agnostic item/field types and the
// orchestration that turns a list of matched items into env-var pairs.
package secret

// Item is a minimal listing-time view of a vault entry.
type Item struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

// Field is one named secret value attached to an item.
type Field struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
	Value   any    `json:"value"`
}

// ItemDetails is the full item view including all fields.
type ItemDetails struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Fields []Field `json:"fields"`
}

// EnvPair is one resolved name/value pair ready to export.
type EnvPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Item  string `json:"item"`
	Field string `json:"field,omitempty"`
}

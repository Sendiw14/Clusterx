package sdk

import "encoding/json"

// HandlerSpec describes an allowlisted handler version for the coordinator's compiled catalogue
// (GET /api/v1/handlers, admission validation, Register capability checks).
type HandlerSpec struct {
	Name                  string
	Version               int
	Description           string
	PayloadSchema         json.RawMessage // JSON Schema 2020-12 (the coordinator enforces a documented subset)
	ResultSchema          json.RawMessage // optional
	DefaultTimeoutSeconds int
}

// Key returns the spec's handler key.
func (s HandlerSpec) Key() HandlerKey { return HandlerKey{Name: s.Name, Version: s.Version} }

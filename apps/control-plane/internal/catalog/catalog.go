// Package catalog is the coordinator's compiled handler allowlist (MASTER §4: workers execute only
// precompiled allowlisted handlers). Payloads are validated against each handler's JSON Schema
// using a documented subset of JSON Schema 2020-12: type, properties, required,
// additionalProperties:false, minimum, maximum, minLength, maxLength, enum, items. A schema that
// uses any other keyword is rejected when the catalogue is built, so validation never silently skips.
package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	basicjobs "github.com/Sendiw14/Clusterx/examples/basic-jobs"
	sdk "github.com/Sendiw14/Clusterx/packages/sdk-go"
)

// Entry is one handler version with its parsed schema.
type Entry struct {
	Spec   sdk.HandlerSpec
	schema *schema
}

// Catalog is immutable after New.
type Catalog struct {
	entries map[sdk.HandlerKey]Entry
	keys    []sdk.HandlerKey
}

// Default is the reference deployment's catalogue (examples/basic-jobs).
func Default() (*Catalog, error) { return New(basicjobs.Specs()) }

// New validates specs and their schemas.
func New(specs []sdk.HandlerSpec) (*Catalog, error) {
	c := &Catalog{entries: map[sdk.HandlerKey]Entry{}}
	for _, s := range specs {
		if _, dup := c.entries[s.Key()]; dup {
			return nil, fmt.Errorf("catalog: duplicate %s", s.Key())
		}
		sc, err := parseSchema(s.PayloadSchema)
		if err != nil {
			return nil, fmt.Errorf("catalog: %s payload schema: %w", s.Key(), err)
		}
		if s.DefaultTimeoutSeconds < 1 || s.DefaultTimeoutSeconds > 86400 {
			return nil, fmt.Errorf("catalog: %s default timeout out of range", s.Key())
		}
		c.entries[s.Key()] = Entry{Spec: s, schema: sc}
		c.keys = append(c.keys, s.Key())
	}
	sort.Slice(c.keys, func(i, j int) bool {
		if c.keys[i].Name != c.keys[j].Name {
			return c.keys[i].Name < c.keys[j].Name
		}
		return c.keys[i].Version < c.keys[j].Version
	})
	return c, nil
}

// Known implements store.Catalog.
func (c *Catalog) Known(name string, version int) bool {
	_, ok := c.entries[sdk.HandlerKey{Name: name, Version: version}]
	return ok
}

// Lookup returns the entry for a handler version.
func (c *Catalog) Lookup(name string, version int) (Entry, bool) {
	e, ok := c.entries[sdk.HandlerKey{Name: name, Version: version}]
	return e, ok
}

// Specs returns every spec sorted by name then version.
func (c *Catalog) Specs() []sdk.HandlerSpec {
	out := make([]sdk.HandlerSpec, 0, len(c.keys))
	for _, k := range c.keys {
		out = append(out, c.entries[k].Spec)
	}
	return out
}

// FieldError is one validation failure: RFC 6901 pointer, OpenAPI ValidationDetails rule, message.
type FieldError struct {
	Pointer, Rule, Message string
}

// Validate checks a decoded payload (decoded with UseNumber) against the entry's schema.
// prefix is prepended to pointers (e.g. "/payload"). At most 50 errors are returned.
func (e Entry) Validate(v any, prefix string) []FieldError {
	var errs []FieldError
	e.schema.validate(v, prefix, &errs)
	if len(errs) > 50 {
		errs = errs[:50]
	}
	return errs
}

type schema struct {
	Type                 string
	Properties           map[string]*schema
	Required             []string
	AdditionalProperties *bool
	Minimum, Maximum     *float64
	MinLength, MaxLength *int
	Enum                 []any
	Items                *schema
}

var allowedKeywords = []string{"type", "properties", "required", "additionalProperties", "minimum", "maximum",
	"minLength", "maxLength", "enum", "items", "description", "title", "$schema"}

func parseSchema(raw json.RawMessage) (*schema, error) {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	s := &schema{}
	for k, v := range generic {
		if !slices.Contains(allowedKeywords, k) {
			return nil, fmt.Errorf("unsupported keyword %q", k)
		}
		var err error
		switch k {
		case "type":
			err = json.Unmarshal(v, &s.Type)
		case "properties":
			var props map[string]json.RawMessage
			if err = json.Unmarshal(v, &props); err == nil {
				s.Properties = map[string]*schema{}
				for name, p := range props {
					if s.Properties[name], err = parseSchema(p); err != nil {
						return nil, fmt.Errorf("%s: %w", name, err)
					}
				}
			}
		case "items":
			s.Items, err = parseSchema(v)
		case "required":
			err = json.Unmarshal(v, &s.Required)
		case "additionalProperties":
			var b bool
			if err = json.Unmarshal(v, &b); err == nil {
				s.AdditionalProperties = &b
			}
		case "minimum":
			s.Minimum = new(float64)
			err = json.Unmarshal(v, s.Minimum)
		case "maximum":
			s.Maximum = new(float64)
			err = json.Unmarshal(v, s.Maximum)
		case "minLength":
			s.MinLength = new(int)
			err = json.Unmarshal(v, s.MinLength)
		case "maxLength":
			s.MaxLength = new(int)
			err = json.Unmarshal(v, s.MaxLength)
		case "enum":
			d := json.NewDecoder(bytes.NewReader(v))
			d.UseNumber()
			err = d.Decode(&s.Enum)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
	}
	switch s.Type {
	case "", "object", "array", "string", "integer", "number", "boolean", "null":
	default:
		return nil, fmt.Errorf("unsupported type %q", s.Type)
	}
	return s, nil
}

func escape(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
}

func typeOf(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		if _, err := strconv.ParseInt(string(x), 10, 64); err == nil {
			return "integer"
		}
		if f, err := x.Float64(); err == nil && f == math.Trunc(f) && !strings.ContainsAny(string(x), ".eE") {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

func (s *schema) validate(v any, ptr string, errs *[]FieldError) {
	if len(*errs) > 50 {
		return
	}
	add := func(rule, msg string) { *errs = append(*errs, FieldError{Pointer: ptr, Rule: rule, Message: msg}) }
	t := typeOf(v)
	if s.Type != "" && t != s.Type && !(s.Type == "number" && t == "integer") {
		add("type", "must be of type "+s.Type)
		return
	}
	if len(s.Enum) > 0 {
		ok := false
		for _, e := range s.Enum {
			ok = ok || fmt.Sprint(e) == fmt.Sprint(v)
		}
		if !ok {
			add("enum", "must be one of the allowed values")
		}
	}
	switch x := v.(type) {
	case json.Number:
		f, _ := x.Float64()
		if (s.Minimum != nil && f < *s.Minimum) || (s.Maximum != nil && f > *s.Maximum) {
			add("range", fmt.Sprintf("must be between %v and %v", fmtBound(s.Minimum), fmtBound(s.Maximum)))
		}
	case string:
		n := utf8.RuneCountInString(x)
		if (s.MinLength != nil && n < *s.MinLength) || (s.MaxLength != nil && n > *s.MaxLength) {
			add("length", "length out of range")
		}
	case []any:
		if s.Items != nil {
			for i, item := range x {
				s.Items.validate(item, ptr+"/"+strconv.Itoa(i), errs)
			}
		}
	case map[string]any:
		for _, r := range s.Required {
			if _, ok := x[r]; !ok {
				*errs = append(*errs, FieldError{Pointer: ptr + "/" + escape(r), Rule: "required", Message: "is required"})
			}
		}
		names := make([]string, 0, len(x))
		for k := range x {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			if ps, ok := s.Properties[k]; ok {
				ps.validate(x[k], ptr+"/"+escape(k), errs)
			} else if s.AdditionalProperties != nil && !*s.AdditionalProperties {
				*errs = append(*errs, FieldError{Pointer: ptr + "/" + escape(k), Rule: "unknown_field", Message: "is not allowed"})
			}
		}
	}
}

func fmtBound(f *float64) string {
	if f == nil {
		return "-"
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

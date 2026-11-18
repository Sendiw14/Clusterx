package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// canonicalJSON serializes a value decoded with UseNumber following RFC 8785 (JCS): object members
// sorted by UTF-16 code units, no insignificant whitespace, ECMAScript number formatting, minimal
// string escaping. Numbers outside IEEE-754 double precision lose precision exactly as JCS specifies.
func canonicalJSON(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		buf.WriteString(strconv.FormatBool(x))
	case json.Number:
		f, err := x.Float64()
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return fmt.Errorf("number %q is not representable", x)
		}
		buf.WriteString(esNumber(f))
	case float64:
		buf.WriteString(esNumber(x))
	case int:
		buf.WriteString(strconv.Itoa(x))
	case string:
		writeJCSString(buf, x)
	case []any:
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := canonicalJSON(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeJCSString(buf, k)
			buf.WriteByte(':')
			if err := canonicalJSON(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("unsupported type %T", v)
	}
	return nil
}

func utf16Less(a, b string) bool {
	ar, br := []rune(a), []rune(b)
	enc := func(rs []rune) []uint16 {
		var out []uint16
		for _, r := range rs {
			if r >= 0x10000 {
				r -= 0x10000
				out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
			} else {
				out = append(out, uint16(r))
			}
		}
		return out
	}
	ua, ub := enc(ar), enc(br)
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

func writeJCSString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}

// esNumber formats like ECMAScript Number.prototype.toString.
func esNumber(f float64) string {
	if f == 0 {
		return "0"
	}
	abs := math.Abs(f)
	if abs >= 1e21 || abs < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64) // e.g. 1e+21, 1.5e-07
		mant, exp, _ := strings.Cut(s, "e")
		sign := exp[0]
		exp = strings.TrimLeft(exp[1:], "0")
		return mant + "e" + string(sign) + exp
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// fingerprint is SHA-256 of the canonical submission (ADR-0004).
func fingerprint(fields map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	if err := canonicalJSON(&buf, fields); err != nil {
		return nil, err
	}
	if !utf8.Valid(buf.Bytes()) {
		return nil, fmt.Errorf("invalid UTF-8")
	}
	h := sha256.Sum256(buf.Bytes())
	return h[:], nil
}

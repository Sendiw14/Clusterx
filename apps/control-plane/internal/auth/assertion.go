// Package auth verifies BFF service assertions (ADR-0005 #2, ADR-0006 §1/§5): compact JWS with
// alg EdDSA (Ed25519, crypto/ed25519), pinned public keys by kid, exact iss/aud, ±5 s skew, exp at most
// 60 s after iat, and single-use jti for mutations and logins (in-memory replay cache; single
// coordinator, ADR-0001). It also mints assertions for tests and the development CLI only.
//
// Verification is DB-independent. The caller then re-checks the user and session in PostgreSQL.
package auth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Fixed claim values (ADR-0006 §1).
const (
	Issuer     = "clusterx-bff"
	Audience   = "clusterx-control-plane"
	HeaderType = "clusterx-assertion+jwt"
	MaxTTL     = 60 * time.Second
	Skew       = 5 * time.Second
	// ReplayWindow is how long a used jti is remembered: longer than MaxTTL + 2*Skew.
	ReplayWindow = 120 * time.Second
	// maxTokenLen bounds the parsed input (an assertion is a few hundred bytes).
	maxTokenLen = 4096
)

// Reasons returned in Error (OpenAPI details.reason).
const (
	ReasonInvalid = "assertion_invalid"
	ReasonExpired = "assertion_expired"
)

// Error is a verification failure. Detail is for server logs only, never returned to the client.
type Error struct {
	Reason string
	Detail string
}

func (e *Error) Error() string { return e.Reason + ": " + e.Detail }

func invalid(format string, a ...any) error {
	return &Error{Reason: ReasonInvalid, Detail: fmt.Sprintf(format, a...)}
}

// Claims are the verified assertion claims.
type Claims struct {
	Kid     string `json:"-"` // from the protected header, never from the payload
	Iss     string `json:"iss"`
	Aud     string `json:"aud"`
	Sub     string `json:"sub"`
	Sid     string `json:"sid,omitempty"`
	Role    string `json:"role,omitempty"`
	Purpose string `json:"purpose,omitempty"`
	Iat     int64  `json:"iat"`
	Nbf     int64  `json:"nbf,omitempty"`
	Exp     int64  `json:"exp"`
	Jti     string `json:"jti"`
	Rid     string `json:"rid,omitempty"`
}

// Login reports whether this is a login assertion (purpose=login, sub=github:<id>, no sid).
func (c Claims) Login() bool { return c.Purpose == "login" }

// UserID returns the uuid of sub=user:<uuid>.
func (c Claims) UserID() string { return strings.TrimPrefix(c.Sub, "user:") }

// GitHubID returns the numeric id of sub=github:<id>.
func (c Claims) GitHubID() int64 {
	n, _ := strconv.ParseInt(strings.TrimPrefix(c.Sub, "github:"), 10, 64)
	return n
}

var (
	uuidRe   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	githubRe = regexp.MustCompile(`^github:[1-9][0-9]{0,18}$`)
	jtiRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
	kidRe    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	ridRe    = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)
)

// LooksLikeAssertion reports whether a bearer value has the compact JWS shape (three dot-separated parts).
func LooksLikeAssertion(s string) bool { return strings.Count(s, ".") == 2 }

// Verifier checks assertions against pinned keys. The zero value has no keys and rejects everything.
type Verifier struct {
	Keys   map[string]ed25519.PublicKey
	Now    func() time.Time // injectable clock for tests; nil = time.Now
	Replay *ReplayCache
}

// NewVerifier returns a verifier with the given keys and a fresh replay cache.
func NewVerifier(keys map[string]ed25519.PublicKey) *Verifier {
	return &Verifier{Keys: keys, Replay: NewReplayCache(100_000)}
}

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// Verify checks signature, header and time claims, and the claim shape for login or normal
// assertions. singleUse consumes the jti (mutations and logins); reads and WebSocket upgrades do not.
func (v *Verifier) Verify(token string, singleUse bool) (Claims, error) {
	if v == nil || len(v.Keys) == 0 {
		return Claims{}, invalid("no BFF assertion keys configured")
	}
	if len(token) > maxTokenLen {
		return Claims{}, invalid("assertion too long")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, invalid("not a compact JWS")
	}
	hb, err1 := base64.RawURLEncoding.DecodeString(parts[0])
	pb, err2 := base64.RawURLEncoding.DecodeString(parts[1])
	sig, err3 := base64.RawURLEncoding.DecodeString(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return Claims{}, invalid("bad base64url")
	}
	var h struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	if err := strictKeys(hb, headerKeys); err != nil {
		return Claims{}, invalid("bad header: %v", err)
	}
	if err := json.Unmarshal(hb, &h); err != nil {
		return Claims{}, invalid("bad header")
	}
	if h.Alg != "EdDSA" || h.Typ != HeaderType {
		return Claims{}, invalid("unsupported header alg=%q typ=%q", h.Alg, h.Typ)
	}
	key, ok := v.Keys[h.Kid]
	if !ok {
		return Claims{}, invalid("unknown kid %q", h.Kid)
	}
	// Signature before any claim is trusted (RFC 7515 signing input = header.payload as sent).
	if len(sig) != ed25519.SignatureSize || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return Claims{}, invalid("bad signature")
	}
	if err := strictKeys(pb, claimKeys); err != nil {
		return Claims{}, invalid("bad claims: %v", err)
	}
	var c Claims
	dec := json.NewDecoder(strings.NewReader(string(pb)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Claims{}, invalid("bad claims: %v", err)
	}
	c.Kid = h.Kid
	if c.Iss != Issuer || c.Aud != Audience {
		return Claims{}, invalid("iss/aud mismatch")
	}
	if !jtiRe.MatchString(c.Jti) {
		return Claims{}, invalid("bad jti")
	}
	if c.Rid != "" && !ridRe.MatchString(c.Rid) {
		return Claims{}, invalid("bad rid")
	}
	if c.Iat <= 0 || c.Exp <= c.Iat || time.Duration(c.Exp-c.Iat)*time.Second > MaxTTL {
		return Claims{}, invalid("exp must be within %s of iat", MaxTTL)
	}
	now := v.now()
	skew := int64(Skew / time.Second)
	if c.Iat > now.Unix()+skew || (c.Nbf != 0 && c.Nbf > now.Unix()+skew) {
		return Claims{}, invalid("issued in the future")
	}
	if c.Exp+skew <= now.Unix() {
		return Claims{}, &Error{Reason: ReasonExpired, Detail: "exp passed"}
	}
	if c.Login() {
		if !githubRe.MatchString(c.Sub) || c.Sid != "" {
			return Claims{}, invalid("login assertion needs sub=github:<id> and no sid")
		}
	} else {
		if c.Purpose != "" || !strings.HasPrefix(c.Sub, "user:") || !uuidRe.MatchString(c.UserID()) || !uuidRe.MatchString(c.Sid) {
			return Claims{}, invalid("assertion needs sub=user:<uuid> and sid")
		}
		if c.Role != "operator" && c.Role != "viewer" {
			return Claims{}, invalid("bad role hint")
		}
	}
	if singleUse || c.Login() {
		if !v.Replay.Use(c.Jti, now) {
			return Claims{}, invalid("replayed or unrecordable jti")
		}
	}
	return c, nil
}

var (
	headerKeys = map[string]bool{"alg": true, "typ": true, "kid": true}
	claimKeys  = map[string]bool{"iss": true, "aud": true, "sub": true, "sid": true, "role": true, "purpose": true,
		"iat": true, "nbf": true, "exp": true, "jti": true, "rid": true}
)

// strictKeys enforces a closed JSON object (RV5-03): every member name must be in allowed, spelled
// exactly (encoding/json alone matches names case-insensitively), and appear once (encoding/json
// alone keeps the last duplicate).
func strictKeys(b []byte, allowed map[string]bool) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return errors.New("not a JSON object")
	}
	seen := map[string]bool{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		k, _ := t.(string)
		if !allowed[k] {
			return fmt.Errorf("member %q not allowed", k)
		}
		if seen[k] {
			return fmt.Errorf("duplicate member %q", k)
		}
		seen[k] = true
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}

// ReplayCache remembers used jti values for ReplayWindow. It is bounded: when full (after purging
// expired entries) it refuses new entries, so verification fails closed.
type ReplayCache struct {
	mu   sync.Mutex
	seen map[string]time.Time
	max  int
}

// NewReplayCache returns a cache holding at most max live entries.
func NewReplayCache(max int) *ReplayCache {
	return &ReplayCache{seen: make(map[string]time.Time), max: max}
}

// Use records jti and reports whether it was unused.
func (r *ReplayCache) Use(jti string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if until, ok := r.seen[jti]; ok && now.Before(until) {
		return false
	}
	if len(r.seen) >= r.max {
		for k, until := range r.seen {
			if !now.Before(until) {
				delete(r.seen, k)
			}
		}
		if len(r.seen) >= r.max {
			return false
		}
	}
	r.seen[jti] = now.Add(ReplayWindow)
	return true
}

// Len returns the number of remembered entries (tests).
func (r *ReplayCache) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.seen)
}

// ---- key material ------------------------------------------------------------------------------

type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	X   string `json:"x,omitempty"`
	D   string `json:"d,omitempty"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

// ParseJWKS reads Ed25519 public keys (RFC 8037 OKP JWK) from a JWKS document. Private members are
// refused so that a signing key is never deployed to the coordinator by mistake.
func ParseJWKS(b []byte) (map[string]ed25519.PublicKey, error) {
	var set jwks
	dec := json.NewDecoder(strings.NewReader(string(b)))
	if err := dec.Decode(&set); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	if len(set.Keys) == 0 || len(set.Keys) > 16 {
		return nil, errors.New("jwks: 1..16 keys required")
	}
	out := make(map[string]ed25519.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" || !kidRe.MatchString(k.Kid) {
			return nil, fmt.Errorf("jwks: key %q must be kty=OKP crv=Ed25519 with a kid", k.Kid)
		}
		if k.D != "" {
			return nil, fmt.Errorf("jwks: key %q contains a private key; give the coordinator public keys only", k.Kid)
		}
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("jwks: key %q has an invalid x", k.Kid)
		}
		if _, dup := out[k.Kid]; dup {
			return nil, fmt.Errorf("jwks: duplicate kid %q", k.Kid)
		}
		out[k.Kid] = ed25519.PublicKey(x)
	}
	return out, nil
}

// LoadJWKSFile reads ParseJWKS from path.
func LoadJWKSFile(path string) (map[string]ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseJWKS(b)
}

// PrivateJWK and PublicJWKS encode a development key pair (dev CLI only).
func PrivateJWK(kid string, priv ed25519.PrivateKey) ([]byte, error) {
	return json.MarshalIndent(jwk{Kty: "OKP", Crv: "Ed25519", Kid: kid,
		X: base64.RawURLEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)),
		D: base64.RawURLEncoding.EncodeToString(priv.Seed())}, "", "  ")
}

// PublicJWKS returns a JWKS containing the public half of priv.
func PublicJWKS(kid string, priv ed25519.PrivateKey) ([]byte, error) {
	return json.MarshalIndent(jwks{Keys: []jwk{{Kty: "OKP", Crv: "Ed25519", Kid: kid,
		X: base64.RawURLEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))}}}, "", "  ")
}

// ParsePrivateJWK reads a private OKP JWK written by PrivateJWK.
func ParsePrivateJWK(b []byte) (string, ed25519.PrivateKey, error) {
	var k jwk
	if err := json.Unmarshal(b, &k); err != nil {
		return "", nil, err
	}
	seed, err := base64.RawURLEncoding.DecodeString(k.D)
	if k.Kty != "OKP" || k.Crv != "Ed25519" || !kidRe.MatchString(k.Kid) || err != nil || len(seed) != ed25519.SeedSize {
		return "", nil, errors.New("not an Ed25519 private JWK")
	}
	return k.Kid, ed25519.NewKeyFromSeed(seed), nil
}

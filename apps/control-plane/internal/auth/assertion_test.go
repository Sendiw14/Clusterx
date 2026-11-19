package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

const (
	uid = "0193a0f2-0000-7000-8000-000000000001"
	sid = "0193a0f2-9000-7000-8000-0000000000e1"
)

func keypair(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func TestVerify(t *testing.T) {
	priv, other := keypair(t), keypair(t)
	now := time.Unix(1_800_000_000, 0)
	v := NewVerifier(map[string]ed25519.PublicKey{"k1": priv.Public().(ed25519.PublicKey)})
	v.Now = func() time.Time { return now }
	normal := Claims{Sub: "user:" + uid, Sid: sid, Role: "operator"}
	mint := func(k ed25519.PrivateKey, kid string, c Claims) string {
		s, err := Mint(kid, k, c, now)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	ok := mint(priv, "k1", normal)
	if c, err := v.Verify(ok, false); err != nil || c.UserID() != uid || c.Sid != sid || c.Kid != "k1" {
		t.Fatalf("valid: %+v %v", c, err)
	}
	// Reads may reuse the assertion until exp; a mutation consumes the jti once.
	if _, err := v.Verify(ok, false); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ok, true); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ok, true); reason(err) != ReasonInvalid {
		t.Fatalf("replayed mutation: %v", err)
	}

	tamper := func(s string) string { // flip the role in the payload, keep the signature
		p := strings.Split(s, ".")
		b, _ := base64.RawURLEncoding.DecodeString(p[1])
		b = []byte(strings.Replace(string(b), `"viewer"`, `"operator"`, 1))
		return p[0] + "." + base64.RawURLEncoding.EncodeToString(b) + "." + p[2]
	}
	cases := map[string]struct {
		tok, want string
	}{
		"wrong key":           {mint(other, "k1", normal), ReasonInvalid},
		"unknown kid":         {mint(priv, "k2", normal), ReasonInvalid},
		"tampered payload":    {tamper(mint(priv, "k1", Claims{Sub: "user:" + uid, Sid: sid, Role: "viewer"})), ReasonInvalid},
		"wrong audience":      {mint(priv, "k1", Claims{Aud: "other", Sub: "user:" + uid, Sid: sid, Role: "viewer"}), ReasonInvalid},
		"wrong issuer":        {mint(priv, "k1", Claims{Iss: "evil", Sub: "user:" + uid, Sid: sid, Role: "viewer"}), ReasonInvalid},
		"expired":             {mint(priv, "k1", Claims{Sub: "user:" + uid, Sid: sid, Role: "viewer", Iat: now.Unix() - 70, Exp: now.Unix() - 10}), ReasonExpired},
		"within skew":         {mint(priv, "k1", Claims{Sub: "user:" + uid, Sid: sid, Role: "viewer", Iat: now.Unix() - 62, Exp: now.Unix() - 2}), ""},
		"ttl too long":        {mint(priv, "k1", Claims{Sub: "user:" + uid, Sid: sid, Role: "viewer", Exp: now.Unix() + 61}), ReasonInvalid},
		"future iat":          {mint(priv, "k1", Claims{Sub: "user:" + uid, Sid: sid, Role: "viewer", Iat: now.Unix() + 30, Exp: now.Unix() + 60}), ReasonInvalid},
		"no sid":              {mint(priv, "k1", Claims{Sub: "user:" + uid, Role: "viewer"}), ReasonInvalid},
		"login with sid":      {mint(priv, "k1", Claims{Sub: "github:42", Sid: sid, Purpose: "login"}), ReasonInvalid},
		"login bad sub":       {mint(priv, "k1", Claims{Sub: "user:" + uid, Purpose: "login"}), ReasonInvalid},
		"login":               {mint(priv, "k1", Claims{Sub: "github:42", Purpose: "login"}), ""},
		"bad role":            {mint(priv, "k1", Claims{Sub: "user:" + uid, Sid: sid, Role: "admin"}), ReasonInvalid},
		"alg none":            {"eyJhbGciOiJub25lIiwidHlwIjoiY2x1c3RlcngtYXNzZXJ0aW9uK2p3dCIsImtpZCI6ImsxIn0." + strings.Split(ok, ".")[1] + ".", ReasonInvalid},
		"garbage":             {"a.b.c", ReasonInvalid},
		"oversized":           {strings.Repeat("a", 5000) + ".b.c", ReasonInvalid},
		"signature too short": {strings.Split(ok, ".")[0] + "." + strings.Split(ok, ".")[1] + ".AAAA", ReasonInvalid},
	}
	for name, c := range cases {
		_, err := v.Verify(c.tok, false)
		if got := reason(err); got != c.want {
			t.Errorf("%s: reason %q want %q (%v)", name, got, c.want, err)
		}
	}
	// A login assertion is always single-use.
	login := mint(priv, "k1", Claims{Sub: "github:7", Purpose: "login"})
	if _, err := v.Verify(login, false); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(login, false); reason(err) != ReasonInvalid {
		t.Fatal("login assertion replayed")
	}
	// No keys configured: everything is rejected.
	if _, err := (&Verifier{}).Verify(ok, false); reason(err) != ReasonInvalid {
		t.Fatal("keyless verifier accepted")
	}
}

func reason(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return e.Reason
	}
	return "other"
}

func TestReplayCacheBoundedAndExpiring(t *testing.T) {
	r := NewReplayCache(2)
	t0 := time.Unix(0, 0)
	if !r.Use("a", t0) || !r.Use("b", t0) {
		t.Fatal("fresh entries refused")
	}
	if r.Use("c", t0) {
		t.Fatal("full cache accepted a new jti (must fail closed)")
	}
	if r.Use("a", t0.Add(time.Second)) {
		t.Fatal("replay accepted")
	}
	later := t0.Add(ReplayWindow + time.Second)
	if !r.Use("c", later) || r.Len() != 1 {
		t.Fatalf("expired entries not purged: len %d", r.Len())
	}
}

func TestJWKS(t *testing.T) {
	priv := keypair(t)
	pub, _ := PublicJWKS("k1", priv)
	keys, err := ParseJWKS(pub)
	if err != nil || !keys["k1"].Equal(priv.Public()) {
		t.Fatalf("public jwks: %v", err)
	}
	privJWK, _ := PrivateJWK("k1", priv)
	if _, err := ParseJWKS([]byte(`{"keys":[` + string(privJWK) + `]}`)); err == nil {
		t.Fatal("private key accepted in coordinator JWKS")
	}
	kid, p2, err := ParsePrivateJWK(privJWK)
	if err != nil || kid != "k1" || !p2.Equal(priv) {
		t.Fatalf("private jwk round trip: %v", err)
	}
}

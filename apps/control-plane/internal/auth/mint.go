package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"
)

// Mint signs claims as a compact JWS (EdDSA). It exists for tests and the development CLI
// (`clusterx-control-plane dev-assertion`, refused in production); the production signer is the BFF.
// Zero iat/exp/jti/iss/aud are filled in (iat=now, exp=now+60s, random 128-bit jti).
func Mint(kid string, priv ed25519.PrivateKey, c Claims, now time.Time) (string, error) {
	if c.Iss == "" {
		c.Iss = Issuer
	}
	if c.Aud == "" {
		c.Aud = Audience
	}
	if c.Iat == 0 {
		c.Iat = now.Unix()
	}
	if c.Exp == 0 {
		c.Exp = c.Iat + int64(MaxTTL/time.Second)
	}
	if c.Jti == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		c.Jti = base64.RawURLEncoding.EncodeToString(b)
	}
	h, err := json.Marshal(map[string]string{"alg": "EdDSA", "typ": HeaderType, "kid": kid})
	if err != nil {
		return "", err
	}
	c.Kid = ""
	p, err := json.Marshal(struct {
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
	}{c.Iss, c.Aud, c.Sub, c.Sid, c.Role, c.Purpose, c.Iat, c.Nbf, c.Exp, c.Jti, c.Rid})
	if err != nil {
		return "", err
	}
	in := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(p)
	return in + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(in))), nil
}

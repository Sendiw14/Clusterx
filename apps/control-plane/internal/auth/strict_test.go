package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

// RV5-03 / RV5-06 (N01, N03): the JOSE header and the claim set are closed objects with exact,
// lower-case, unique member names; typ and nbf are enforced.
func TestVerifyStrictJOSE(t *testing.T) {
	priv := keypair(t)
	now := time.Unix(1_800_000_000, 0)
	v := NewVerifier(map[string]ed25519.PublicKey{"k1": priv.Public().(ed25519.PublicKey)})
	v.Now = func() time.Time { return now }
	enc := base64.RawURLEncoding.EncodeToString
	n := 0
	sign := func(header, payload string) string {
		h, p := enc([]byte(header)), enc([]byte(payload))
		return h + "." + p + "." + enc(ed25519.Sign(priv, []byte(h+"."+p)))
	}
	hdr := `{"alg":"EdDSA","typ":"` + HeaderType + `","kid":"k1"}`
	claims := func(extra string) string {
		n++
		return fmt.Sprintf(`{"iss":"%s","aud":"%s","sub":"user:%s","sid":"%s","role":"viewer","iat":%d,"exp":%d,"jti":"jti-strict-test-%04d"%s}`,
			Issuer, Audience, uid, sid, now.Unix(), now.Unix()+30, n, extra)
	}
	if _, err := v.Verify(sign(hdr, claims("")), false); err != nil {
		t.Fatalf("canonical assertion rejected: %v", err)
	}
	bad := map[string]string{
		"extra kid claim":       sign(hdr, claims(`,"kid":"k1"`)),
		"unknown claim":         sign(hdr, claims(`,"scope":"x"`)),
		"upper-case SUB":        sign(hdr, `{"SUB":"x",`+claims("")[1:]),
		"duplicate sub":         sign(hdr, `{"sub":"user:ffffffff-0000-7000-8000-000000000001",`+claims("")[1:]),
		"future nbf":            sign(hdr, claims(fmt.Sprintf(`,"nbf":%d`, now.Unix()+30))),
		"wrong typ":             sign(`{"alg":"EdDSA","typ":"JWT","kid":"k1"}`, claims("")),
		"missing typ":           sign(`{"alg":"EdDSA","kid":"k1"}`, claims("")),
		"header jku":            sign(`{"alg":"EdDSA","typ":"`+HeaderType+`","kid":"k1","jku":"https://x"}`, claims("")),
		"header crit":           sign(`{"alg":"EdDSA","typ":"`+HeaderType+`","kid":"k1","crit":["b64"]}`, claims("")),
		"header duplicate kid":  sign(`{"alg":"EdDSA","typ":"`+HeaderType+`","kid":"k2","kid":"k1"}`, claims("")),
		"header upper-case ALG": sign(`{"ALG":"EdDSA","typ":"`+HeaderType+`","kid":"k1"}`, claims("")),
		"payload not object":    sign(hdr, `[]`),
	}
	for name, tok := range bad {
		if _, err := v.Verify(tok, false); reason(err) != ReasonInvalid {
			t.Errorf("%s: got %v, want %s", name, err, ReasonInvalid)
		}
	}
}

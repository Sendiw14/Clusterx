package catalog

import (
	"bytes"
	"encoding/json"
	"testing"

	sdk "github.com/Sendiw14/Clusterx/packages/sdk-go"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader([]byte(s)))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestDefaultCatalogValidation(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if !c.Known("sleep_ms", 1) || c.Known("sleep_ms", 2) || len(c.Specs()) != 3 {
		t.Fatal("catalogue contents")
	}
	e, _ := c.Lookup("sleep_ms", 1)
	for payload, want := range map[string]string{
		`{"ms":10}`:               "",
		`{"ms":60001}`:            "/payload/ms:range",
		`{"ms":1.5}`:              "/payload/ms:type",
		`{}`:                      "/payload/ms:required",
		`{"ms":1,"cmd":"rm -rf"}`: "/payload/cmd:unknown_field",
		`[1]`:                     "/payload:type",
	} {
		errs := e.Validate(decode(t, payload), "/payload")
		got := ""
		if len(errs) > 0 {
			got = errs[0].Pointer + ":" + errs[0].Rule
		}
		if got != want {
			t.Errorf("%s: got %q want %q", payload, got, want)
		}
	}
	h, _ := c.Lookup("hash_sha256", 1)
	big := `{"data":"` + string(bytes.Repeat([]byte("a"), 16385)) + `"}`
	if errs := h.Validate(decode(t, big), "/payload"); len(errs) != 1 || errs[0].Rule != "length" {
		t.Fatalf("length: %+v", errs)
	}
	if _, err := New([]sdk.HandlerSpec{{Name: "x", Version: 1, DefaultTimeoutSeconds: 1, PayloadSchema: json.RawMessage(`{"pattern":"a"}`)}}); err == nil {
		t.Fatal("unsupported keyword accepted")
	}
}

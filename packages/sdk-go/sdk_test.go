package sdk_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	sdk "github.com/Sendiw14/Clusterx/packages/sdk-go"
)

type resizeIn struct {
	Width int `json:"width"`
}
type resizeOut struct {
	Area int `json:"area"`
}

func job(name string, v int) sdk.Job {
	return sdk.Job{ID: "j", AttemptID: "a", Attempt: 1, MaxAttempts: 3, Handler: sdk.HandlerKey{Name: name, Version: v}}
}

func TestRegisterValidation(t *testing.T) {
	h := sdk.HandlerFunc[resizeIn, resizeOut](func(context.Context, sdk.Job, resizeIn) (resizeOut, error) { return resizeOut{}, nil })
	cases := []struct {
		name    string
		version int
		ok      bool
	}{
		{"image.resize", 1, true},
		{"a", 1000, true},
		{"Image", 1, false},
		{"", 1, false},
		{"1abc", 1, false},
		{strings.Repeat("a", 64), 1, false},
		{"ok", 0, false},
		{"ok", 1001, false},
	}
	for _, c := range cases {
		var r sdk.Registry
		err := sdk.Register(&r, c.name, c.version, h)
		if (err == nil) != c.ok {
			t.Errorf("Register(%q,%d) err=%v, want ok=%v", c.name, c.version, err, c.ok)
		}
	}
	var r sdk.Registry
	if err := sdk.Register(&r, "dup", 1, h); err != nil {
		t.Fatal(err)
	}
	if err := sdk.Register(&r, "dup", 1, h); err == nil {
		t.Fatal("duplicate registration accepted")
	}
	if err := sdk.Register[resizeIn, resizeOut](&r, "nilh", 1, nil); err == nil {
		t.Fatal("nil handler accepted")
	}
}

func TestInvokeTypedRoundTrip(t *testing.T) {
	var r sdk.Registry
	sdk.MustRegister(&r, "image.resize", 2, sdk.HandlerFunc[resizeIn, resizeOut](
		func(_ context.Context, j sdk.Job, p resizeIn) (resizeOut, error) {
			if j.Handler.Version != 2 {
				return resizeOut{}, fmt.Errorf("wrong version %d", j.Handler.Version)
			}
			return resizeOut{Area: p.Width * p.Width}, nil
		}))
	out, err := r.Invoke(context.Background(), job("image.resize", 2), []byte(`{"width":3}`))
	if err != nil || string(out) != `{"area":9}` {
		t.Fatalf("got %s, %v", out, err)
	}
	if _, err := r.Invoke(context.Background(), job("image.resize", 1), []byte(`{}`)); !errors.Is(err, sdk.ErrUnknownHandler) {
		t.Fatalf("want ErrUnknownHandler, got %v", err)
	}
	if got := r.Keys(); len(got) != 1 || got[0] != (sdk.HandlerKey{Name: "image.resize", Version: 2}) {
		t.Fatalf("Keys=%v", got)
	}
}

func TestInvokeFailureClasses(t *testing.T) {
	var r sdk.Registry
	sdk.MustRegister(&r, "x", 1, sdk.HandlerFunc[map[string]string, string](
		func(_ context.Context, _ sdk.Job, p map[string]string) (string, error) {
			switch p["mode"] {
			case "retry":
				return "", sdk.Retryable("upstream_busy", errors.New("503"))
			case "perm":
				return "", fmt.Errorf("wrapped: %w", sdk.Permanent("bad_input", errors.New("nope")))
			case "plain":
				return "", errors.New("boom")
			case "panic":
				panic("kaboom")
			case "big":
				return strings.Repeat("a", sdk.MaxResultBytes), nil
			}
			return "ok", nil
		}))
	sdk.MustRegister(&r, "strict", 1, sdk.HandlerFunc[resizeIn, int](
		func(context.Context, sdk.Job, resizeIn) (int, error) { return 0, nil }))

	cases := []struct {
		name, handler, payload string
		class                  sdk.Class
		code                   string
	}{
		{"retryable", "x", `{"mode":"retry"}`, sdk.ClassRetryable, "upstream_busy"},
		{"permanent wrapped", "x", `{"mode":"perm"}`, sdk.ClassPermanent, "bad_input"},
		{"untyped", "x", `{"mode":"plain"}`, sdk.ClassRetryable, sdk.CodeHandlerError},
		{"panic", "x", `{"mode":"panic"}`, sdk.ClassRetryable, sdk.CodeHandlerPanic},
		{"result too large", "x", `{"mode":"big"}`, sdk.ClassPermanent, sdk.CodeResultTooLarge},
		{"bad json", "x", `{`, sdk.ClassPermanent, sdk.CodeInvalidPayload},
		{"trailing data", "x", `{} {}`, sdk.ClassPermanent, sdk.CodeInvalidPayload},
		{"unknown field", "strict", `{"width":1,"height":2}`, sdk.ClassPermanent, sdk.CodeInvalidPayload},
		{"payload too large", "x", `"` + strings.Repeat("a", sdk.MaxPayloadBytes) + `"`, sdk.ClassPermanent, sdk.CodeInvalidPayload},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := r.Invoke(context.Background(), job(c.handler, 1), []byte(c.payload))
			if err == nil {
				t.Fatal("want error")
			}
			class, code := sdk.Classify(err)
			if class != c.class || code != c.code {
				t.Fatalf("Classify=%v/%s, want %v/%s (err=%v)", class, code, c.class, c.code, err)
			}
		})
	}
}

func TestInvalidCodeSanitized(t *testing.T) {
	if _, code := sdk.Classify(sdk.Permanent("Bad Code!", nil)); code != sdk.CodeHandlerError {
		t.Fatalf("code=%q", code)
	}
}

func TestRegistryConcurrentUse(t *testing.T) {
	var r sdk.Registry
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			name := fmt.Sprintf("h%d", i)
			sdk.MustRegister(&r, name, 1, sdk.HandlerFunc[int, int](func(_ context.Context, _ sdk.Job, p int) (int, error) { return p + 1, nil }))
			out, err := r.Invoke(context.Background(), job(name, 1), []byte(`41`))
			if err != nil || string(out) != "42" {
				t.Errorf("%s: %s %v", name, out, err)
			}
			_ = r.Keys()
		})
	}
	wg.Wait()
	if n := len(r.Keys()); n != 32 {
		t.Fatalf("keys=%d", n)
	}
}

func ExampleRegister() {
	var reg sdk.Registry
	type in struct {
		Name string `json:"name"`
	}
	sdk.MustRegister(&reg, "demo.greet", 1, sdk.HandlerFunc[in, string](
		func(ctx context.Context, job sdk.Job, p in) (string, error) {
			if p.Name == "" {
				return "", sdk.Permanent("missing_name", errors.New("name is required"))
			}
			return "hello " + p.Name, nil
		}))
	out, _ := reg.Invoke(context.Background(), sdk.Job{Handler: sdk.HandlerKey{Name: "demo.greet", Version: 1}}, []byte(`{"name":"ada"}`))
	fmt.Println(string(out), reg.Keys())
	// Output: "hello ada" [demo.greet@v1]
}

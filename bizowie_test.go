package bizowie

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNew_validation(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		want string
	}{
		{"missing site", Options{APIKey: "a", SecretKey: "b"}, "site not specified"},
		{"missing api_key", Options{Site: "s", SecretKey: "b"}, "api_key not specified"},
		{"missing secret_key", Options{Site: "s", APIKey: "a"}, "secret_key not specified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.opts); err == nil || err.Error() != tc.want {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestNew_constructs(t *testing.T) {
	bz, err := New(Options{APIKey: "a", SecretKey: "b", Site: "s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bz.apiKey != "a" || bz.secretKey != "b" || bz.site != "s" {
		t.Fatalf("fields not set correctly: %+v", bz)
	}
}

func TestCall_emptyMethod(t *testing.T) {
	bz, _ := New(Options{APIKey: "a", SecretKey: "b", Site: "s"})
	if _, err := bz.Call(context.Background(), "", nil); err == nil {
		t.Fatal("expected error for empty method")
	}
}

// roundTripper lets us hijack the HTTP client to inspect outgoing requests
// without a real server.
type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCall_payloadShape(t *testing.T) {
	var gotReq *http.Request
	var gotBody []byte

	client := &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		gotReq = r
		gotBody, _ = io.ReadAll(r.Body)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"success":1,"note_id":42}`)),
			Header:     http.Header{},
		}, nil
	})}

	bz, _ := New(Options{
		APIKey:     "k",
		SecretKey:  "s",
		Site:       "example.com",
		HTTPClient: client,
	})

	res, err := bz.Call(context.Background(), "some/method",
		map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("call returned error: %v", err)
	}

	if got, want := gotReq.URL.String(), "https://example.com/bz/apiv2/call/some/method"; got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
	if got := gotReq.Header.Get("Content-Type"); got != "form-data" {
		t.Fatalf("content-type = %q, want form-data", got)
	}
	if got := gotReq.Header.Get("User-Agent"); got != "Bizowie::API" {
		t.Fatalf("user-agent = %q, want Bizowie::API", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("body is not JSON: %v\nbody: %s", err, gotBody)
	}
	if payload["api_key"] != "k" || payload["secret_key"] != "s" {
		t.Errorf("auth not injected: %+v", payload)
	}
	if payload["api_version"] != "1.00" {
		t.Errorf("api_version = %v, want 1.00", payload["api_version"])
	}
	if payload["foo"] != "bar" {
		t.Errorf("user param dropped: %+v", payload)
	}

	if res.Success != 1 {
		t.Errorf("success = %d, want 1", res.Success)
	}
	if res.Data["note_id"] != float64(42) {
		t.Errorf("data[note_id] = %v, want 42", res.Data["note_id"])
	}
	if _, leaked := res.Data["success"]; leaked {
		t.Error("success field leaked into data")
	}
}

func TestCall_unprocessedOnNonJSON(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader(`<html>oops</html>`)),
			Header:     http.Header{},
		}, nil
	})}

	bz, _ := New(Options{APIKey: "k", SecretKey: "s", Site: "example.com", HTTPClient: client})
	res, err := bz.Call(context.Background(), "foo", nil)
	if err != nil {
		t.Fatalf("call returned error: %v", err)
	}
	if res.Success != 0 {
		t.Errorf("success = %d, want 0", res.Success)
	}
	if res.Data["unprocessed"] != 1 && res.Data["unprocessed"] != float64(1) {
		t.Errorf("unprocessed = %v, want 1", res.Data["unprocessed"])
	}
}

// Verifies real HTTP transport round-trip with httptest.
func TestCall_integration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		_, _ = io.WriteString(w, `{"success":1,"echo":"`+payload["api_key"].(string)+`"}`)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	bz, _ := New(Options{APIKey: "abc", SecretKey: "xyz", Site: u.Host})

	// Override the scheme to http for the test server (the client hardcodes https).
	bz.client = &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = u.Host
		return http.DefaultTransport.RoundTrip(r)
	})}

	res, err := bz.Call(context.Background(), "anything", nil)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.Success != 1 || res.Data["echo"] != "abc" {
		t.Fatalf("unexpected response: success=%d data=%v", res.Success, res.Data)
	}
}

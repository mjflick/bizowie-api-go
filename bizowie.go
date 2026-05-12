package bizowie

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

const userAgent = "Bizowie::API"

// Options configures a new API client. Only APIKey, SecretKey, and Site are
// required.
type Options struct {
	// APIKey is your Bizowie API key. Required.
	APIKey string
	// SecretKey is your Bizowie secret key. Required.
	SecretKey string
	// Site is the hostname of your Bizowie instance (e.g. "mysite.bizowie.com").
	// Required.
	Site string
	// APIVersion is the API version sent with each request. Defaults to
	// "1.00" when empty.
	APIVersion string
	// Debug, if true, logs the raw HTTP body to stderr when the response
	// can't be parsed as JSON.
	Debug bool
	// HTTPClient is an optional override for the underlying HTTP client.
	// If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// API is the Bizowie ERP API client.
type API struct {
	apiKey     string
	secretKey  string
	site       string
	apiVersion string
	debug      bool
	client     *http.Client
}

// Response is the result of a successful (transport-level) API call.
//
// Success is lifted from the response body's "success" field; the remaining
// fields are kept on Data. If the body could not be parsed as JSON, Data is
// {"unprocessed": 1} and Success is 0.
type Response struct {
	Data    map[string]any
	Success int
}

// New constructs an API client. It returns an error if APIKey, SecretKey, or
// Site is empty.
func New(opts Options) (*API, error) {
	if opts.Site == "" {
		return nil, errors.New("site not specified")
	}
	if opts.APIKey == "" {
		return nil, errors.New("api_key not specified")
	}
	if opts.SecretKey == "" {
		return nil, errors.New("secret_key not specified")
	}

	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	return &API{
		apiKey:     opts.APIKey,
		secretKey:  opts.SecretKey,
		site:       opts.Site,
		apiVersion: opts.APIVersion,
		debug:      opts.Debug,
		client:     client,
	}, nil
}

// Call makes an API call against the v2 endpoint (/bz/apiv2/call/).
//
// Call does not return an error for HTTP-level failures (4xx/5xx). Those are
// surfaced via Response.Success == 0 and whatever the server returned in
// Response.Data. Only transport-level failures (DNS, connection refused, TLS
// errors, context cancellation) are returned as errors.
//
// APIKey/SecretKey/APIVersion are injected automatically — don't include them
// in params.
func (a *API) Call(ctx context.Context, method string, params map[string]any) (*Response, error) {
	if method == "" {
		return nil, errors.New("[Bizowie::API] fatal error: no method given")
	}

	payload := map[string]any{}
	for k, v := range params {
		payload[k] = v
	}
	payload["api_key"] = a.apiKey
	payload["secret_key"] = a.secretKey
	if _, set := payload["api_version"]; !set {
		v := a.apiVersion
		if v == "" {
			v = "1.00"
		}
		payload["api_version"] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding payload: %w", err)
	}

	url := fmt.Sprintf("https://%s/bz/apiv2/call/%s", a.site, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "form-data")
	req.Header.Set("User-Agent", userAgent)

	return a.do(req)
}

func (a *API) do(req *http.Request) (*Response, error) {
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil || data == nil {
		if a.debug {
			fmt.Fprintf(os.Stderr, "[Bizowie::API] %s\n%s\n", resp.Status, bodyBytes)
		}
		data = map[string]any{"unprocessed": 1}
	}

	success := 0
	if raw, ok := data["success"]; ok {
		switch v := raw.(type) {
		case float64:
			success = int(v)
		case bool:
			if v {
				success = 1
			}
		case string:
			if v == "1" || v == "true" {
				success = 1
			}
		}
		delete(data, "success")
	}

	return &Response{Data: data, Success: success}, nil
}

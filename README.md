# bizowie-api-go

Go client for [Bizowie's](https://bizowie.com) ERP API. Port of the Perl
[`WWW::Bizowie::API`](https://github.com/bizowie/WWW-Bizowie-API) module.

- Zero dependencies (Go standard library only)
- Supports both the v1 and v2 API endpoints
- Context-aware (cancellation / deadlines via `context.Context`)

## Requirements

- Go **1.22 or newer**

## Install

```bash
go get github.com/bizowie/bizowie-api-go
```

> The module path above is a placeholder — replace it with the actual repo
> path where you publish (e.g. `github.com/mjflick/bizowie-api-go`) and
> update `go.mod` accordingly.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	bizowie "github.com/bizowie/bizowie-api-go"
)

func main() {
	bz, err := bizowie.New(bizowie.Options{
		APIKey:    "02cc7058-cd22-4c8e-ad7c-a8f3f2a64bd0",
		SecretKey: "58c57abc-1e16-3571-bb35-73876bcef746",
		Site:      "mysite.bizowie.com",
		V2:        true, // recommended for new integrations
	})
	if err != nil {
		log.Fatal(err)
	}

	res, err := bz.Call(context.Background(), "databases/add_note/3/10/123",
		map[string]any{"comment": "hello from Go"},
	)
	if err != nil {
		log.Fatal(err) // transport-level failure (DNS, TLS, context cancel, ...)
	}

	if res.Success == 1 {
		fmt.Println("ok:", res.Data)
	} else {
		fmt.Println("failed:", res.Data)
	}
}
```

## API

### `bizowie.New(opts Options) (*API, error)`

Creates a client. Returns an error if `APIKey`, `SecretKey`, or `Site` is empty.

#### `Options`

| Field         | Type            | Required | Description                                                                   |
| ------------- | --------------- | -------- | ----------------------------------------------------------------------------- |
| `APIKey`      | `string`        | yes      | Your Bizowie API key.                                                         |
| `SecretKey`   | `string`        | yes      | Your Bizowie secret key.                                                      |
| `Site`        | `string`        | yes      | Hostname of your Bizowie instance (e.g. `mysite.bizowie.com`).                |
| `V2`          | `bool`          | no       | Route calls through the v2 endpoint (`/bz/apiv2/call/`). Recommended.         |
| `APIVersion`  | `string`        | no       | API version sent with each v2 request. Defaults to `"1.00"` when empty.       |
| `Debug`       | `bool`          | no       | Log the raw HTTP body to stderr when the response can't be parsed as JSON.    |
| `HTTPClient`  | `*http.Client`  | no       | Override the underlying HTTP client. Defaults to `http.DefaultClient`.        |

### `(a *API) Call(ctx context.Context, method string, params map[string]any) (*Response, error)`

Makes an API call.

- `ctx` — standard `context.Context`. Cancel it to abort an in-flight call.
- `method` — path to the API method (everything after `/bz/api/` for v1 or
  `/bz/apiv2/call/` for v2). Returns an error if empty.
- `params` — `map[string]any` of parameters; JSON-encoded for you. May be
  `nil`. In v2 mode, `api_key` / `secret_key` / `api_version` are injected
  automatically — don't include them.

### `Response`

```go
type Response struct {
    Data    map[string]any
    Success int
}
```

- `Success` — `1` on success, `0` otherwise. Pulled from the response body's
  `success` field.
- `Data` — decoded JSON response (with `success` removed). On a non-JSON
  response this is `map[string]any{"unprocessed": 1}`.

## Error handling

`Call` does **not** return an error for HTTP-level failures (4xx/5xx) —
those are surfaced via `Success == 0` and whatever the server returned in
`Data`. Only transport-level failures (DNS, connection refused, TLS errors,
context cancellation) come back as errors:

```go
res, err := bz.Call(ctx, "some/method", params)
if err != nil {
    // transport-level failure
    return err
}
if res.Success != 1 {
    // application-level failure — see res.Data
}
```

If the server returns non-JSON (e.g., an HTML error page), `res.Success`
will be `0` and `res.Data` will be `map[string]any{"unprocessed": 1}`. Set
`Options.Debug = true` to log the raw body to stderr while wiring things up.

## Context support

`Call` accepts a `context.Context`, which means timeouts and cancellation
are standard Go patterns:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

res, err := bz.Call(ctx, "some/method", params)
```

## Custom HTTP client

Provide your own `*http.Client` via `Options.HTTPClient` to control timeouts,
proxies, TLS configuration, etc.:

```go
bz, _ := bizowie.New(bizowie.Options{
    APIKey:    "...",
    SecretKey: "...",
    Site:      "...",
    HTTPClient: &http.Client{Timeout: 10 * time.Second},
})
```

## v1 vs v2

| Aspect          | v1 (default)                                       | v2 (`V2: true`)                                    |
| --------------- | -------------------------------------------------- | -------------------------------------------------- |
| Endpoint        | `https://{site}/bz/api/{method}`                   | `https://{site}/bz/apiv2/call/{method}`            |
| Auth            | Sent as separate multipart form fields             | Injected into the JSON request body                |
| Body            | `multipart/form-data` with a `request` JSON field  | Raw JSON body with `Content-Type: form-data`       |
| `api_version`   | not sent                                           | sent (defaults to `"1.00"`)                        |

v2 is recommended for new integrations.

## License

Dual-licensed under the
[Artistic License 1.0](https://opensource.org/licenses/Artistic-1.0) or the
GPL 1.0+, matching the original Perl module.

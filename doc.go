// Package bizowie is a Go client for Bizowie's ERP API.
//
// Bizowie: https://bizowie.com
//
// It is a port of the Perl WWW::Bizowie::API module. The client targets the
// v2 API endpoint and uses only the Go standard library.
//
// # Example
//
//	bz, err := bizowie.New(bizowie.Options{
//	    APIKey:    "02cc7058-cd22-4c8e-ad7c-a8f3f2a64bd0",
//	    SecretKey: "58c57abc-1e16-3571-bb35-73876bcef746",
//	    Site:      "mysite.bizowie.com",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	res, err := bz.Call(context.Background(), "databases/add_note/3/10/123",
//	    map[string]any{"comment": "hi from Go"},
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if res.Success == 1 {
//	    fmt.Println("ok:", res.Data)
//	} else {
//	    fmt.Println("failed:", res.Data)
//	}
package bizowie

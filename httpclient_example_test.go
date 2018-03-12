package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/pkg/errors"
)

// drainBody reads all of b to memory and then returns two equivalent
// ReadClosers yielding the same bytes.
//
// It returns an error if the initial slurp of all bytes fails. It does not attempt
// to make the returned ReadClosers have identical error-matching behavior.
// https://golang.org/src/net/http/httputil/dump.go#L26
func drainBody(b io.ReadCloser) (r1, r2 io.ReadCloser, err error) {
	if b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return http.NoBody, http.NoBody, nil
	}
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, b, err
	}
	if err = b.Close(); err != nil {
		return nil, b, err
	}
	return ioutil.NopCloser(&buf), ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// This example shows how the RequestCallback function can be used to dump requests. Any pre-processing
// of requests would be possible using RequestCallback (eg. custom error checking).
func ExampleRequestCallbackFunc() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/example" {
			http.Error(w, "invalid", http.StatusNotFound)
			return
		}
	}))
	defer ts.Close()
	c, _ := New(ts.URL)

	// Overwrite the clients standard RequestCallback function with our custom function which dumps
	// some information for each request (r) to os.Stdout and returns the unmodified request.
	// Clients functionality is not affected by this, because the standard requestCallback function of
	// the client just returns the unmodified request (r).
	c.RequestCallback = func(r *http.Request) *http.Request {

		// to convert the request in an appropriate curl command
		// command, _ := http2curl.GetCurlCommand(r)

		fmt.Fprintf(os.Stdout, "Accept: %s\nContent-Type: %s\nMethod: %s", r.Header.Get("Accept"), r.Header.Get("Content-Type"), r.Method)
		return r
	}
	req, _ := c.NewRequest(http.MethodGet, "/example", nil)
	c.Do(context.Background(), req, nil)
	// Output:
	// Accept: application/json
	// Content-Type: application/json
	// Method: GET
}

// This example shows how the ResponseCallback function can be used to dump responses. Any post-processing
// of responses would be possible using ResponseCallback (eg. custom error checking).
func ExampleResponseCallbackFunc() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/example" {
			http.Error(w, "invalid", http.StatusNotFound)
			return
		}

		// check post data
		body, _ := ioutil.ReadAll(r.Body)
		w.Write(body)
	}))
	defer ts.Close()
	c, _ := New(ts.URL)

	type message struct {
		Text string
	}

	// Overwrite the clients standard ResponseCallback function with our custom function which dumps
	// some information for each response (r) to os.Stdout.
	// IMPORTANT: The clients standard responseCallback function returns an error if
	// http.StatusCode is outside of the 200 range. If you want to preserve the standard functionality consider adding the
	// if statement shown in this example.
	c.ResponseCallback = func(r *http.Response) (*http.Response, error) {
		// save the body of the response, because we consume it here but still want to return it to the caller
		var save io.ReadCloser
		save, r.Body, _ = drainBody(r.Body)
		body, _ := ioutil.ReadAll(r.Body)

		// print and then restore the body
		fmt.Fprintf(os.Stdout, "%s", body)
		r.Body = save

		// important to preserve standard clients behaviour
		if c := r.StatusCode; c >= 200 && c <= 299 {
			return r, nil
		}
		return r, errors.New(r.Status)
	}

	req, _ := c.NewRequest(http.MethodPost, "/example", &message{Text: "example"})
	c.Do(context.Background(), req, nil)
	// Output:
	// {"Text":"example"}
}

// Package httpclient provides the basic infrastructure for doing RESTish http requests
// an application specific client will be generated with the client-gen-go tool.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"golang.org/x/time/rate"

	yaml "gopkg.in/yaml.v2"

	"github.com/google/go-querystring/query"
	"github.com/pkg/errors"
)

// Constants
const (
	ContentTypeText = "text/plain"
	ContentTypeJSON = "application/json"
	ContentTypeYAML = "application/yaml"
)

// Variables
var (
	ErrUnknownContentType = errors.New("unknown media type")
	ErrTooManyRequest     = errors.New("too many requests")
)

// Client provides ....
type Client struct {
	// HTTP client used to communicate with the server
	client *http.Client

	// rate limiter
	limiter *rate.Limiter

	// Base URL for API requests.
	BaseURL *url.URL

	// ContentType is used as Content-Type and Accept in request headers.
	ContentType string

	// username/password for basic authentication.
	username string
	password string

	// if true, http.Response.Body will not be closed.
	keepResponseBody bool

	// custom http header(s)
	header http.Header

	Marshaler   MarshalerFunc
	Unmarshaler UnmarshalerFunc

	RequestCallback  RequestCallbackFunc
	ResponseCallback ResponseCallbackFunc
}

// Opt are options for New.
type Opt func(*Client) error

// RequestCallbackFunc for custom pre-processing of requests
// possible use cases: custom error checking, dumping requests for debugging etc.
type RequestCallbackFunc func(*http.Request) *http.Request

// ResponseCallbackFunc for custom post-processing of responses
// possible use cases: custom error checking, dumping responses for debugging etc.
type ResponseCallbackFunc func(*http.Response) (*http.Response, error)

// MarshalerFunc for custom marshaling function
type MarshalerFunc func(io.Writer, interface{}, string) (string, error)

// UnmarshalerFunc for custom unmarshaling function
type UnmarshalerFunc func(io.Reader, interface{}, string) error

// QueryOptions adds query options opt to URL u
// opt has to be a struct tagged according to https://github.com/google/go-querystring
// e.g.:
// type options struct {
//     Page    int    `url:"page,omitempty"`
//     PerPage int    `url:"per_page,omitempty"`
//     Search  string `url:"search,omitempty"`
// }
// opt := options{1, 10, "name=testHost"}
// ... will be added to URL u as "?page=1&per_page=10&search=name%3DtestHost"
func QueryOptions(u string, opt interface{}) (string, error) {
	v := reflect.ValueOf(opt)

	if v.Kind() == reflect.Ptr && v.IsNil() {
		return u, nil
	}

	origURL, err := url.Parse(u)
	if err != nil {
		return u, err
	}

	origValues := origURL.Query()

	newValues, err := query.Values(opt)
	if err != nil {
		return u, err
	}

	for k, v := range newValues {
		origValues[k] = v
	}

	origURL.RawQuery = origValues.Encode()
	return origURL.String(), nil
}

// New returns a new client instance.
func New(baseURL string, opts ...Opt) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		BaseURL:          u,
		ContentType:      ContentTypeJSON,
		Marshaler:        marshal,
		Unmarshaler:      unmarshal,
		RequestCallback:  requestCallback,
		ResponseCallback: responseCallback,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// WithPassword is a client option for setting the password for basic authentication.
func WithPassword(p string) Opt {
	return func(c *Client) error {
		if len(p) == 0 {
			return errors.New("password cannot be empty")
		}
		c.password = p
		return nil
	}
}

// WithUsername is a client option for setting the username.
func WithUsername(u string) Opt {
	return func(c *Client) error {
		if len(u) == 0 {
			return errors.New("username cannot be empty")
		}
		c.username = u
		return nil
	}
}

// WithHTTPClient is a client option for setting another http client than the default one
func WithHTTPClient(c *http.Client) Opt {
	return func(cli *Client) error {
		cli.client = c
		return nil
	}
}

// WithRateLimiter see https://godoc.org/golang.org/x/time/rate
func WithRateLimiter(l *rate.Limiter) Opt {
	return func(cli *Client) error {
		cli.limiter = l
		return nil
	}
}

// WithContentType is a client option for setting the content type
func WithContentType(ct string) Opt {
	return func(c *Client) error {
		if len(ct) == 0 {
			return errors.New("content type cannot be empty")
		}
		c.ContentType = ct
		return nil
	}
}

// WithHeader is a client option for setting custom http header(s) for each request
// Content-Type and Accept headers will be appended by the clients ContentType setting
// Authorization header is overwritten if WithUsername/WithPassowrd was used to setup the client
func WithHeader(header http.Header) Opt {
	return func(c *Client) error {
		c.header = header
		return nil
	}
}

// WithKeepResponseBody you are responsible for closing the http.Response.Body to prevent any
// resource leakages. Check http://engineering.rainchasers.com/golang/2015/03/03/memory-leak-missing-body-close.html
// for more details.
func WithKeepResponseBody() Opt {
	return func(c *Client) error {
		c.keepResponseBody = true
		return nil
	}
}

// NewRequest creates an API request. A relative URL can be provided in urlStr, which will be resolved to the
// BaseURL of the Client. Relative URLs should always be specified without a preceding slash. If specified, the
// value pointed to by body will be encoded and included in as the request body.
func (c *Client) NewRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.ResolveReference(rel)

	if c.Marshaler == nil {
		panic("Marshaler is nil")
	}

	buf := new(bytes.Buffer)
	contentType, err := c.Marshaler(buf, body, c.ContentType)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if c.header != nil {
		req.Header = c.header
	}

	if len(c.username) > 0 && len(c.password) > 0 {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Add("Content-Type", contentType)
	req.Header.Add("Accept", contentType)

	if c.RequestCallback == nil {
		panic("RequestCallback is nil")
	}
	return c.RequestCallback(req), nil
}

// marshal is the default marshaler
func marshal(w io.Writer, v interface{}, mediaType string) (string, error) {
	if v == nil {
		return mediaType, nil
	}
	switch mediaType {
	case ContentTypeJSON:
		return ContentTypeJSON, MarshalJSON(w, v, mediaType)
	case ContentTypeYAML:
		return ContentTypeYAML, MarshalYAML(w, v, mediaType)
	case ContentTypeText:
		_, err := fmt.Fprint(w, v)
		return ContentTypeText, err
	default:
		return mediaType, errors.Wrap(ErrUnknownContentType, mediaType)
	}
}

// MarshalJSON marshal JSON
func MarshalJSON(w io.Writer, v interface{}, mediaType string) error {
	return json.NewEncoder(w).Encode(v)
}

// MarshalYAML marshal JSON
func MarshalYAML(w io.Writer, v interface{}, mediaType string) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// Do sends an API request and returns the API response. The API response will be decoded and stored in the value
// pointed to by v, or returned as an error if an API error has occurred. If v implements the io.Writer interface,
// the raw response will be written to v, without attempting to decode it.
func (c *Client) Do(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {

	// rate limit
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, ErrTooManyRequest
		}
	}

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		return resp, err
	}

	deferFunc := func() {
		if rerr := resp.Body.Close(); rerr == nil {
			err = rerr
		}
	}

	var save io.ReadCloser
	if c.keepResponseBody {
		save, resp.Body, _ = drainBody(resp.Body)
		deferFunc = func() {
			resp.Body = save
		}
	}

	defer deferFunc()

	if c.ResponseCallback == nil {
		panic("ResponseCallback is nil")
	}

	resp, err = c.ResponseCallback(resp)
	if err != nil {
		return resp, err
	}

	if c.Unmarshaler == nil {
		panic("Unmarshaler is nil")
	}

	if err = c.Unmarshaler(resp.Body, v, c.ContentType); err != nil {
		return resp, err
	}

	return resp, err
}

// unmarshal is the default unmarshaler
func unmarshal(r io.Reader, v interface{}, mediaType string) error {
	if v == nil {
		return nil
	}
	// if v is a io.Writer copy the request body to v
	if w, ok := v.(io.Writer); ok {
		_, err := io.Copy(w, r)
		return err
	}

	switch mediaType {
	case ContentTypeJSON:
		return UnmarshalJSON(r, v, mediaType)
	case ContentTypeYAML:
		return UnmarshalYAML(r, v, mediaType)
	case ContentTypeText:
		if x, ok := v.(*string); ok {
			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(r); err != nil {
				return errors.Wrap(err, "read into buffer")
			}

			*x = buf.String()
			return nil
		}
		return errors.New("target type is not string")
	default:
		return errors.Wrap(ErrUnknownContentType, mediaType)
	}
}

// UnmarshalJSON unmarshal JSON
func UnmarshalJSON(r io.Reader, v interface{}, mediaType string) error {
	return json.NewDecoder(r).Decode(v)
}

// UnmarshalYAML unmarshal YAML
func UnmarshalYAML(r io.Reader, v interface{}, mediaType string) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

// requestCallback returns the unmodified request
func requestCallback(r *http.Request) *http.Request {
	return r
}

// responseCallback checks the API response for errors, and returns them if present. A response is considered an
// error if it has a status code outside the 200 range. API error responses are expected to have no response body.
func responseCallback(r *http.Response) (*http.Response, error) {
	if c := r.StatusCode; c >= 200 && c <= 299 {
		return r, nil
	}
	return r, errors.New(r.Status)
}

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

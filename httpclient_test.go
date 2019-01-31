package httpclient

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"golang.org/x/time/rate"

	"github.com/moul/http2curl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type options struct {
	Page    int    `url:"page,omitempty"`
	PerPage int    `url:"per_page,omitempty"`
	Search  string `url:"search,omitempty"`
}

type message struct {
	Text string
}

func (m message) String() string {
	return m.Text
}

var (
	baseurl        = "https://hostname.domain"
	baseurlInvalid = "https://show you how deep the rabbit hole goes.domain"
	username       = "cognitive"
	password       = "distortions"
	contentType    = "application/yaml"
	testMessage    = message{
		Text: "it's only rock'n'roll",
	}
)

func TestClient(t *testing.T) {
	t.Run("query options", func(t *testing.T) {
		opt := options{1, 10, "name=testHost"}
		u, err := QueryOptions(baseurl, opt)
		assert.Nil(t, err)
		assert.Equal(t, "https://hostname.domain?page=1&per_page=10&search=name%3DtestHost", u)
	})

	t.Run("query options nil", func(t *testing.T) {
		var opt *options
		u, err := QueryOptions(baseurl, opt)
		assert.Nil(t, err)
		assert.Equal(t, baseurl, u)
	})

	t.Run("baseurl invalid", func(t *testing.T) {
		u, err := QueryOptions(baseurlInvalid, nil)
		assert.NotNil(t, err)
		assert.Equal(t, baseurlInvalid, u)
	})

	t.Run("query options not query values", func(t *testing.T) {
		opt := 42
		u, err := QueryOptions(baseurl, opt)
		assert.NotNil(t, err)
		assert.Equal(t, baseurl, u)
	})

	t.Run("new client invalid baseurl", func(t *testing.T) {
		_, err := New(baseurlInvalid)
		assert.NotNil(t, err)
	})

	t.Run("new client valid baseurl", func(t *testing.T) {
		c, err := New(baseurl)
		assert.Nil(t, err)
		assert.NotNil(t, c)
		u, _ := url.Parse(baseurl)
		assert.Equal(t, u, c.BaseURL)
	})

	t.Run("new client valid baseurl invalid username", func(t *testing.T) {
		_, err := New(baseurl, WithUsername(""))
		assert.NotNil(t, err)
	})

	t.Run("new client valid baseurl valid username", func(t *testing.T) {
		c, err := New(baseurl, WithUsername(username))
		assert.Nil(t, err)
		assert.NotNil(t, c)
		assert.Equal(t, username, c.username)
	})

	t.Run("new client valid baseurl invalid password", func(t *testing.T) {
		_, err := New(baseurl, WithPassword(""))
		assert.NotNil(t, err)
	})

	t.Run("new client valid baseurl valid password", func(t *testing.T) {
		c, err := New(baseurl, WithPassword(password))
		assert.Nil(t, err)
		assert.NotNil(t, c)
		assert.Equal(t, password, c.password)
	})

	t.Run("new client with headers", func(t *testing.T) {
		customHeader := http.Header{
			"X-Requested-By": []string{"test"},
		}
		c, err := New(baseurl, WithHeader(customHeader))
		assert.Nil(t, err)
		assert.NotNil(t, c)

		req, err := c.NewRequest(http.MethodGet, "/test", nil)
		assert.Nil(t, err)
		assert.Equal(t, req.Header["X-Requested-By"], []string{"test"})
		assert.Contains(t, req.Header, "Content-Type")
		assert.Contains(t, req.Header, "Accept")
	})

	t.Run("new client with headers and basic auth", func(t *testing.T) {
		username := "user1"
		password := "123456"
		fakeAuthHeader := http.Header{
			"Authorization": []string{"fake"},
		}

		c, err := New(baseurl, WithHeader(fakeAuthHeader), WithUsername(username), WithPassword(password))
		assert.Nil(t, err)
		assert.NotNil(t, c)

		req, err := c.NewRequest(http.MethodGet, "/test", nil)
		assert.Nil(t, err)
		user, passwd, ok := req.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, user, username)
		assert.Equal(t, passwd, passwd)
	})

	t.Run("new client valid baseurl valid HTTP client", func(t *testing.T) {
		httpC := &http.Client{}
		c, err := New(baseurl, WithHTTPClient(httpC))
		assert.Nil(t, err)
		assert.NotNil(t, c)
		assert.True(t, httpC == c.client)
	})

	t.Run("new client valid baseurl valid rate limiter", func(t *testing.T) {
		limiter := rate.NewLimiter(1.0, 1)
		c, err := New(baseurl, WithRateLimiter(limiter))
		assert.Nil(t, err)
		assert.NotNil(t, c)
		assert.True(t, limiter == c.limiter)
	})

	t.Run("new client valid baseurl valid content type", func(t *testing.T) {
		_, err := New(baseurl, WithContentType(contentType))
		assert.Nil(t, err)
	})

	t.Run("new client valid baseurl invalid content type", func(t *testing.T) {
		_, err := New(baseurl, WithContentType(""))
		assert.NotNil(t, err)
	})

	t.Run("new request with Marshaler == nil", func(t *testing.T) {
		defer func() {
			assert.NotNil(t, recover())
		}()
		c, _ := New(baseurl)
		c.Marshaler = nil
		_, _ = c.NewRequest(http.MethodGet, "node", nil)
		assert.Fail(t, "NewRequest did not panic")
	})

	t.Run("new request with RequestCallback == nil", func(t *testing.T) {
		defer func() {
			assert.NotNil(t, recover())
		}()
		c, _ := New(baseurl)
		c.RequestCallback = nil
		_, _ = c.NewRequest(http.MethodGet, "node", nil)
		assert.Fail(t, "NewRequest did not panic")
	})

	t.Run("new request using RequestCallback to dump request", func(t *testing.T) {
		c, _ := New(baseurl)
		var dump []byte
		c.RequestCallback = func(r *http.Request) *http.Request {
			dump, _ = httputil.DumpRequestOut(r, true)
			return r
		}
		_, _ = c.NewRequest(http.MethodGet, "node", nil)
		assert.True(t, len(dump) > 0)
		//t.Log(string(dump))
	})

	t.Run("new request using RequestCallback to dump request", func(t *testing.T) {
		c, _ := New(baseurl)
		var command *http2curl.CurlCommand
		c.RequestCallback = func(r *http.Request) *http.Request {
			command, _ = http2curl.GetCurlCommand(r)
			return r
		}
		_, _ = c.NewRequest(http.MethodGet, "node", nil)
		assert.True(t, len(command.String()) > 0)
		//t.Log(string(command.String()))
	})

	t.Run("new request without basic auth", func(t *testing.T) {
		c, err := New(baseurl)
		assert.Nil(t, err)
		assert.NotNil(t, c)
		req, err := c.NewRequest(http.MethodGet, "node", nil)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		_, _, ok := req.BasicAuth()
		assert.False(t, ok)
	})

	t.Run("new request with basic auth", func(t *testing.T) {
		c, err := New(baseurl, WithUsername(username), WithPassword(password))
		assert.Nil(t, err)
		assert.NotNil(t, c)
		req, err := c.NewRequest(http.MethodGet, "node", nil)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		u, p, ok := req.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, username, u)
		assert.Equal(t, password, p)
	})

	t.Run("new request with content type application/json", func(t *testing.T) {
		c, err := New(baseurl)
		assert.Nil(t, err)
		assert.NotNil(t, c)
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, req.Body)
		assert.Nil(t, err)
		assert.Equal(t, "{\"Text\":\"it's only rock'n'roll\"}\n", buf.String())
	})

	t.Run("new request with content type application/yaml", func(t *testing.T) {
		c, err := New(baseurl)
		c.ContentType = ContentTypeYAML
		assert.Nil(t, err)
		assert.NotNil(t, c)
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, req.Body)
		assert.Nil(t, err)
		assert.Equal(t, "text: it's only rock'n'roll\n", buf.String())
	})

	t.Run("new request with content type text/plain", func(t *testing.T) {
		c, err := New(baseurl)
		c.ContentType = ContentTypeText
		assert.Nil(t, err)
		assert.NotNil(t, c)
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, req.Body)
		assert.Nil(t, err)
		assert.Equal(t, "it's only rock'n'roll", buf.String())
	})

	t.Run("new request with unknown content type ", func(t *testing.T) {
		c, err := New(baseurl)
		c.ContentType = "unknown/unknown"
		assert.Nil(t, err)
		assert.NotNil(t, c)
		_, err = c.NewRequest(http.MethodGet, "node", struct{ Message string }{Message: "it's only rock'n'roll"})
		assert.NotNil(t, err)
	})

	// Test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/node" {
			http.Error(w, "invalid", http.StatusNotFound)
			return
		}
		// check post data
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		switch r.Header.Get("Content-Type") {
		case ContentTypeJSON:
			w.Header().Set("Content-Type", ContentTypeJSON)
		case ContentTypeYAML:
			w.Header().Set("Content-Type", ContentTypeYAML)
		case ContentTypeText:
			w.Header().Set("Content-Type", ContentTypeText)
		default:
			w.Header().Set("Content-Type", "unknown/unknown")
		}

		_, err = w.Write(body)
		if err != nil {
			t.Fatal(err)
		}
	}))
	defer ts.Close()
	t.Log("test server URL:", ts.URL)

	t.Run("do a request with Unmarshaler == nil", func(t *testing.T) {
		defer func() {
			assert.NotNil(t, recover())
		}()
		c, _ := New(ts.URL)
		c.Unmarshaler = nil
		ctx := context.Background()
		act := &message{}
		req, err := c.NewRequest(http.MethodGet, "node", act)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		_, _ = c.Do(ctx, req, act)
		assert.Fail(t, "Do did not panic")
	})

	t.Run("do a request with ResponseCallback == nil", func(t *testing.T) {
		defer func() {
			assert.NotNil(t, recover())
		}()
		c, _ := New(ts.URL)
		c.ResponseCallback = nil
		ctx := context.Background()
		act := &message{}
		req, err := c.NewRequest(http.MethodGet, "node", act)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		_, _ = c.Do(ctx, req, act)
		assert.Fail(t, "Do did not panic")
	})

	t.Run("do a request with a writer", func(t *testing.T) {
		c, _ := New(ts.URL)
		ctx := context.Background()
		act := &message{}
		req, err := c.NewRequest(http.MethodGet, "node", act)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		var buf bytes.Buffer
		_, err = c.Do(ctx, req, bufio.NewWriter(&buf))
		assert.Nil(t, err)
		assert.True(t, buf.Len() > 0)
	})

	t.Run("do a request with content type application/json", func(t *testing.T) {
		c, _ := New(ts.URL)
		ctx := context.Background()
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		act := &message{}
		_, err = c.Do(ctx, req, act)
		assert.Nil(t, err)
		assert.Equal(t, &testMessage, act)
	})

	t.Run("do a request with content type application/yaml", func(t *testing.T) {
		c, _ := New(ts.URL)
		c.ContentType = ContentTypeYAML
		ctx := context.Background()
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		act := &message{}
		_, err = c.Do(ctx, req, act)
		assert.Nil(t, err)
		assert.Equal(t, &testMessage, act)
	})

	t.Run("do a request with content type text/plain", func(t *testing.T) {
		c, _ := New(ts.URL)
		c.ContentType = ContentTypeText
		ctx := context.Background()
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		act := ""
		_, err = c.Do(ctx, req, &act)
		assert.Nil(t, err)
		assert.Equal(t, testMessage.String(), act)
	})

	t.Run("do a request with content type text/plain and provide non-string-type", func(t *testing.T) {
		c, _ := New(ts.URL)
		c.ContentType = ContentTypeText
		ctx := context.Background()
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		act := 42
		_, err = c.Do(ctx, req, &act)
		assert.NotNil(t, err)
	})

	t.Run("do a request with error in response using default ResponseCallback", func(t *testing.T) {
		c, _ := New(ts.URL)
		ctx := context.Background()
		act := &message{}
		req, err := c.NewRequest(http.MethodGet, "invalid", act)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		var buf bytes.Buffer
		_, err = c.Do(ctx, req, bufio.NewWriter(&buf))
		assert.NotNil(t, err)
		assert.Equal(t, "404 Not Found", err.Error())
	})

	t.Run("do a request using ResponseCallback to dump the response", func(t *testing.T) {
		c, _ := New(ts.URL)
		var dump []byte
		c.ResponseCallback = func(r *http.Response) (*http.Response, error) {
			dump, _ = httputil.DumpResponse(r, true)
			return r, nil
		}
		ctx := context.Background()
		act := &message{}
		req, err := c.NewRequest(http.MethodGet, "node", act)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		var buf bytes.Buffer
		_, err = c.Do(ctx, req, bufio.NewWriter(&buf))
		assert.Nil(t, err)
		assert.True(t, len(dump) > 0)
		//t.Log(string(dump))
	})

	t.Run("do a request with content type unknown/unknown in response to test unmarshal behaviour", func(t *testing.T) {
		c, _ := New(ts.URL)
		c.ContentType = ContentTypeText
		c.ResponseCallback = func(r *http.Response) (*http.Response, error) {
			r.Header.Set("Content-Type", "unknown/unknown")
			return r, nil
		}
		ctx := context.Background()
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		assert.Nil(t, err)
		assert.NotNil(t, req)
		act := 42
		_, err = c.Do(ctx, req, &act)
		assert.NotNil(t, err)
	})

	t.Run("do a request WithKeepResponseBody and verify body is not closed", func(t *testing.T) {
		c, _ := New(ts.URL, WithKeepResponseBody())
		ctx := context.Background()
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		require.NoError(t, err)
		assert.NotNil(t, req)
		r, err := c.Do(ctx, req, nil)
		require.NoError(t, err)
		_, err = ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
	})

	t.Run("do a request without WithKeepResponseBody and verify body is closed", func(t *testing.T) {
		c, _ := New(ts.URL)
		ctx := context.Background()
		req, err := c.NewRequest(http.MethodGet, "node", testMessage)
		require.NoError(t, err)
		assert.NotNil(t, req)
		r, err := c.Do(ctx, req, nil)
		require.NoError(t, err)
		_, err = ioutil.ReadAll(r.Body)
		assert.Error(t, err)
	})
}

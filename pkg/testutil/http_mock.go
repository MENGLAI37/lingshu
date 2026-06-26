package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// HTTP Mock helpers
// ===========================================================================

// HTTPHandler is a function that handles HTTP requests.
type HTTPHandler func(w http.ResponseWriter, r *http.Request)

// NewTestServer creates a test HTTP server.
func NewTestServer(t *testing.T, handler HTTPHandler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handler))
}

// NewTestServerWithHandler creates a test server with an http.Handler.
func NewTestServerWithHandler(t *testing.T, handler http.Handler) *httptest.Server {
	return httptest.NewServer(handler)
}

// DoGET performs a GET request to the test server.
func DoGET(t *testing.T, server *httptest.Server, path string) *http.Response {
	req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
	require.NoError(t, err)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	
	return resp
}

// DoPOST performs a POST request to the test server.
func DoPOST(t *testing.T, server *httptest.Server, path, body string) *http.Response {
	req, err := http.NewRequest(http.MethodPost, server.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	
	return resp
}

// DoDELETE performs a DELETE request to the test server.
func DoDELETE(t *testing.T, server *httptest.Server, path string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete, server.URL+path, nil)
	require.NoError(t, err)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	
	return resp
}

// AssertStatusCode asserts the response status code.
func AssertStatusCode(t *testing.T, resp *http.Response, expected int) {
	assert.Equal(t, expected, resp.StatusCode)
}

// AssertHeader asserts a response header value.
func AssertHeader(t *testing.T, resp *http.Response, key, expected string) {
	assert.Equal(t, expected, resp.Header.Get(key))
}

// AssertContentType asserts the content type header.
func AssertContentType(t *testing.T, resp *http.Response, expected string) {
	AssertHeader(t, resp, "Content-Type", expected)
}

// ===========================================================================
// Context helpers for HTTP
// ===========================================================================

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, "request_id", requestID)
}

// GetRequestID gets the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value("request_id").(string); ok {
		return id
	}
	return ""
}

// WithUserID adds a user ID to the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, "user_id", userID)
}

// GetUserID gets the user ID from the context.
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value("user_id").(string); ok {
		return id
	}
	return ""
}

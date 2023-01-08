package mocks

import "net/http"

// MockClient is the mock client.
type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

// GetDoFunc fetches the mock client's `Do` func.
var GetDoFunc func(req *http.Request) (*http.Response, error)

// Do is the mock client's `Do` func.
func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	return GetDoFunc(req)
}

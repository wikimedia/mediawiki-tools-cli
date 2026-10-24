package cli

import "net/http"

// NewHTTPClient copies the default client and identifies CLI requests, including
// redirects. Callers can still configure timeouts and redirect policies.
func NewHTTPClient() *http.Client {
	return WithHTTPUserAgent(http.DefaultClient)
}

// WithHTTPUserAgent copies client and wraps its transport, preserving timeouts
// and redirect policies. Requests with an explicit User-Agent keep that value.
func WithHTTPUserAgent(client *http.Client) *http.Client {
	copy := *client
	base := copy.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copy.Transport = userAgentTransport{base: base}
	return &copy
}

type userAgentTransport struct {
	base http.RoundTripper
}

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") != "" {
		return t.base.RoundTrip(req)
	}
	// RoundTrippers must not mutate the caller's request.
	identified := req.Clone(req.Context())
	if identified.Header == nil {
		identified.Header = make(http.Header)
	}
	identified.Header.Set("User-Agent", HTTPUserAgent)
	return t.base.RoundTrip(identified)
}

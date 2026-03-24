// Package fetch provides a resource fetcher for URI objects in RDF triples.
//
// This package provides HTTP-based retrieval of resources referenced by URI
// objects in RDF triples. Fetching is opt-in and never recursive — only one
// hop is performed per URI. This is useful for linked data enrichment and
// ontology inference from external sources.
//
// The Fetcher supports configurable timeouts via the WithTimeout option and
// returns types.Resource containing the response body, content type, or any
// errors encountered. It's designed to work alongside the rdf and ttl parsers
// for processing external ontology references.
//
// Example:
//
//	fetcher := fetch.New(fetch.WithTimeout(5*time.Second))
//	resource, err := fetcher.FetchURI("https://example.org/ontology.rdf")
//	if err != nil {
//	    // handle error
//	}
//	fmt.Println(string(resource.Body))
package fetch

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/soypete/ontology-go/types"
)

// DefaultTimeout is the default HTTP request timeout.
const DefaultTimeout = 10 * time.Second

// Fetcher retrieves resources at URI objects found in RDF triples.
type Fetcher struct {
	client  *http.Client
	timeout time.Duration
}

// Option configures a Fetcher.
type Option func(*Fetcher)

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) Option {
	return func(f *Fetcher) {
		f.timeout = d
	}
}

// WithHTTPClient sets a custom HTTP client (useful for testing).
func WithHTTPClient(c *http.Client) Option {
	return func(f *Fetcher) {
		f.client = c
	}
}

// New creates a new Fetcher with the given options.
func New(opts ...Option) *Fetcher {
	f := &Fetcher{
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(f)
	}
	if f.client == nil {
		f.client = &http.Client{Timeout: f.timeout}
	}
	return f
}

// IsURI returns true if the value looks like an HTTP(S) URI.
func IsURI(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// Fetch retrieves the resource at the given URI.
// On any failure (timeout, DNS, HTTP error), the returned Resource has Error set
// and the function does not return a Go error — it never fails the caller.
func (f *Fetcher) Fetch(uri string) types.Resource {
	if !IsURI(uri) {
		return types.Resource{
			URI:   uri,
			Error: "not an HTTP(S) URI",
		}
	}

	resp, err := f.client.Get(uri)
	if err != nil {
		return types.Resource{
			URI:   uri,
			Error: fmt.Sprintf("fetch failed: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return types.Resource{
			URI:   uri,
			Error: fmt.Sprintf("HTTP %d %s", resp.StatusCode, resp.Status),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.Resource{
			URI:         uri,
			ContentType: resp.Header.Get("Content-Type"),
			Error:       fmt.Sprintf("read body failed: %v", err),
		}
	}

	return types.Resource{
		URI:         uri,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        body,
	}
}

// FetchAll fetches all unique URI objects found in the given triples.
// Returns a map of URI to Resource. Failures are recorded in Resource.Error;
// the map is always returned (never nil).
func (f *Fetcher) FetchAll(triples []types.Triple) map[string]types.Resource {
	resources := make(map[string]types.Resource)
	seen := make(map[string]bool)

	for _, t := range triples {
		if IsURI(t.Object) && !seen[t.Object] {
			seen[t.Object] = true
			resources[t.Object] = f.Fetch(t.Object)
		}
	}

	return resources
}

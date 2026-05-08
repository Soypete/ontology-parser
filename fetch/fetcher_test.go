package fetch

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/soypete/ontology-go/types"
)

func TestFetcher_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer server.Close()

	f := New(WithHTTPClient(server.Client()))
	// Override the base URL to use the test server
	res := f.Fetch(server.URL + "/resource")

	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if res.ContentType != "text/plain" {
		t.Errorf("expected text/plain, got %q", res.ContentType)
	}
	if string(res.Body) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(res.Body))
	}
}

func TestFetcher_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	f := New(WithHTTPClient(server.Client()))
	res := f.Fetch(server.URL + "/missing")

	if res.Error == "" {
		t.Error("expected error for 404, got none")
	}
}

func TestFetcher_500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	f := New(WithHTTPClient(server.Client()))
	res := f.Fetch(server.URL + "/error")

	if res.Error == "" {
		t.Error("expected error for 500, got none")
	}
}

func TestFetcher_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte("too slow"))
	}))
	defer server.Close()

	f := New(WithTimeout(100 * time.Millisecond))
	res := f.Fetch(server.URL + "/slow")

	if res.Error == "" {
		t.Error("expected timeout error, got none")
	}
	if res.URI != server.URL+"/slow" {
		t.Errorf("expected URI to be preserved, got %q", res.URI)
	}
}

func TestFetcher_DNSFailure(t *testing.T) {
	f := New(WithTimeout(1 * time.Second))
	res := f.Fetch("http://this-domain-does-not-exist-xyz123.invalid/path")

	if res.Error == "" {
		t.Error("expected DNS failure error, got none")
	}
}

func TestFetcher_NotURI(t *testing.T) {
	f := New()
	res := f.Fetch("not-a-uri")

	if res.Error == "" {
		t.Error("expected error for non-URI, got none")
	}
}

func TestFetcher_ContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdf+xml")
		_, _ = w.Write([]byte("<rdf>data</rdf>"))
	}))
	defer server.Close()

	f := New(WithHTTPClient(server.Client()))
	res := f.Fetch(server.URL + "/rdf")

	if res.ContentType != "application/rdf+xml" {
		t.Errorf("expected application/rdf+xml, got %q", res.ContentType)
	}
}

func TestFetcher_FetchAll(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("data for " + r.URL.Path))
	}))
	defer server.Close()

	f := New(WithHTTPClient(server.Client()))

	triples := []types.Triple{
		{Subject: "s1", Predicate: "p1", Object: server.URL + "/a"},
		{Subject: "s2", Predicate: "p2", Object: server.URL + "/b"},
		{Subject: "s3", Predicate: "p3", Object: server.URL + "/a"}, // duplicate URI
		{Subject: "s4", Predicate: "p4", Object: "not-a-uri"},       // should be skipped
	}

	resources := f.FetchAll(triples)

	// Should have fetched 2 unique URIs
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Check /a was fetched
	resA, ok := resources[server.URL+"/a"]
	if !ok {
		t.Fatal("missing resource for /a")
	}
	if resA.Error != "" {
		t.Errorf("unexpected error for /a: %s", resA.Error)
	}

	// Check /b was fetched
	resB, ok := resources[server.URL+"/b"]
	if !ok {
		t.Fatal("missing resource for /b")
	}
	if resB.Error != "" {
		t.Errorf("unexpected error for /b: %s", resB.Error)
	}

	// Should not have fetched the duplicate
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}

func TestIsURI(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"http://example.org", true},
		{"https://example.org", true},
		{"ftp://example.org", false},
		{"not-a-uri", false},
		{"", false},
		{"mailto:user@example.org", false},
	}

	for _, tc := range tests {
		got := IsURI(tc.input)
		if got != tc.want {
			t.Errorf("IsURI(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

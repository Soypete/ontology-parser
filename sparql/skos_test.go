package query

import (
	"testing"

	"github.com/soypete/ontology-go/store"
	"github.com/soypete/ontology-go/types"
)

func TestSKOSConstants(t *testing.T) {
	if SKOSNS != "http://www.w3.org/2004/02/skos/core#" {
		t.Errorf("SKOSNS mismatch: got %s", SKOSNS)
	}
	if SKOSBroader != SKOSNS+"broader" {
		t.Errorf("SKOSBroader mismatch")
	}
	if SKOSNarrower != SKOSNS+"narrower" {
		t.Errorf("SKOSNarrower mismatch")
	}
	if SKOSRelated != SKOSNS+"related" {
		t.Errorf("SKOSRelated mismatch")
	}
	if SKOSExactMatch != SKOSNS+"exactMatch" {
		t.Errorf("SKOSExactMatch mismatch")
	}
	if SKOSCloseMatch != SKOSNS+"closeMatch" {
		t.Errorf("SKOSCloseMatch mismatch")
	}
}

func TestSKOSInferenceOption(t *testing.T) {
	tests := []struct {
		name  string
		value SKOSInferenceOption
	}{
		{"SKOSInferenceNone", SKOSInferenceNone},
		{"SKOSInferenceBroader", SKOSInferenceBroader},
		{"SKOSInferenceNarrower", SKOSInferenceNarrower},
		{"SKOSInferenceRelated", SKOSInferenceRelated},
		{"SKOSInferenceExactMatch", SKOSInferenceExactMatch},
		{"SKOSInferenceCloseMatch", SKOSInferenceCloseMatch},
		{"SKOSInferenceAll", SKOSInferenceAll},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value < SKOSInferenceNone || tt.value > SKOSInferenceAll {
				t.Errorf("unexpected SKOSInferenceOption value: %d", tt.value)
			}
		})
	}
}

func TestAuthorityMatchMode(t *testing.T) {
	tests := []struct {
		name  string
		value AuthorityMatchMode
	}{
		{"AuthorityMatchNone", AuthorityMatchNone},
		{"AuthorityMatchFull", AuthorityMatchFull},
		{"AuthorityMatchAuthority", AuthorityMatchAuthority},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value < AuthorityMatchNone || tt.value > AuthorityMatchAuthority {
				t.Errorf("unexpected AuthorityMatchMode value: %d", tt.value)
			}
		})
	}
}

func TestDefaultSKOSOptions(t *testing.T) {
	if defaultSKOSOptions.Inference != SKOSInferenceNone {
		t.Errorf("expected default Inference to be SKOSInferenceNone")
	}
	if defaultSKOSOptions.AuthorityMode != AuthorityMatchNone {
		t.Errorf("expected default AuthorityMode to be AuthorityMatchNone")
	}
}

func TestWithSKOSInference(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	engine.ApplyOption(WithSKOSInference(SKOSInferenceAll))
	if engine.skosOptions.Inference != SKOSInferenceAll {
		t.Errorf("expected SKOSInferenceAll, got %v", engine.skosOptions.Inference)
	}
}

func TestWithAuthorityMatch(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	engine.ApplyOption(WithAuthorityMatch(AuthorityMatchAuthority))
	if engine.skosOptions.AuthorityMode != AuthorityMatchAuthority {
		t.Errorf("expected AuthorityMatchAuthority, got %v", engine.skosOptions.AuthorityMode)
	}
}

func TestEngine_SetGetSKOSOptions(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	opts := SKOSOptions{
		Inference:     SKOSInferenceAll,
		AuthorityMode: AuthorityMatchAuthority,
	}
	engine.SetSKOSOptions(opts)

	got := engine.GetSKOSOptions()
	if got.Inference != opts.Inference {
		t.Errorf("expected Inference %v, got %v", opts.Inference, got.Inference)
	}
	if got.AuthorityMode != opts.AuthorityMode {
		t.Errorf("expected AuthorityMode %v, got %v", opts.AuthorityMode, got.AuthorityMode)
	}
}

func TestEngine_inferSKOSTriples_NoInference(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)
	engine.SetSKOSOptions(SKOSOptions{Inference: SKOSInferenceNone})

	triples := []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSBroader, Object: "http://example.org/b"},
	}

	result := engine.inferSKOSTriples(triples)
	if len(result) != 1 {
		t.Errorf("expected 1 triple, got %d", len(result))
	}
}

func TestEngine_inferBroaderNarrower_Broader(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/child", Predicate: SKOSBroader, Object: "http://example.org/parent", Graph: "test"},
	}

	result := engine.inferBroaderNarrower(triples, false)
	if len(result) != 1 {
		t.Fatalf("expected 1 inferred triple, got %d", len(result))
	}

	if result[0].Predicate != SKOSNarrower {
		t.Errorf("expected predicate %s, got %s", SKOSNarrower, result[0].Predicate)
	}
	if result[0].Subject != "http://example.org/child" {
		t.Errorf("expected subject child, got %s", result[0].Subject)
	}
	if result[0].Object != "http://example.org/parent" {
		t.Errorf("expected object parent, got %s", result[0].Object)
	}
}

func TestEngine_inferBroaderNarrower_Narrower(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/parent", Predicate: SKOSNarrower, Object: "http://example.org/child", Graph: "test"},
	}

	result := engine.inferBroaderNarrower(triples, true)
	if len(result) != 1 {
		t.Fatalf("expected 1 inferred triple, got %d", len(result))
	}

	if result[0].Predicate != SKOSBroader {
		t.Errorf("expected predicate %s, got %s", SKOSBroader, result[0].Predicate)
	}
}

func TestEngine_inferBroaderNarrower_Transitive(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/grandchild", Predicate: SKOSBroader, Object: "http://example.org/child", Graph: "test"},
		{Subject: "http://example.org/child", Predicate: SKOSBroader, Object: "http://example.org/parent", Graph: "test"},
	}

	result := engine.inferBroaderNarrower(triples, false)
	found := false
	for _, t := range result {
		if t.Subject == "http://example.org/grandchild" && t.Predicate == SKOSNarrower && t.Object == "http://example.org/parent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected transitive inference grandchild -> parent")
	}
}

func TestEngine_inferBroaderNarrower_LoopDetection(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSBroader, Object: "http://example.org/b", Graph: "test"},
		{Subject: "http://example.org/b", Predicate: SKOSBroader, Object: "http://example.org/a", Graph: "test"},
	}

	result := engine.inferBroaderNarrower(triples, false)
	if len(result) == 0 {
		t.Error("expected some inferences despite loop")
	}
}

func TestEngine_inferRelated(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSRelated, Object: "http://example.org/b"},
	}

	result := engine.inferRelated(triples)
	if len(result) != 1 {
		t.Fatalf("expected 1 inferred triple, got %d", len(result))
	}

	if result[0].Subject != "http://example.org/b" || result[0].Object != "http://example.org/a" {
		t.Errorf("expected symmetric relation, got %s -> %s", result[0].Subject, result[0].Object)
	}
}

func TestEngine_inferRelated_Multiple(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSRelated, Object: "http://example.org/b"},
		{Subject: "http://example.org/a", Predicate: SKOSRelated, Object: "http://example.org/c"},
	}

	result := engine.inferRelated(triples)
	if len(result) != 2 {
		t.Fatalf("expected 2 inferred triples, got %d", len(result))
	}
}

func TestEngine_inferExactMatch(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSExactMatch, Object: "http://example.org/b"},
		{Subject: "http://example.org/a", Predicate: SKOSExactMatch, Object: "http://example.org/c"},
	}

	result := engine.inferExactMatch(triples)
	if len(result) != 4 {
		t.Fatalf("expected 4 inferred triples (transitive), got %d", len(result))
	}

	objects := make(map[string]bool)
	for _, t := range result {
		objects[t.Object] = true
	}
	if !objects["http://example.org/b"] || !objects["http://example.org/c"] {
		t.Error("expected b and c in inferred triples")
	}
}

func TestEngine_inferExactMatch_Single(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSExactMatch, Object: "http://example.org/b"},
	}

	result := engine.inferExactMatch(triples)
	if len(result) != 0 {
		t.Errorf("expected 0 inferred triples for single match, got %d", len(result))
	}
}

func TestEngine_inferCloseMatch(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSCloseMatch, Object: "http://example.org/b"},
	}

	result := engine.inferCloseMatch(triples)
	if len(result) != 2 {
		t.Fatalf("expected 2 inferred triples, got %d", len(result))
	}

	if result[0].Predicate != SKOSExactMatch {
		t.Errorf("expected predicate %s, got %s", SKOSExactMatch, result[0].Predicate)
	}
}

func TestEngine_inferCloseMatch_Multiple(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	triples := []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSCloseMatch, Object: "http://example.org/b"},
		{Subject: "http://example.org/a", Predicate: SKOSCloseMatch, Object: "http://example.org/c"},
	}

	result := engine.inferCloseMatch(triples)
	if len(result) != 4 {
		t.Fatalf("expected 4 inferred triples, got %d", len(result))
	}
}

func TestEngine_addAuthorityTriples_AuthorityMatch(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)
	engine.SetSKOSOptions(SKOSOptions{Inference: SKOSInferenceNone, AuthorityMode: AuthorityMatchAuthority})

	triples := []types.Triple{
		{Subject: "http://example.com/resource1", Predicate: "http://p1", Object: "http://example.com/resource2"},
		{Subject: "http://example.com/resource3", Predicate: "http://p2", Object: "http://example.com/resource4"},
	}

	result := engine.addAuthorityTriples(triples)

	found := 0
	for _, t := range result[2:] {
		if t.Predicate == SKOSExactMatch {
			found++
		}
	}
	if found == 0 {
		t.Error("expected authority-based exactMatch triples")
	}
}

func TestEngine_addAuthorityTriples_NoAuthorityMatch(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)
	engine.SetSKOSOptions(SKOSOptions{Inference: SKOSInferenceNone, AuthorityMode: AuthorityMatchNone})

	triples := []types.Triple{
		{Subject: "http://example.com/resource1", Predicate: "http://p1", Object: "http://example.com/resource2"},
	}

	result := engine.addAuthorityTriples(triples)
	if len(result) != 1 {
		t.Errorf("expected 1 triple, got %d", len(result))
	}
}

func TestEngine_addAuthorityTriples_DifferentAuthorities(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)
	engine.SetSKOSOptions(SKOSOptions{Inference: SKOSInferenceNone, AuthorityMode: AuthorityMatchAuthority})

	triples := []types.Triple{
		{Subject: "http://example.com/resource1", Predicate: "http://p1", Object: "http://other.com/resource1"},
	}

	result := engine.addAuthorityTriples(triples)
	if len(result) != 1 {
		t.Errorf("expected 1 triple (no cross-authority matching), got %d", len(result))
	}
}

func TestEngine_inferSKOSTriples_AllInferences(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)
	engine.SetSKOSOptions(SKOSOptions{Inference: SKOSInferenceAll, AuthorityMode: AuthorityMatchNone})

	triples := []types.Triple{
		{Subject: "http://example.org/child", Predicate: SKOSBroader, Object: "http://example.org/parent", Graph: "test"},
		{Subject: "http://example.org/a", Predicate: SKOSRelated, Object: "http://example.org/b"},
		{Subject: "http://example.org/x", Predicate: SKOSExactMatch, Object: "http://example.org/y"},
		{Subject: "http://example.org/p", Predicate: SKOSCloseMatch, Object: "http://example.org/q"},
	}

	result := engine.inferSKOSTriples(triples)
	if len(result) <= len(triples) {
		t.Errorf("expected more triples with all inference enabled, got %d", len(result))
	}
}

func TestIsHTTPURI(t *testing.T) {
	tests := []struct {
		uri      string
		expected bool
	}{
		{"http://example.org/test", true},
		{"https://example.org/test", true},
		{"ftp://example.org/test", false},
		{"urn:isbn:0451450523", false},
		{"", false},
		{"http:", false},
		{"httpexample.org", false},
	}

	for _, tt := range tests {
		result := isHTTPURI(tt.uri)
		if result != tt.expected {
			t.Errorf("isHTTPURI(%q) = %v, expected %v", tt.uri, result, tt.expected)
		}
	}
}

func TestGetAuthority(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"http://example.org/test", "example.org"},
		{"https://www.example.org:8080/path", "www.example.org:8080"},
		{"http://localhost:8080/data", "localhost:8080"},
		{"", ""},
		{"not-a-uri", ""},
	}

	for _, tt := range tests {
		result := getAuthority(tt.uri)
		if result != tt.expected {
			t.Errorf("getAuthority(%q) = %q, expected %q", tt.uri, result, tt.expected)
		}
	}
}

func TestEngine_computeTransitiveBroader(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	subjectMap := map[string][]string{
		"http://example.org/child":  {"http://example.org/parent"},
		"http://example.org/parent": {"http://example.org/grandparent"},
	}

	result := engine.computeTransitiveBroader(subjectMap, SKOSBroader, SKOSNarrower, "graph")

	found := false
	for _, t := range result {
		if t.Subject == "http://example.org/child" && t.Object == "http://example.org/grandparent" && t.Predicate == SKOSNarrower {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected transitive inference child -> grandparent")
	}
}

func TestEngine_inferSKOSTriples_WithAuthority(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)
	engine.SetSKOSOptions(SKOSOptions{Inference: SKOSInferenceNone, AuthorityMode: AuthorityMatchAuthority})

	triples := []types.Triple{
		{Subject: "http://example.com/a", Predicate: "http://p", Object: "http://example.com/b"},
		{Subject: "http://example.com/c", Predicate: "http://p", Object: "http://example.com/d"},
	}

	result := engine.inferSKOSTriples(triples)
	if len(result) > len(triples) {
		t.Logf("Authority inference added %d triples", len(result)-len(triples))
	}
}

func TestEngine_Execute_WithSKOSInference(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.org/child", Predicate: SKOSBroader, Object: "http://example.org/parent"},
	})

	engine := NewEngine(s, WithSKOSInference(SKOSInferenceBroader))

	result, err := engine.Execute(`
		PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
		SELECT ?child ?parent WHERE {
			?child skos:narrower ?parent .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) == 0 {
		t.Error("expected inferred narrower triple to match")
	}
}

func TestEngine_Execute_WithExactMatchInference(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSExactMatch, Object: "http://example.org/b"},
		{Subject: "http://example.org/b", Predicate: SKOSExactMatch, Object: "http://example.org/c"},
	})

	engine := NewEngine(s, WithSKOSInference(SKOSInferenceExactMatch))

	result, err := engine.Execute(`
		PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
		SELECT ?s ?o WHERE {
			?s skos:exactMatch ?o .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) == 0 {
		t.Error("expected transitive exactMatch triples")
	}
}

func TestEngine_Execute_WithRelatedInference(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSRelated, Object: "http://example.org/b"},
	})

	engine := NewEngine(s, WithSKOSInference(SKOSInferenceRelated))

	result, err := engine.Execute(`
		PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
		SELECT ?s ?o WHERE {
			?o skos:related ?s .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) == 0 {
		t.Error("expected symmetric related triple")
	}
}

func TestEngine_Execute_WithCloseMatchInference(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.org/a", Predicate: SKOSCloseMatch, Object: "http://example.org/b"},
	})

	engine := NewEngine(s, WithSKOSInference(SKOSInferenceCloseMatch))

	result, err := engine.Execute(`
		PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
		SELECT ?s ?o WHERE {
			?s skos:exactMatch ?o .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) == 0 {
		t.Error("expected closeMatch to infer exactMatch")
	}
}

func TestEngine_Execute_WithAuthorityMatch(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.com/1", Predicate: "http://p", Object: "http://example.com/2"},
	})

	engine := NewEngine(s, WithAuthorityMatch(AuthorityMatchAuthority))

	result, err := engine.Execute(`
		PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
		SELECT ?s ?o WHERE {
			?s skos:exactMatch ?o .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Authority match results: %d bindings", len(result.Bindings))
}

package validate

import (
	"context"
	"testing"

	"github.com/soypete/ontology-go/types"
)

func TestNewValidator(t *testing.T) {
	triples := []types.Triple{
		{Subject: "ex:concept1", Predicate: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", Object: "http://www.w3.org/2004/02/skos/core#Concept"},
		{Subject: "ex:concept1", Predicate: "http://www.w3.org/2004/02/skos/core#prefLabel", Object: "Test Concept@en"},
		{Subject: "ex:scheme1", Predicate: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", Object: "http://www.w3.org/2004/02/skos/core#ConceptScheme"},
	}

	v := NewValidator(triples)

	if len(v.concepts) != 1 {
		t.Errorf("expected 1 concept, got %d", len(v.concepts))
	}
	if !v.concepts["ex:concept1"] {
		t.Error("expected concept ex:concept1 to be in concepts map")
	}

	if len(v.schemes) != 1 {
		t.Errorf("expected 1 scheme, got %d", len(v.schemes))
	}
	if !v.schemes["ex:scheme1"] {
		t.Error("expected scheme ex:scheme1 to be in schemes map")
	}
}

func TestValidator_checkLabelingQuality(t *testing.T) {
	tests := []struct {
		name           string
		triples        []types.Triple
		wantIssueTypes []IssueType
	}{
		{
			name: "missing preferred label",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSNotation, Object: "001"},
			},
			wantIssueTypes: []IssueType{IssueMissingPrefLabel},
		},
		{
			name: "multiple preferred labels in same language",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Label 1@en"},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Label 2@en"},
				{Subject: "ex:concept1", Predicate: SKOSNotation, Object: "001"},
			},
			wantIssueTypes: []IssueType{IssueMultiplePrefLabels},
		},
		{
			name: "overlapping labels",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test@en"},
				{Subject: "ex:concept1", Predicate: SKOSAltLabel, Object: "Test@en"},
				{Subject: "ex:concept1", Predicate: SKOSNotation, Object: "001"},
			},
			wantIssueTypes: []IssueType{IssueOverlappingLabels},
		},
		{
			name: "valid concept with notation",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test Concept@en"},
				{Subject: "ex:concept1", Predicate: SKOSNotation, Object: "001"},
			},
			wantIssueTypes: []IssueType{},
		},
		{
			name: "missing notation - info level",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test Concept@en"},
			},
			wantIssueTypes: []IssueType{IssueMissingNotation},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.triples)
			var issues []Issue
			v.checkLabelingQuality(&issues)

			found := make(map[IssueType]bool)
			for _, issue := range issues {
				found[issue.Type] = true
			}

			for _, want := range tt.wantIssueTypes {
				if !found[want] {
					t.Errorf("expected issue type %v, but it was not found", want)
				}
			}

			if len(issues) != len(tt.wantIssueTypes) {
				t.Errorf("expected %d issues, got %d", len(tt.wantIssueTypes), len(issues))
			}
		})
	}
}

func TestValidator_checkStructuralQuality(t *testing.T) {
	tests := []struct {
		name           string
		triples        []types.Triple
		wantIssueTypes []IssueType
	}{
		{
			name: "orphan concept",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test@en"},
			},
			wantIssueTypes: []IssueType{IssueOrphanConcept},
		},
		{
			name: "concept in scheme",
			triples: []types.Triple{
				{Subject: "ex:scheme1", Predicate: RDFType, Object: SKOSConceptScheme},
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test@en"},
				{Subject: "ex:concept1", Predicate: SKOSInScheme, Object: "ex:scheme1"},
			},
			wantIssueTypes: []IssueType{},
		},
		{
			name: "scheme without top concept",
			triples: []types.Triple{
				{Subject: "ex:scheme1", Predicate: RDFType, Object: SKOSConceptScheme},
			},
			wantIssueTypes: []IssueType{IssueMissingTopConcept},
		},
		{
			name: "scheme with top concept",
			triples: []types.Triple{
				{Subject: "ex:scheme1", Predicate: RDFType, Object: SKOSConceptScheme},
				{Subject: "ex:scheme1", Predicate: SKOSHasTopConcept, Object: "ex:concept1"},
			},
			wantIssueTypes: []IssueType{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.triples)
			var issues []Issue
			v.checkStructuralQuality(&issues)

			found := make(map[IssueType]bool)
			for _, issue := range issues {
				found[issue.Type] = true
			}

			for _, want := range tt.wantIssueTypes {
				if !found[want] {
					t.Errorf("expected issue type %v, but it was not found", want)
				}
			}
		})
	}
}

func TestValidator_checkCircularBroader(t *testing.T) {
	tests := []struct {
		name           string
		triples        []types.Triple
		wantIssueTypes []IssueType
	}{
		{
			name: "direct circular relation",
			triples: []types.Triple{
				{Subject: "ex:a", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:a", Predicate: SKOSPrefLabel, Object: "A@en"},
				{Subject: "ex:a", Predicate: SKOSBroader, Object: "ex:b"},
				{Subject: "ex:b", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:b", Predicate: SKOSPrefLabel, Object: "B@en"},
				{Subject: "ex:b", Predicate: SKOSBroader, Object: "ex:a"},
			},
			wantIssueTypes: []IssueType{IssueCircularBroader, IssueCircularBroader},
		},
		{
			name: "no circular relation",
			triples: []types.Triple{
				{Subject: "ex:a", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:a", Predicate: SKOSPrefLabel, Object: "A@en"},
				{Subject: "ex:a", Predicate: SKOSBroader, Object: "ex:b"},
				{Subject: "ex:b", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:b", Predicate: SKOSPrefLabel, Object: "B@en"},
			},
			wantIssueTypes: []IssueType{},
		},
		{
			name: "transitive circular relation",
			triples: []types.Triple{
				{Subject: "ex:a", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:a", Predicate: SKOSPrefLabel, Object: "A@en"},
				{Subject: "ex:a", Predicate: SKOSBroader, Object: "ex:b"},
				{Subject: "ex:b", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:b", Predicate: SKOSPrefLabel, Object: "B@en"},
				{Subject: "ex:b", Predicate: SKOSBroader, Object: "ex:c"},
				{Subject: "ex:c", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:c", Predicate: SKOSPrefLabel, Object: "C@en"},
				{Subject: "ex:c", Predicate: SKOSBroader, Object: "ex:a"},
			},
			wantIssueTypes: []IssueType{IssueCircularBroader, IssueCircularBroader, IssueCircularBroader},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.triples)
			var issues []Issue
			v.checkCircularBroader(&issues)

			found := make(map[IssueType]int)
			for _, issue := range issues {
				found[issue.Type]++
			}

			for _, want := range tt.wantIssueTypes {
				if found[want] <= 0 {
					t.Errorf("expected issue type %v, but it was not found", want)
				}
				found[want]--
			}
		})
	}
}

func TestValidator_checkConsistencyQuality(t *testing.T) {
	tests := []struct {
		name           string
		triples        []types.Triple
		wantIssueTypes []IssueType
	}{
		{
			name: "broken match link",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test@en"},
				{Subject: "ex:concept1", Predicate: SKOSExactMatch, Object: "ex:missing"},
			},
			wantIssueTypes: []IssueType{IssueBrokenLink},
		},
		{
			name: "valid match link",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test@en"},
				{Subject: "ex:concept1", Predicate: SKOSExactMatch, Object: "ex:concept2"},
				{Subject: "ex:concept2", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept2", Predicate: SKOSPrefLabel, Object: "Test 2@en"},
			},
			wantIssueTypes: []IssueType{},
		},
		{
			name: "invalid concept scheme reference",
			triples: []types.Triple{
				{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
				{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test@en"},
				{Subject: "ex:concept1", Predicate: SKOSInScheme, Object: "ex:invalidScheme"},
			},
			wantIssueTypes: []IssueType{IssueInvalidScheme},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.triples)
			var issues []Issue
			v.checkConsistencyQuality(&issues)

			found := make(map[IssueType]bool)
			for _, issue := range issues {
				found[issue.Type] = true
			}

			for _, want := range tt.wantIssueTypes {
				if !found[want] {
					t.Errorf("expected issue type %v, but it was not found", want)
				}
			}
		})
	}
}

func TestValidator_Validate(t *testing.T) {
	triples := []types.Triple{
		{Subject: "ex:scheme1", Predicate: RDFType, Object: SKOSConceptScheme},
		{Subject: "ex:concept1", Predicate: RDFType, Object: SKOSConcept},
		{Subject: "ex:concept1", Predicate: SKOSPrefLabel, Object: "Test@en"},
		{Subject: "ex:concept1", Predicate: SKOSInScheme, Object: "ex:scheme1"},
		{Subject: "ex:concept2", Predicate: RDFType, Object: SKOSConcept},
	}

	v := NewValidator(triples)
	ctx := context.Background()
	report, err := v.Validate(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalTriples != 5 {
		t.Errorf("expected 5 total triples, got %d", report.TotalTriples)
	}

	if report.TotalConcepts != 2 {
		t.Errorf("expected 2 total concepts, got %d", report.TotalConcepts)
	}

	if report.TotalSchemes != 1 {
		t.Errorf("expected 1 total scheme, got %d", report.TotalSchemes)
	}

	if len(report.Issues) == 0 {
		t.Error("expected some validation issues")
	}
}

func TestExtractLang(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test@en", "en"},
		{"Test@de", "de"},
		{"Test@fr", "fr"},
		{"Test", ""},
	}

	for _, tt := range tests {
		result := extractLang(tt.input)
		if result != tt.expected {
			t.Errorf("extractLang(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test@en", "test"},
		{"  Test  @de", "test"},
		{"Test@en", "test"},
		{"HELLO@en", "hello"},
	}

	for _, tt := range tests {
		result := normalizeLabel(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeLabel(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`"test"`, true},
		{`"test"@en`, true},
		{`"123"`, true},
		{"ex:Concept", false},
		{"http://example.org/test", false},
		{"test", false},
	}

	for _, tt := range tests {
		result := isLiteral(tt.input)
		if result != tt.expected {
			t.Errorf("isLiteral(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIssueSeverityOrder(t *testing.T) {
	triples := []types.Triple{
		{Subject: "ex:a", Predicate: RDFType, Object: SKOSConcept},
		{Subject: "ex:a", Predicate: SKOSPrefLabel, Object: "A@en"},
		{Subject: "ex:a", Predicate: SKOSBroader, Object: "ex:b"},
		{Subject: "ex:b", Predicate: RDFType, Object: SKOSConcept},
		{Subject: "ex:b", Predicate: SKOSPrefLabel, Object: "B@en"},
		{Subject: "ex:b", Predicate: SKOSBroader, Object: "ex:a"},
	}

	v := NewValidator(triples)
	ctx := context.Background()
	report, err := v.Validate(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Issues) == 0 {
		t.Fatal("expected issues")
	}

	if report.Issues[0].Severity != SeverityError {
		t.Errorf("first issue should have SeverityError, got %v", report.Issues[0].Severity)
	}
}

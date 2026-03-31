// Package validate provides SKOS quality validation functionality.
//
// This package offers tools to validate SKOS (Simple Knowledge Organization
// System) RDF vocabularies against quality criteria inspired by qSKOS.
// It supports both Turtle (.ttl) and RDF/XML (.rdf) file formats with
// automatic format detection.
//
// Example:
//
//	reader, err := validate.NewReader("vocabulary.ttl")
//	triples, err := reader.Parse()
//	validator := validate.NewValidator(triples)
//	report, err := validator.Validate(ctx)
package validate

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/soypete/ontology-go/types"
)

const (
	RDFType = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"

	SKOSNS = "http://www.w3.org/2004/02/skos/core#"

	SKOSConcept       = SKOSNS + "Concept"
	SKOSConceptScheme = SKOSNS + "ConceptScheme"
	SKOSCollection    = SKOSNS + "Collection"

	SKOSPrefLabel = SKOSNS + "prefLabel"
	SKOSAltLabel  = SKOSNS + "altLabel"
	SKOSHidden    = SKOSNS + "hiddenLabel"
	SKOSNotation  = SKOSNS + "notation"

	SKOSInScheme      = SKOSNS + "inScheme"
	SKOSHasTopConcept = SKOSNS + "hasTopConcept"
	SKOSTopConceptOf  = SKOSNS + "topConceptOf"

	SKOSBroader  = SKOSNS + "broader"
	SKOSNarrower = SKOSNS + "narrower"
	SKOSRelated  = SKOSNS + "related"

	SKOSExactMatch = SKOSNS + "exactMatch"
	SKOSCloseMatch = SKOSNS + "closeMatch"
)

type IssueSeverity string

const (
	SeverityWarning IssueSeverity = "warning"
	SeverityError   IssueSeverity = "error"
	SeverityInfo    IssueSeverity = "info"
)

type IssueType string

const (
	IssueMissingPrefLabel      IssueType = "missing_preferred_label"
	IssueMultiplePrefLabels    IssueType = "multiple_preferred_labels"
	IssueOverlappingLabels     IssueType = "overlapping_labels"
	IssueMissingNotation       IssueType = "missing_notation"
	IssueOrphanConcept         IssueType = "orphan_concept"
	IssueMissingTopConcept     IssueType = "missing_top_concept"
	IssueCircularBroader       IssueType = "circular_broader_relation"
	IssueInconsistentHierarchy IssueType = "inconsistent_hierarchy"
	IssueBrokenLink            IssueType = "broken_link"
	IssueInvalidScheme         IssueType = "invalid_concept_scheme"
)

type Issue struct {
	Type       IssueType      `json:"type"`
	Severity   IssueSeverity  `json:"severity"`
	Subject    string         `json:"subject"`
	Message    string         `json:"message"`
	Context    map[string]any `json:"context,omitempty"`
	Confidence float64        `json:"confidence"`
}

type ValidationReport struct {
	TotalTriples  int            `json:"total_triples"`
	TotalConcepts int            `json:"total_concepts"`
	TotalSchemes  int            `json:"total_schemes"`
	Issues        []Issue        `json:"issues"`
	Stats         map[string]int `json:"stats"`
}

type Validator struct {
	triples     []types.Triple
	concepts    map[string]bool
	schemes     map[string]bool
	labels      map[string]map[string][]string
	notations   map[string]map[string]bool
	broaderMap  map[string][]string
	narrowerMap map[string][]string
	schemeMap   map[string][]string
	topConcepts map[string][]string
	relatedMap  map[string][]string
	matchMap    map[string][]string
}

func NewValidator(triples []types.Triple) *Validator {
	v := &Validator{
		triples:     triples,
		concepts:    make(map[string]bool),
		schemes:     make(map[string]bool),
		labels:      make(map[string]map[string][]string),
		notations:   make(map[string]map[string]bool),
		broaderMap:  make(map[string][]string),
		narrowerMap: make(map[string][]string),
		schemeMap:   make(map[string][]string),
		topConcepts: make(map[string][]string),
		relatedMap:  make(map[string][]string),
		matchMap:    make(map[string][]string),
	}
	v.buildIndices()
	return v
}

func (v *Validator) buildIndices() {
	for _, t := range v.triples {
		if t.Predicate == RDFType {
			if t.Object == SKOSConcept {
				v.concepts[t.Subject] = true
			} else if t.Object == SKOSConceptScheme {
				v.schemes[t.Subject] = true
			} else if t.Object == SKOSCollection {
				v.concepts[t.Subject] = true
			}
		}

		if t.Predicate == SKOSPrefLabel || t.Predicate == SKOSAltLabel || t.Predicate == SKOSHidden {
			lang := extractLang(t.Object)
			if v.labels[t.Subject] == nil {
				v.labels[t.Subject] = make(map[string][]string)
			}
			v.labels[t.Subject][t.Predicate] = append(v.labels[t.Subject][t.Predicate], t.Object)
			_ = lang
		}

		if t.Predicate == SKOSNotation {
			if v.notations[t.Subject] == nil {
				v.notations[t.Subject] = make(map[string]bool)
			}
			v.notations[t.Subject][t.Object] = true
		}

		if t.Predicate == SKOSBroader {
			v.broaderMap[t.Subject] = append(v.broaderMap[t.Subject], t.Object)
		}

		if t.Predicate == SKOSNarrower {
			v.narrowerMap[t.Subject] = append(v.narrowerMap[t.Subject], t.Object)
		}

		if t.Predicate == SKOSInScheme {
			v.schemeMap[t.Subject] = append(v.schemeMap[t.Subject], t.Object)
		}

		if t.Predicate == SKOSHasTopConcept {
			v.topConcepts[t.Subject] = append(v.topConcepts[t.Subject], t.Object)
		}

		if t.Predicate == SKOSRelated {
			v.relatedMap[t.Subject] = append(v.relatedMap[t.Subject], t.Object)
		}

		if t.Predicate == SKOSExactMatch || t.Predicate == SKOSCloseMatch {
			v.matchMap[t.Subject] = append(v.matchMap[t.Subject], t.Object)
		}
	}
}

func extractLang(literal string) string {
	if idx := strings.LastIndex(literal, "@"); idx > 0 {
		return literal[idx+1:]
	}
	return ""
}

func (v *Validator) Validate(ctx context.Context) (*ValidationReport, error) {
	report := &ValidationReport{
		TotalTriples:  len(v.triples),
		TotalConcepts: len(v.concepts),
		TotalSchemes:  len(v.schemes),
		Issues:        []Issue{},
		Stats:         make(map[string]int),
	}

	v.checkLabelingQuality(&report.Issues)
	v.checkStructuralQuality(&report.Issues)
	v.checkConsistencyQuality(&report.Issues)

	for _, issue := range report.Issues {
		report.Stats[string(issue.Type)]++
	}

	sort.Slice(report.Issues, func(i, j int) bool {
		if report.Issues[i].Severity != report.Issues[j].Severity {
			return report.Issues[i].Severity == SeverityError
		}
		return report.Issues[i].Type < report.Issues[j].Type
	})

	return report, nil
}

func (v *Validator) checkLabelingQuality(issues *[]Issue) {
	for concept := range v.concepts {
		prefLabels := v.labels[concept][SKOSPrefLabel]
		altLabels := v.labels[concept][SKOSAltLabel]

		if len(prefLabels) == 0 {
			*issues = append(*issues, Issue{
				Type:       IssueMissingPrefLabel,
				Severity:   SeverityWarning,
				Subject:    concept,
				Message:    "Concept lacks a preferred label",
				Confidence: 1.0,
			})
		}

		langCount := make(map[string]int)
		for _, label := range prefLabels {
			lang := extractLang(label)
			langCount[lang]++
			if langCount[lang] > 1 {
				*issues = append(*issues, Issue{
					Type:       IssueMultiplePrefLabels,
					Severity:   SeverityWarning,
					Subject:    concept,
					Message:    fmt.Sprintf("Concept has multiple preferred labels in language '%s'", lang),
					Context:    map[string]any{"language": lang, "labels": prefLabels},
					Confidence: 1.0,
				})
				break
			}
		}

		prefLabelSet := make(map[string]bool)
		for _, l := range prefLabels {
			prefLabelSet[normalizeLabel(l)] = true
		}
		for _, l := range altLabels {
			if prefLabelSet[normalizeLabel(l)] {
				*issues = append(*issues, Issue{
					Type:       IssueOverlappingLabels,
					Severity:   SeverityWarning,
					Subject:    concept,
					Message:    "Concept has overlapping preferred and alternative labels",
					Context:    map[string]any{"prefLabel": prefLabels, "altLabel": l},
					Confidence: 0.9,
				})
				break
			}
		}

		_, hasNotation := v.notations[concept]
		if !hasNotation {
			*issues = append(*issues, Issue{
				Type:       IssueMissingNotation,
				Severity:   SeverityInfo,
				Subject:    concept,
				Message:    "Concept lacks a notation",
				Confidence: 0.5,
			})
		}
	}
}

func normalizeLabel(label string) string {
	re := regexp.MustCompile(`@[^@]+$`)
	return strings.ToLower(strings.TrimSpace(re.ReplaceAllString(label, "")))
}

func (v *Validator) checkStructuralQuality(issues *[]Issue) {
	for concept := range v.concepts {
		schemes, inScheme := v.schemeMap[concept]
		if !inScheme || len(schemes) == 0 {
			*issues = append(*issues, Issue{
				Type:       IssueOrphanConcept,
				Severity:   SeverityWarning,
				Subject:    concept,
				Message:    "Concept is not part of any concept scheme",
				Confidence: 0.8,
			})
		}
	}

	for scheme := range v.schemes {
		topConcepts, hasTop := v.topConcepts[scheme]
		if !hasTop || len(topConcepts) == 0 {
			*issues = append(*issues, Issue{
				Type:       IssueMissingTopConcept,
				Severity:   SeverityInfo,
				Subject:    scheme,
				Message:    "Concept scheme has no top concepts",
				Confidence: 0.7,
			})
		}
	}

	v.checkCircularBroader(issues)
	v.checkInconsistentHierarchy(issues)
}

func (v *Validator) checkCircularBroader(issues *[]Issue) {
	for concept := range v.concepts {
		path := make(map[string]bool)
		visited := make(map[string]bool)
		v.detectCycle(concept, path, visited, issues)
	}
}

func (v *Validator) detectCycle(subject string, path map[string]bool, visited map[string]bool, issues *[]Issue) {
	if path[subject] {
		*issues = append(*issues, Issue{
			Type:       IssueCircularBroader,
			Severity:   SeverityError,
			Subject:    subject,
			Message:    "Concept has a circular broader relation",
			Confidence: 1.0,
		})
		return
	}

	if visited[subject] {
		return
	}

	path[subject] = true
	visited[subject] = true

	for _, broader := range v.broaderMap[subject] {
		v.detectCycle(broader, path, visited, issues)
	}

	delete(path, subject)
}

func (v *Validator) checkInconsistentHierarchy(issues *[]Issue) {
	for subject, narrowers := range v.narrowerMap {
		for _, narrower := range narrowers {
			if v.hasBroaderRelation(narrower, subject) {
				*issues = append(*issues, Issue{
					Type:       IssueInconsistentHierarchy,
					Severity:   SeverityError,
					Subject:    subject,
					Message:    fmt.Sprintf("Inconsistent hierarchy: %s is broader than %s but also narrower", subject, narrower),
					Context:    map[string]any{"narrower": narrower},
					Confidence: 1.0,
				})
			}
		}
	}
}

func (v *Validator) hasBroaderRelation(subject, potentialBroader string) bool {
	visited := make(map[string]bool)
	return v.checkBroaderPath(subject, potentialBroader, visited)
}

func (v *Validator) checkBroaderPath(subject, target string, visited map[string]bool) bool {
	if visited[subject] {
		return false
	}
	visited[subject] = true

	for _, broader := range v.broaderMap[subject] {
		if broader == target {
			return true
		}
		if v.checkBroaderPath(broader, target, visited) {
			return true
		}
	}
	return false
}

func isLiteral(s string) bool {
	if strings.HasPrefix(s, `"`) {
		return true
	}
	if isExternalURI(s) {
		return false
	}
	return strings.Contains(s, "@")
}

func isExternalURI(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "urn:")
}

func (v *Validator) checkConsistencyQuality(issues *[]Issue) {
	definedConcepts := make(map[string]bool)
	for t := range v.concepts {
		definedConcepts[t] = true
	}

	for subject, matches := range v.matchMap {
		for _, match := range matches {
			if !definedConcepts[match] && !isExternalURI(match) {
				*issues = append(*issues, Issue{
					Type:       IssueBrokenLink,
					Severity:   SeverityWarning,
					Subject:    subject,
					Message:    fmt.Sprintf("Broken match link: target '%s' does not exist in the vocabulary", match),
					Confidence: 0.6,
				})
			}
		}
	}

	for concept, schemes := range v.schemeMap {
		for _, scheme := range schemes {
			if !v.schemes[scheme] {
				*issues = append(*issues, Issue{
					Type:       IssueInvalidScheme,
					Severity:   SeverityError,
					Subject:    concept,
					Message:    fmt.Sprintf("Concept references invalid concept scheme: %s", scheme),
					Context:    map[string]any{"scheme": scheme},
					Confidence: 1.0,
				})
			}
		}
	}
}

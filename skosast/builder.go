// Package skosast provides an AST representation of SKOS thesaurus hierarchies.
//
// This package builds a semantic tree from Turtle AST source, extracting
// SKOS concepts, schemes, and their relationships (broader, narrower, related,
// exactMatch, closeMatch) for reasoning and inference.
//
// The Hierarchy type provides methods for:
//   - Transitive closure of broader/narrower relations
//   - Cycle detection in the concept hierarchy
//   - Inconsistency detection (concept both broader and narrower than same target)
//
// Example:
//
//	// Parse Turtle file
//	parser := ttlast.NewParser()
//	doc, err := parser.ParseFile("thesaurus.ttl")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Build SKOS hierarchy from AST
//	hierarchy, err := skosast.BuildFromDocument(doc)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get all transitive broader relations for a concept
//	allBroader := hierarchy.GetAllBroaderTransitive("http://example.org/onlineCourse")
//
//	// Check for cycles
//	if hasCycle, msg := hierarchy.HasCycle(); hasCycle {
//	    fmt.Println("Cycle detected:", msg)
//	}
//
//	// Find inconsistencies
//	for _, msg := range hierarchy.FindInconsistencies {
//	    fmt.Println("Inconsistency:", msg)
//	}
package skosast

import (
	"fmt"
	"strings"

	"github.com/soypete/ontology-go/ttlast"
)

func BuildFromDocument(doc *ttlast.Document) (*Hierarchy, error) {
	h := NewHierarchy()

	prefixes := extractPrefixes(doc)

	for _, stmt := range doc.Statements {
		triple := stmt.Triple

		subj := resolveTerm(triple.Subject, prefixes)
		pred := resolveTerm(triple.Predicate, prefixes)
		obj := resolveTerm(triple.Object, prefixes)

		if subj == "" || pred == "" || obj == "" {
			continue
		}

		if pred == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
			switch obj {
			case SKOSConcept:
				h.AddConcept(subj)
			case SKOSConceptScheme:
				h.AddScheme(subj)
			case SKOSCollection:
				h.AddCollection(subj)
			case SKOSOrderedCollection:
				h.AddOrderedCollection(subj)
			}
			continue
		}

		switch pred {
		case SKOSPrefLabel:
			litVal := extractLiteralValue(triple.Object)
			if c, ok := h.Concepts[subj]; ok {
				c.PrefLabel = litVal
				c.BaseNode.label = litVal
			} else if s, ok := h.Schemes[subj]; ok {
				s.BaseNode.label = litVal
			}

		case SKOSAltLabel:
			if c := h.Concepts[subj]; c != nil {
				c.AltLabels = append(c.AltLabels, extractLiteralValue(triple.Object))
			}

		case SKOSHiddenLabel:
			if c := h.Concepts[subj]; c != nil {
				c.HiddenLabels = append(c.HiddenLabels, extractLiteralValue(triple.Object))
			}

		case SKOSNotation:
			if c := h.Concepts[subj]; c != nil {
				c.Notation = extractLiteralValue(triple.Object)
			}

		case SKOSBroader:
			if c := h.AddConcept(subj); c != nil {
				c.Broader = append(c.Broader, obj)
				if narrowerC := h.AddConcept(obj); narrowerC != nil {
					narrowerC.Narrower = append(narrowerC.Narrower, subj)
				}
			}

		case SKOSNarrower:
			if c := h.AddConcept(subj); c != nil {
				c.Narrower = append(c.Narrower, obj)
				if broaderC := h.AddConcept(obj); broaderC != nil {
					broaderC.Broader = append(broaderC.Broader, subj)
				}
			}

		case SKOSRelated:
			if c := h.AddConcept(subj); c != nil {
				c.Related = append(c.Related, obj)
			}
			if c := h.AddConcept(obj); c != nil {
				c.Related = append(c.Related, subj)
			}

		case SKOSExactMatch:
			if c := h.AddConcept(subj); c != nil {
				c.ExactMatch = append(c.ExactMatch, obj)
			}

		case SKOSCloseMatch:
			if c := h.AddConcept(subj); c != nil {
				c.CloseMatch = append(c.CloseMatch, obj)
			}

		case SKOSInScheme:
			if c := h.Concepts[subj]; c != nil {
				c.InScheme = append(c.InScheme, obj)
			}

		case SKOSHasTopConcept:
			if s := h.Schemes[subj]; s != nil {
				s.HasTopConcept = append(s.HasTopConcept, obj)
				h.TopConcepts = append(h.TopConcepts, obj)
			}

		case SKOSTopConceptOf:
			if c := h.Concepts[subj]; c != nil {
				c.TopConceptOf = append(c.TopConceptOf, obj)
			}

		case SKOSMember:
			if c := h.Collections[subj]; c != nil {
				c.Members = append(c.Members, obj)
			}

		case SKOSMemberList:
			if c := h.OrderedCollections[subj]; c != nil {
				c.MemberList = append(c.MemberList, obj)
			}
		}
	}

	return h, nil
}

func extractPrefixes(doc *ttlast.Document) map[string]string {
	prefixes := make(map[string]string)
	for _, p := range doc.Prefixes {
		prefixes[p.Prefix] = p.IRI.Value
	}
	return prefixes
}

func resolveTerm(term ttlast.Term, prefixes map[string]string) string {
	switch t := term.(type) {
	case *ttlast.IRI:
		return t.Value
	case *ttlast.PrefixedName:
		if base, ok := prefixes[t.Prefix]; ok {
			return base + t.Local
		}
		return t.Prefix + ":" + t.Local
	case *ttlast.BlankNode:
		return "_:" + t.Label
	case *ttlast.Literal:
		return t.Value
	default:
		return ""
	}
}

func extractLiteralValue(term ttlast.Term) string {
	if lit, ok := term.(*ttlast.Literal); ok {
		return lit.Value
	}
	return ""
}

func (h *Hierarchy) GetAllBroaderTransitive(iri string) []string {
	visited := make(map[string]bool)
	var result []string

	var dfs func(current string)
	dfs = func(current string) {
		if visited[current] {
			return
		}
		visited[current] = true

		if c := h.Concepts[current]; c != nil {
			for _, b := range c.Broader {
				if !visited[b] {
					result = append(result, b)
					dfs(b)
				}
			}
		}
	}

	dfs(iri)
	return result
}

func (h *Hierarchy) GetAllNarrowerTransitive(iri string) []string {
	visited := make(map[string]bool)
	var result []string

	var dfs func(current string)
	dfs = func(current string) {
		if visited[current] {
			return
		}
		visited[current] = true

		if c := h.Concepts[current]; c != nil {
			for _, n := range c.Narrower {
				if !visited[n] {
					result = append(result, n)
					dfs(n)
				}
			}
		}
	}

	dfs(iri)
	return result
}

func (h *Hierarchy) HasCycle() (bool, string) {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(iri string, path []string) (bool, string)
	dfs = func(iri string, path []string) (bool, string) {
		visited[iri] = true
		recStack[iri] = true
		path = append(path, iri)

		if c := h.Concepts[iri]; c != nil {
			for _, broader := range c.Broader {
				if !visited[broader] {
					if found, cycle := dfs(broader, path); found {
						return found, cycle
					}
				} else if recStack[broader] {
					return true, fmt.Sprintf("Cycle: %v -> %s", path, broader)
				}
			}
		}

		recStack[iri] = false
		return false, ""
	}

	for iri := range h.Concepts {
		if !visited[iri] {
			if found, cycle := dfs(iri, nil); found {
				return true, cycle
			}
		}
	}

	return false, ""
}

func (h *Hierarchy) FindInconsistencies() []string {
	var inconsistencies []string

	for iri, c := range h.Concepts {
		broaderSet := make(map[string]bool)
		for _, b := range c.Broader {
			broaderSet[b] = true
		}

		for _, n := range c.Narrower {
			if broaderSet[n] {
				inconsistencies = append(inconsistencies,
					fmt.Sprintf("Concept %s is both broader and narrower than %s", iri, n))
			}
		}
	}

	if hasCycle, cycleMsg := h.HasCycle(); hasCycle {
		inconsistencies = append(inconsistencies, "Cycle detected: "+cycleMsg)
	}

	return inconsistencies
}

func (h *Hierarchy) GetConceptsByScheme(schemeIRI string) []*Concept {
	var result []*Concept
	for _, c := range h.Concepts {
		for _, s := range c.InScheme {
			if s == schemeIRI {
				result = append(result, c)
				break
			}
		}
	}
	return result
}

func (h *Hierarchy) GetTopConceptsByScheme(schemeIRI string) []*Concept {
	var result []*Concept
	for _, topIRI := range h.TopConcepts {
		if c := h.Concepts[topIRI]; c != nil {
			for _, s := range c.TopConceptOf {
				if s == schemeIRI {
					result = append(result, c)
					break
				}
			}
		}
	}
	return result
}

func termToString(term ttlast.Term, prefixes map[string]string) string {
	switch t := term.(type) {
	case *ttlast.IRI:
		return t.Value
	case *ttlast.PrefixedName:
		return t.Prefix + t.Local
	case *ttlast.BlankNode:
		return "_:" + t.Label
	case *ttlast.Literal:
		if t.Language != "" {
			return fmt.Sprintf("%q@%s", t.Value, t.Language)
		}
		if t.Datatype != "" {
			return fmt.Sprintf("%q^^%s", t.Value, t.Datatype)
		}
		return fmt.Sprintf("%q", t.Value)
	case *ttlast.Collection:
		var elems []string
		for _, e := range t.Elements {
			elems = append(elems, termToString(e, prefixes))
		}
		return "(" + strings.Join(elems, " ") + ")"
	default:
		return ""
	}
}

package reasoner

import (
	"fmt"
	"strings"

	"github.com/soypete/ontology-go/skosast"
	"github.com/soypete/ontology-go/ttlast"
)

type Rule interface {
	Name() string
	Apply(doc *ttlast.Document, hierarchy *skosast.Hierarchy, facts *FactSet) error
}

type TransitiveBroaderRule struct{}

func (r *TransitiveBroaderRule) Name() string {
	return "TransitiveBroader"
}

func (r *TransitiveBroaderRule) Apply(doc *ttlast.Document, hierarchy *skosast.Hierarchy, facts *FactSet) error {
	prefixes := extractPrefixes(doc)

	for _, stmt := range doc.Statements {
		triple := stmt.Triple
		pred := resolveTerm(triple.Predicate, prefixes)

		if pred == skosast.SKOSBroader {
			subj := resolveTerm(triple.Subject, prefixes)
			obj := resolveTerm(triple.Object, prefixes)

			facts.AddFact(Fact{
				Subject:    subj,
				Predicate:  skosast.SKOSBroader,
				Object:     obj,
				Derived:    false,
				Provenance: provenanceFromStatement(doc, &stmt),
			})

			allBroader := hierarchy.GetAllBroaderTransitive(obj)
			for _, b := range allBroader {
				facts.AddFact(Fact{
					Subject:    subj,
					Predicate:  skosast.SKOSBroaderTransitive,
					Object:     b,
					Derived:    true,
					Provenance: provenanceFromStatement(doc, &stmt),
				})
			}
		}
	}

	return nil
}

type TransitiveNarrowerRule struct{}

func (r *TransitiveNarrowerRule) Name() string {
	return "TransitiveNarrower"
}

func (r *TransitiveNarrowerRule) Apply(doc *ttlast.Document, hierarchy *skosast.Hierarchy, facts *FactSet) error {
	prefixes := extractPrefixes(doc)

	for _, stmt := range doc.Statements {
		triple := stmt.Triple
		pred := resolveTerm(triple.Predicate, prefixes)

		if pred == skosast.SKOSNarrower {
			subj := resolveTerm(triple.Subject, prefixes)
			obj := resolveTerm(triple.Object, prefixes)

			facts.AddFact(Fact{
				Subject:    subj,
				Predicate:  skosast.SKOSNarrower,
				Object:     obj,
				Derived:    false,
				Provenance: provenanceFromStatement(doc, &stmt),
			})

			allNarrower := hierarchy.GetAllNarrowerTransitive(obj)
			for _, n := range allNarrower {
				facts.AddFact(Fact{
					Subject:    subj,
					Predicate:  skosast.SKOSNarrowerTransitive,
					Object:     n,
					Derived:    true,
					Provenance: provenanceFromStatement(doc, &stmt),
				})
			}
		}
	}

	return nil
}

type SymmetricRelatedRule struct{}

func (r *SymmetricRelatedRule) Name() string {
	return "SymmetricRelated"
}

func (r *SymmetricRelatedRule) Apply(doc *ttlast.Document, hierarchy *skosast.Hierarchy, facts *FactSet) error {
	prefixes := extractPrefixes(doc)
	relatedPairs := make(map[string]map[string]bool)

	for _, stmt := range doc.Statements {
		triple := stmt.Triple
		pred := resolveTerm(triple.Predicate, prefixes)

		if pred == skosast.SKOSRelated {
			subj := resolveTerm(triple.Subject, prefixes)
			obj := resolveTerm(triple.Object, prefixes)

			facts.AddFact(Fact{
				Subject:    subj,
				Predicate:  skosast.SKOSRelated,
				Object:     obj,
				Derived:    false,
				Provenance: provenanceFromStatement(doc, &stmt),
			})

			if relatedPairs[subj] == nil {
				relatedPairs[subj] = make(map[string]bool)
			}
			relatedPairs[subj][obj] = true
		}
	}

	for subj, objects := range relatedPairs {
		for obj := range objects {
			if !relatedPairs[obj][subj] {
				facts.AddFact(Fact{
					Subject:   obj,
					Predicate: skosast.SKOSRelated,
					Object:    subj,
					Derived:   true,
				})
			}
		}
	}

	return nil
}

type TransitiveExactMatchRule struct{}

func (r *TransitiveExactMatchRule) Name() string {
	return "TransitiveExactMatch"
}

func (r *TransitiveExactMatchRule) Apply(doc *ttlast.Document, hierarchy *skosast.Hierarchy, facts *FactSet) error {
	prefixes := extractPrefixes(doc)
	matchMap := make(map[string][]string)
	visitedPairs := make(map[string]bool)

	for _, stmt := range doc.Statements {
		triple := stmt.Triple
		pred := resolveTerm(triple.Predicate, prefixes)

		if pred == skosast.SKOSExactMatch {
			subj := resolveTerm(triple.Subject, prefixes)
			obj := resolveTerm(triple.Object, prefixes)

			facts.AddFact(Fact{
				Subject:    subj,
				Predicate:  skosast.SKOSExactMatch,
				Object:     obj,
				Derived:    false,
				Provenance: provenanceFromStatement(doc, &stmt),
			})

			matchMap[subj] = append(matchMap[subj], obj)
		}
	}

	var transitiveInfer func(subject string, visited map[string]bool)
	transitiveInfer = func(subject string, visited map[string]bool) {
		if visited[subject] {
			return
		}
		visited[subject] = true

		for _, directMatch := range matchMap[subject] {
			for otherConcept := range matchMap {
				if otherConcept == subject || otherConcept == directMatch {
					continue
				}
				pairKey := directMatch + "|" + otherConcept
				if !visitedPairs[pairKey] {
					visitedPairs[pairKey] = true

					transitiveInfer(directMatch, visited)
				}
			}
		}
	}

	for subject := range matchMap {
		transitiveInfer(subject, make(map[string]bool))
	}

	for pairKey := range visitedPairs {
		parts := strings.Split(pairKey, "|")
		if len(parts) == 2 {
			facts.AddFact(Fact{
				Subject:   parts[0],
				Predicate: skosast.SKOSExactMatch,
				Object:    parts[1],
				Derived:   true,
			})
			facts.AddFact(Fact{
				Subject:   parts[1],
				Predicate: skosast.SKOSExactMatch,
				Object:    parts[0],
				Derived:   true,
			})
		}
	}

	return nil
}

type InconsistencyRule struct{}

func (r *InconsistencyRule) Name() string {
	return "InconsistencyDetector"
}

func (r *InconsistencyRule) Apply(doc *ttlast.Document, hierarchy *skosast.Hierarchy, facts *FactSet) error {
	prefixes := extractPrefixes(doc)

	for iri, c := range hierarchy.Concepts {
		broaderSet := make(map[string]bool)
		for _, b := range c.Broader {
			broaderSet[b] = true
		}

		for _, n := range c.Narrower {
			if broaderSet[n] {
				prov := findProvenanceForConcept(doc, iri, prefixes)
				facts.AddInconsistency(Inconsistency{
					Type:       "broaderAndNarrower",
					Message:    fmt.Sprintf("Concept %s is both broader and narrower than %s", iri, n),
					Provenance: prov,
				})
			}
		}
	}

	if hasCycle, cycleMsg := hierarchy.HasCycle(); hasCycle {
		facts.AddInconsistency(Inconsistency{
			Type:    "cycle",
			Message: cycleMsg,
		})
	}

	return nil
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

func provenanceFromStatement(doc *ttlast.Document, stmt *ttlast.Statement) *Provenance {
	if stmt == nil {
		return nil
	}

	line := 1
	column := 1
	pos := stmt.Pos()
	for i := 0; i < pos && i < len(doc.Input); i++ {
		if doc.Input[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}

	return &Provenance{
		SourceFile: "",
		Line:       line,
		Column:     column,
		StartPos:   stmt.Pos(),
		EndPos:     stmt.End(),
	}
}

func findProvenanceForConcept(doc *ttlast.Document, conceptIRI string, prefixes map[string]string) *Provenance {
	for _, stmt := range doc.Statements {
		subj := resolveTerm(stmt.Triple.Subject, prefixes)
		if subj == conceptIRI {
			return provenanceFromStatement(doc, &stmt)
		}
	}
	return nil
}

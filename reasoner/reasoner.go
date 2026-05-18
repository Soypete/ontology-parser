package reasoner

import (
	"context"
	"fmt"

	"github.com/soypete/ontology-go/skosast"
	"github.com/soypete/ontology-go/ttlast"
)

type Reasoner struct {
	doc        *ttlast.Document
	hierarchy  *skosast.Hierarchy
	factSet    *FactSet
	rules      []Rule
	sourceFile string
}

func New(doc *ttlast.Document, hierarchy *skosast.Hierarchy, sourceFile string) *Reasoner {
	return &Reasoner{
		doc:        doc,
		hierarchy:  hierarchy,
		factSet:    &FactSet{},
		sourceFile: sourceFile,
		rules: []Rule{
			&TransitiveBroaderRule{},
			&TransitiveNarrowerRule{},
			&SymmetricRelatedRule{},
			&TransitiveExactMatchRule{},
			&InconsistencyRule{},
		},
	}
}

func (r *Reasoner) Run(ctx context.Context) error {
	for _, rule := range r.rules {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := rule.Apply(r.doc, r.hierarchy, r.factSet); err != nil {
			return fmt.Errorf("rule %s failed: %w", rule.Name(), err)
		}
	}

	for i := range r.factSet.Facts {
		if r.factSet.Facts[i].Provenance != nil && r.factSet.Facts[i].Provenance.SourceFile == "" {
			r.factSet.Facts[i].Provenance.SourceFile = r.sourceFile
		}
	}

	return nil
}

func (r *Reasoner) Facts() []Fact {
	return r.factSet.Facts
}

func (r *Reasoner) FactSet() *FactSet {
	return r.factSet
}

func (r *Reasoner) Inconsistencies() []Inconsistency {
	return r.factSet.Inconsistencies
}

func (r *Reasoner) AddRule(rule Rule) {
	r.rules = append(r.rules, rule)
}

func (r *Reasoner) ClearRules() {
	r.rules = nil
}

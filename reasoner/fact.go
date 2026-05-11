// Package reasoner provides inference over SKOS thesaurus hierarchies.
//
// This package runs inference rules over both the Turtle source AST and
// the SKOS concept hierarchy, producing a structured fact set with
// provenance information for each inferred fact.
//
// The output is designed to be consumed by a future DSL for SPARQL query generation.
//
// Example:
//
//	parser := ttlast.NewParser()
//	doc, err := parser.ParseFile("thesaurus.ttl")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	hierarchy, err := skosast.BuildFromDocument(doc)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	r := reasoner.New(doc, hierarchy)
//	if err := r.Run(context.Background()); err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, fact := range r.Facts() {
//	    fmt.Printf("%s %s %s\n", fact.Subject, fact.Predicate, fact.Object)
//	}
package reasoner

import "fmt"

type Provenance struct {
	SourceFile string `json:"source_file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	StartPos   int    `json:"start_pos"`
	EndPos     int    `json:"end_pos"`
}

func (p *Provenance) String() string {
	return fmt.Sprintf("%s:%d:%d (bytes %d-%d)", p.SourceFile, p.Line, p.Column, p.StartPos, p.EndPos)
}

type Fact struct {
	Subject    string      `json:"subject"`
	Predicate  string      `json:"predicate"`
	Object     string      `json:"object"`
	Derived    bool        `json:"derived"`
	Provenance *Provenance `json:"provenance,omitempty"`
}

func (f *Fact) String() string {
	origin := "source"
	if f.Derived {
		origin = "inferred"
	}
	if f.Provenance != nil {
		return fmt.Sprintf("%s %s %s (%s from %s)", f.Subject, f.Predicate, f.Object, origin, f.Provenance)
	}
	return fmt.Sprintf("%s %s %s (%s)", f.Subject, f.Predicate, f.Object, origin)
}

type Inconsistency struct {
	Type       string      `json:"type"`
	Message    string      `json:"message"`
	Provenance *Provenance `json:"provenance,omitempty"`
}

func (i *Inconsistency) String() string {
	if i.Provenance != nil {
		return fmt.Sprintf("%s: %s (at %s)", i.Type, i.Message, i.Provenance)
	}
	return fmt.Sprintf("%s: %s", i.Type, i.Message)
}

type FactSet struct {
	Facts           []Fact          `json:"facts"`
	Inconsistencies []Inconsistency `json:"inconsistencies,omitempty"`
}

func (fs *FactSet) AddFact(fact Fact) {
	fs.Facts = append(fs.Facts, fact)
}

func (fs *FactSet) AddInconsistency(inconsistency Inconsistency) {
	fs.Inconsistencies = append(fs.Inconsistencies, inconsistency)
}

func (fs *FactSet) FactsByPredicate(predicate string) []Fact {
	var result []Fact
	for _, f := range fs.Facts {
		if f.Predicate == predicate {
			result = append(result, f)
		}
	}
	return result
}

func (fs *FactSet) FactsForSubject(subject string) []Fact {
	var result []Fact
	for _, f := range fs.Facts {
		if f.Subject == subject {
			result = append(result, f)
		}
	}
	return result
}

func (fs *FactSet) FactsForObject(obj string) []Fact {
	var result []Fact
	for _, f := range fs.Facts {
		if f.Object == obj {
			result = append(result, f)
		}
	}
	return result
}

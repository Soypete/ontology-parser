package skosast

import "fmt"

type Node interface {
	IRI() string
	Type() string
	Label() string
}

type BaseNode struct {
	iri      string
	nodeType string
	label    string
}

func (b *BaseNode) IRI() string   { return b.iri }
func (b *BaseNode) Type() string  { return b.nodeType }
func (b *BaseNode) Label() string { return b.label }

type Concept struct {
	BaseNode
	Broader      []string
	Narrower     []string
	Related      []string
	ExactMatch   []string
	CloseMatch   []string
	InScheme     []string
	TopConceptOf []string
	PrefLabel    string
	AltLabels    []string
	HiddenLabels []string
	Notation     string
}

func (c *Concept) String() string {
	return fmt.Sprintf("Concept(%s)", c.iri)
}

type ConceptScheme struct {
	BaseNode
	HasTopConcept []string
	TopConceptOf  []string
}

func (s *ConceptScheme) String() string {
	return fmt.Sprintf("ConceptScheme(%s)", s.iri)
}

type Collection struct {
	BaseNode
	Members    []string
	MemberList []string
}

func (c *Collection) String() string {
	return fmt.Sprintf("Collection(%s)", c.iri)
}

type OrderedCollection struct {
	BaseNode
	Members    []string
	MemberList []string
}

func (c *OrderedCollection) String() string {
	return fmt.Sprintf("OrderedCollection(%s)", c.iri)
}

type Hierarchy struct {
	Concepts           map[string]*Concept
	Schemes            map[string]*ConceptScheme
	Collections        map[string]*Collection
	OrderedCollections map[string]*OrderedCollection
	TopConcepts        []string
}

func NewHierarchy() *Hierarchy {
	return &Hierarchy{
		Concepts:           make(map[string]*Concept),
		Schemes:            make(map[string]*ConceptScheme),
		Collections:        make(map[string]*Collection),
		OrderedCollections: make(map[string]*OrderedCollection),
	}
}

func (h *Hierarchy) GetConcept(iri string) *Concept {
	return h.Concepts[iri]
}

func (h *Hierarchy) GetScheme(iri string) *ConceptScheme {
	return h.Schemes[iri]
}

func (h *Hierarchy) GetCollection(iri string) *Collection {
	return h.Collections[iri]
}

func (h *Hierarchy) GetOrderedCollection(iri string) *OrderedCollection {
	return h.OrderedCollections[iri]
}

func (h *Hierarchy) AddConcept(iri string) *Concept {
	if c, ok := h.Concepts[iri]; ok {
		return c
	}
	c := &Concept{
		BaseNode: BaseNode{iri: iri, nodeType: "Concept"},
	}
	h.Concepts[iri] = c
	return c
}

func (h *Hierarchy) AddScheme(iri string) *ConceptScheme {
	if s, ok := h.Schemes[iri]; ok {
		return s
	}
	s := &ConceptScheme{
		BaseNode: BaseNode{iri: iri, nodeType: "ConceptScheme"},
	}
	h.Schemes[iri] = s
	return s
}

func (h *Hierarchy) AddCollection(iri string) *Collection {
	if c, ok := h.Collections[iri]; ok {
		return c
	}
	c := &Collection{
		BaseNode: BaseNode{iri: iri, nodeType: "Collection"},
	}
	h.Collections[iri] = c
	return c
}

func (h *Hierarchy) AddOrderedCollection(iri string) *OrderedCollection {
	if c, ok := h.OrderedCollections[iri]; ok {
		return c
	}
	c := &OrderedCollection{
		BaseNode: BaseNode{iri: iri, nodeType: "OrderedCollection"},
	}
	h.OrderedCollections[iri] = c
	return c
}

const (
	SKOSNS = "http://www.w3.org/2004/02/skos/core#"

	SKOSConcept           = SKOSNS + "Concept"
	SKOSConceptScheme     = SKOSNS + "ConceptScheme"
	SKOSCollection        = SKOSNS + "Collection"
	SKOSOrderedCollection = SKOSNS + "OrderedCollection"

	SKOSPrefLabel   = SKOSNS + "prefLabel"
	SKOSAltLabel    = SKOSNS + "altLabel"
	SKOSHiddenLabel = SKOSNS + "hiddenLabel"
	SKOSNotation    = SKOSNS + "notation"

	SKOSBroader            = SKOSNS + "broader"
	SKOSNarrower           = SKOSNS + "narrower"
	SKOSRelated            = SKOSNS + "related"
	SKOSBroaderTransitive  = SKOSNS + "broaderTransitive"
	SKOSNarrowerTransitive = SKOSNS + "narrowerTransitive"

	SKOSExactMatch = SKOSNS + "exactMatch"
	SKOSCloseMatch = SKOSNS + "closeMatch"

	SKOSInScheme      = SKOSNS + "inScheme"
	SKOSHasTopConcept = SKOSNS + "hasTopConcept"
	SKOSTopConceptOf  = SKOSNS + "topConceptOf"

	SKOSMember     = SKOSNS + "member"
	SKOSMemberList = SKOSNS + "memberList"
)

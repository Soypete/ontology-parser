package viz

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/soypete/ontology-go/types"
)

type Node struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Comment   string `json:"comment,omitempty"`
}

type Edge struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Predicate string `json:"predicate"`
	Label     string `json:"label"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

type Graph struct {
	Nodes      []Node   `json:"nodes"`
	Edges      []Edge   `json:"edges"`
	Namespaces []string `json:"namespaces"`
	Types      []string `json:"types"`
	Predicates []string `json:"predicates"`
}

func NewGraph() *Graph {
	return &Graph{
		Nodes:      []Node{},
		Edges:      []Edge{},
		Namespaces: []string{},
		Types:      []string{},
		Predicates: []string{},
	}
}

var (
	rdfType        = types.RDFType
	rdfsSubClass   = types.RDFSSubClassOf
	rdfsSubProp    = types.RDFSSubPropertyOf
	owlClass       = types.OWLClass
	skosBroader    = "http://www.w3.org/2004/02/skos/core#broader"
	skosNarrower   = "http://www.w3.org/2004/02/skos/core#narrower"
	skosExactMatch = "http://www.w3.org/2004/02/skos/core#exactMatch"
	skosCloseMatch = "http://www.w3.org/2004/02/skos/core#closeMatch"
)

var nodeTypes = map[string]string{
	types.OWLClass:              "class",
	types.OWLObjectProperty:     "objectProperty",
	types.OWLDatatypeProperty:   "datatypeProperty",
	types.OWLAnnotationProperty: "annotationProperty",
	types.RDFSClass:             "class",
	types.RDFSResource:          "resource",
	types.RDFSLiteral:           "literal",
}

var edgeTypes = map[string]string{
	types.RDFType:                                    "rdf:type",
	types.RDFSSubClassOf:                             "rdfs:subClassOf",
	types.RDFSSubPropertyOf:                          "rdfs:subPropertyOf",
	types.OWLEquivalentClass:                         "owl:equivalentClass",
	types.OWLEquivalentProperty:                      "owl:equivalentProperty",
	types.OWLInverseOf:                               "owl:inverseOf",
	"http://www.w3.org/2004/02/skos/core#broader":    "skos:broader",
	"http://www.w3.org/2004/02/skos/core#narrower":   "skos:narrower",
	"http://www.w3.org/2004/02/skos/core#exactMatch": "skos:exactMatch",
	"http://www.w3.org/2004/02/skos/core#closeMatch": "skos:closeMatch",
}

func (g *Graph) ProcessTriples(triples []types.Triple) {
	nodeMap := make(map[string]*Node)
	labels := make(map[string]string)

	for _, t := range triples {
		if t.Predicate == types.RDFSLabel && t.IsLiteral {
			labels[t.Subject] = t.Object
		}
		if t.Predicate == types.RDFSComment && t.IsLiteral {
			if node, ok := nodeMap[t.Subject]; ok {
				node.Comment = t.Object
			}
		}
	}

	for _, t := range triples {
		isClass := false
		isProp := false

		if t.Predicate == rdfType {
			if ntype, ok := nodeTypes[t.Object]; ok {
				isClass = ntype == "class"
				isProp = strings.HasSuffix(ntype, "Property")
			}
		}

		if isClass || isProp || isRelevantPredicate(t.Predicate) || t.Predicate == rdfType {
			if nodeMap[t.Subject] == nil {
				ns := extractNamespace(t.Subject)
				node := &Node{
					ID:        t.Subject,
					Label:     getLabel(t.Subject, labels),
					Namespace: ns,
					Type:      "",
				}
				g.addNamespace(ns)
				nodeMap[t.Subject] = node
				g.Nodes = append(g.Nodes, *node)
			}
		}

		if t.Predicate == rdfType {
			if ntype, ok := nodeTypes[t.Object]; ok {
				if node, ok := nodeMap[t.Subject]; ok {
					node.Type = ntype
				}
				g.addType(ntype)
			}
		}
	}

	// Sync nodeMap changes back to g.Nodes
	for i := range g.Nodes {
		if n, ok := nodeMap[g.Nodes[i].ID]; ok {
			g.Nodes[i].Type = n.Type
			g.Nodes[i].Comment = n.Comment
		}
	}

	for _, t := range triples {
		if isRelevantPredicate(t.Predicate) && !t.IsLiteral {
			if nodeMap[t.Subject] == nil {
				ns := extractNamespace(t.Subject)
				node := &Node{
					ID:        t.Subject,
					Label:     getLabel(t.Subject, labels),
					Namespace: ns,
					Type:      "resource",
				}
				g.addNamespace(ns)
				nodeMap[t.Subject] = node
				g.Nodes = append(g.Nodes, *node)
			}
			if nodeMap[t.Object] == nil && !strings.HasPrefix(t.Object, "http://www.w3.org/") {
				ns := extractNamespace(t.Object)
				node := &Node{
					ID:        t.Object,
					Label:     getLabel(t.Object, labels),
					Namespace: ns,
					Type:      "resource",
				}
				g.addNamespace(ns)
				nodeMap[t.Object] = node
				g.Nodes = append(g.Nodes, *node)
			}

			// Only create edges for non-rdf:type predicates
			if edgePredicates[t.Predicate] {
				edgeType := "other"
				if et, ok := edgeTypes[t.Predicate]; ok {
					edgeType = et
				}
				ns := extractNamespace(t.Predicate)

				edge := Edge{
					Source:    t.Subject,
					Target:    t.Object,
					Predicate: t.Predicate,
					Label:     edgeType,
					Namespace: ns,
					Type:      edgeType,
				}
				g.Edges = append(g.Edges, edge)
				g.addPredicate(edgeType)
			}
		}
	}
}

var edgePredicates = map[string]bool{
	rdfType:                     false, // only for node typing, not edges
	rdfsSubClass:                true,
	rdfsSubProp:                 true,
	types.OWLEquivalentClass:    true,
	types.OWLEquivalentProperty: true,
	types.OWLInverseOf:          true,
	skosBroader:                 true,
	skosNarrower:                true,
	skosExactMatch:              true,
	skosCloseMatch:              true,
}

func isRelevantPredicate(pred string) bool {
	_, ok := edgePredicates[pred]
	return ok
}

func extractNamespace(iri string) string {
	if strings.HasPrefix(iri, "_:") {
		return "_"
	}
	lastHash := strings.LastIndex(iri, "#")
	lastSlash := strings.LastIndex(iri, "/")
	sep := lastHash
	if lastSlash > sep {
		sep = lastSlash
	}
	if sep > 0 {
		ns := iri[:sep+1]
		if ns == "http://www.w3.org/" {
			if strings.HasPrefix(iri, "http://www.w3.org/1999/") {
				return "rdf:"
			}
			if strings.HasPrefix(iri, "http://www.w3.org/2000/01/") {
				return "rdfs:"
			}
			if strings.HasPrefix(iri, "http://www.w3.org/2001/") {
				return "xsd:"
			}
			if strings.HasPrefix(iri, "http://www.w3.org/2002/07/") {
				return "owl:"
			}
			if strings.HasPrefix(iri, "http://www.w3.org/2004/02/skos/") {
				return "skos:"
			}
		}
		prefix := ns
		if strings.HasSuffix(prefix, "#") {
			prefix = strings.TrimSuffix(prefix, "#") + "#"
		}
		return prefix
	}
	return iri
}

func getLabel(iri string, labels map[string]string) string {
	if label, ok := labels[iri]; ok {
		return label
	}

	if strings.HasPrefix(iri, "_:") {
		return iri
	}

	lastHash := strings.LastIndex(iri, "#")
	lastSlash := strings.LastIndex(iri, "/")

	sep := lastHash
	if lastSlash > sep {
		sep = lastSlash
	}

	if sep > 0 && sep < len(iri)-1 {
		return iri[sep+1:]
	}

	return iri
}

func (g *Graph) addNamespace(ns string) {
	for _, n := range g.Namespaces {
		if n == ns {
			return
		}
	}
	g.Namespaces = append(g.Namespaces, ns)
}

func (g *Graph) addType(t string) {
	for _, nt := range g.Types {
		if nt == t {
			return
		}
	}
	g.Types = append(g.Types, t)
}

func (g *Graph) addPredicate(p string) {
	for _, pred := range g.Predicates {
		if pred == p {
			return
		}
	}
	g.Predicates = append(g.Predicates, p)
}

func (g *Graph) ToJSON() []byte {
	data, err := json.Marshal(g)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error": "%v"}`, err))
	}
	return data
}

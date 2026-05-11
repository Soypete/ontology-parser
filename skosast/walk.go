// Package skosast provides traversal primitives for SKOS hierarchies.
//
// This file contains walk functions similar to go/ast.Walk for traversing
// the SKOS concept hierarchy.
package skosast

func Walk(h *Hierarchy, v Visitor, n Node) {
	if n == nil {
		return
	}

	if v = v.Visit(n); v == nil {
		return
	}

	walkChildren(h, v, n)
}

func walkChildren(h *Hierarchy, v Visitor, n Node) {
	switch node := n.(type) {
	case *Concept:
		for _, broader := range node.Broader {
			if c := h.Concepts[broader]; c != nil {
				Walk(h, v, c)
			}
		}

	case *ConceptScheme:
		for _, top := range node.HasTopConcept {
			if c := h.Concepts[top]; c != nil {
				Walk(h, v, c)
			}
		}

	case *Collection:
		for _, member := range node.Members {
			if c := h.Concepts[member]; c != nil {
				Walk(h, v, c)
			}
		}

	case *OrderedCollection:
		for _, member := range node.MemberList {
			if c := h.Concepts[member]; c != nil {
				Walk(h, v, c)
			}
		}
	}
}

func PreOrder(h *Hierarchy, n Node) []Node {
	var result []Node
	preOrderRecursive(h, n, &result)
	return result
}

func preOrderRecursive(h *Hierarchy, n Node, result *[]Node) {
	if n == nil {
		return
	}
	*result = append(*result, n)

	switch node := n.(type) {
	case *Concept:
		for _, broader := range node.Broader {
			if c := h.Concepts[broader]; c != nil {
				preOrderRecursive(h, c, result)
			}
		}

	case *ConceptScheme:
		for _, top := range node.HasTopConcept {
			if c := h.Concepts[top]; c != nil {
				preOrderRecursive(h, c, result)
			}
		}
	}
}

func PostOrder(h *Hierarchy, n Node) []Node {
	var result []Node
	postOrderRecursive(h, n, &result)
	return result
}

func postOrderRecursive(h *Hierarchy, n Node, result *[]Node) {
	if n == nil {
		return
	}

	switch node := n.(type) {
	case *Concept:
		for _, broader := range node.Broader {
			if c := h.Concepts[broader]; c != nil {
				postOrderRecursive(h, c, result)
			}
		}

	case *ConceptScheme:
		for _, top := range node.HasTopConcept {
			if c := h.Concepts[top]; c != nil {
				postOrderRecursive(h, c, result)
			}
		}
	}

	*result = append(*result, n)
}

type Visitor interface {
	Visit(n Node) Visitor
}

type DefaultVisitor struct{}

func (v DefaultVisitor) Visit(n Node) Visitor {
	return v
}

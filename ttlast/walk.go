package ttlast

func Walk(v Visitor, n Node) {
	if n == nil {
		return
	}

	if v = v.Visit(n); v == nil {
		return
	}

	walkChildren(v, n)
}

func walkChildren(v Visitor, n Node) {
	switch node := n.(type) {
	case *Document:
		for i := range node.Prefixes {
			Walk(v, &node.Prefixes[i])
		}
		for i := range node.Statements {
			Walk(v, &node.Statements[i])
		}
		for i := range node.Comments {
			Walk(v, &node.Comments[i])
		}

	case *PrefixDecl:
		Walk(v, &node.IRI)

	case *Statement:
		Walk(v, &node.Triple)

	case *Triple:
		Walk(v, node.Subject)
		Walk(v, node.Predicate)
		Walk(v, node.Object)

	case *Comment:
		// No children

	case *IRI, *PrefixedName, *BlankNode, *Literal:
		// Terminal terms

	case *Collection:
		for _, e := range node.Elements {
			Walk(v, e)
		}
	}
}

func PreOrder(n Node) []Node {
	var result []Node
	preOrderRecursive(n, &result)
	return result
}

func preOrderRecursive(n Node, result *[]Node) {
	if n == nil {
		return
	}
	*result = append(*result, n)

	switch node := n.(type) {
	case *Document:
		for i := range node.Prefixes {
			preOrderRecursive(&node.Prefixes[i], result)
		}
		for i := range node.Statements {
			preOrderRecursive(&node.Statements[i], result)
		}
		for i := range node.Comments {
			preOrderRecursive(&node.Comments[i], result)
		}

	case *PrefixDecl:
		preOrderRecursive(&node.IRI, result)

	case *Statement:
		preOrderRecursive(&node.Triple, result)

	case *Triple:
		preOrderRecursive(node.Subject, result)
		preOrderRecursive(node.Predicate, result)
		preOrderRecursive(node.Object, result)

	case *Collection:
		for _, e := range node.Elements {
			preOrderRecursive(e, result)
		}
	}
}

func PostOrder(n Node) []Node {
	var result []Node
	postOrderRecursive(n, &result)
	return result
}

func postOrderRecursive(n Node, result *[]Node) {
	if n == nil {
		return
	}

	switch node := n.(type) {
	case *Document:
		for i := range node.Prefixes {
			postOrderRecursive(&node.Prefixes[i], result)
		}
		for i := range node.Statements {
			postOrderRecursive(&node.Statements[i], result)
		}
		for i := range node.Comments {
			postOrderRecursive(&node.Comments[i], result)
		}

	case *PrefixDecl:
		postOrderRecursive(&node.IRI, result)

	case *Statement:
		postOrderRecursive(&node.Triple, result)

	case *Triple:
		postOrderRecursive(node.Subject, result)
		postOrderRecursive(node.Predicate, result)
		postOrderRecursive(node.Object, result)

	case *Collection:
		for _, e := range node.Elements {
			postOrderRecursive(e, result)
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

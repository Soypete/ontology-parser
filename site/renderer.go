package site

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/soypete/ontology-go/ttl"
	"github.com/soypete/ontology-go/types"
	"github.com/soypete/ontology-go/validate"
)

const (
	rdfType               = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	rdfsClass             = "http://www.w3.org/2000/01/rdf-schema#Class"
	rdfsSubClassOf        = "http://www.w3.org/2000/01/rdf-schema#subClassOf"
	rdfsLabel             = "http://www.w3.org/2000/01/rdf-schema#label"
	rdfsComment           = "http://www.w3.org/2000/01/rdf-schema#comment"
	owlClass              = "http://www.w3.org/2002/07/owl#Class"
	owlObjectProperty     = "http://www.w3.org/2002/07/owl#ObjectProperty"
	owlDatatypeProperty   = "http://www.w3.org/2002/07/owl#DatatypeProperty"
	owlAnnotationProperty = "http://www.w3.org/2002/07/owl#AnnotationProperty"
	skosConceptScheme     = "http://www.w3.org/2004/02/skos/core#ConceptScheme"
	skosConcept           = "http://www.w3.org/2004/02/skos/core#Concept"
	skosHasTopConcept     = "http://www.w3.org/2004/02/skos/core#hasTopConcept"
	skosDefinition        = "http://www.w3.org/2004/02/skos/core#definition"
)

type Config struct {
	OutputDir string
	Title     string
}

type Renderer struct {
	config     Config
	classes    []ClassInfo
	properties []PropertyInfo
	schemes    []SchemeInfo
	triples    []types.Triple
	report     *validate.ValidationReport
}

type ClassInfo struct {
	Name        string
	URI         string
	Label       string
	Description string
	SubClassOf  []string
	Properties  []string
	InScheme    string
}

type PropertyInfo struct {
	Name        string
	URI         string
	Label       string
	Description string
	Domain      string
	Range       string
	IsDatatype  bool
}

type SchemeInfo struct {
	Name        string
	URI         string
	Label       string
	TopConcepts []string
}

func NewRenderer(config Config) *Renderer {
	return &Renderer{
		config: config,
	}
}

func (r *Renderer) LoadOntology(ctx context.Context, files []string) error {
	var allTriples []types.Triple

	for _, file := range files {
		triples, err := r.loadFile(file)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", file, err)
		}
		allTriples = append(allTriples, triples...)
	}

	r.triples = allTriples
	r.extractMetadata()
	r.runValidation(ctx)

	return nil
}

func (r *Renderer) loadFile(path string) ([]types.Triple, error) {
	parser := ttl.NewTurtleParser()
	return parser.ParseFile(path)
}

func (r *Renderer) extractMetadata() {
	classMap := make(map[string]ClassInfo)
	propertyMap := make(map[string]PropertyInfo)
	schemeMap := make(map[string]SchemeInfo)

	for _, t := range r.triples {
		if t.Predicate == rdfType {
			switch t.Object {
			case rdfsClass, owlClass, skosConcept:
				if _, ok := classMap[t.Subject]; !ok {
					classMap[t.Subject] = ClassInfo{Name: r.localName(t.Subject), URI: t.Subject}
				}
			case "http://www.w3.org/1999/02/22-rdf-syntax-ns#Property", owlObjectProperty, owlDatatypeProperty, owlAnnotationProperty:
				if _, ok := propertyMap[t.Subject]; !ok {
					p := PropertyInfo{Name: r.localName(t.Subject), URI: t.Subject, IsDatatype: t.Object == owlDatatypeProperty}
					propertyMap[t.Subject] = p
				}
			case skosConceptScheme:
				if _, ok := schemeMap[t.Subject]; !ok {
					schemeMap[t.Subject] = SchemeInfo{Name: r.localName(t.Subject), URI: t.Subject}
				}
			}
		}

		if t.Predicate == rdfsLabel && t.IsLiteral {
			if c, ok := classMap[t.Subject]; ok {
				c.Label = t.Object
				classMap[t.Subject] = c
			}
			if p, ok := propertyMap[t.Subject]; ok {
				p.Label = t.Object
				propertyMap[t.Subject] = p
			}
			if s, ok := schemeMap[t.Subject]; ok {
				s.Label = t.Object
				schemeMap[t.Subject] = s
			}
		}

		// rdfs:comment populates Description as a fallback.
		// skos:definition (handled next) takes precedence when present.
		if t.Predicate == rdfsComment && t.IsLiteral {
			if c, ok := classMap[t.Subject]; ok && c.Description == "" {
				c.Description = t.Object
				classMap[t.Subject] = c
			}
			if p, ok := propertyMap[t.Subject]; ok && p.Description == "" {
				p.Description = t.Object
				propertyMap[t.Subject] = p
			}
		}
		if t.Predicate == skosDefinition && t.IsLiteral {
			if c, ok := classMap[t.Subject]; ok {
				c.Description = t.Object
				classMap[t.Subject] = c
			}
			if p, ok := propertyMap[t.Subject]; ok {
				p.Description = t.Object
				propertyMap[t.Subject] = p
			}
		}

		if t.Predicate == rdfsSubClassOf {
			if c, ok := classMap[t.Subject]; ok {
				c.SubClassOf = append(c.SubClassOf, r.localName(t.Object))
				classMap[t.Subject] = c
			}
		}

		if t.Predicate == skosHasTopConcept {
			if s, ok := schemeMap[t.Object]; ok {
				s.TopConcepts = append(s.TopConcepts, r.localName(t.Subject))
				schemeMap[t.Object] = s
			}
		}
	}

	r.classes = make([]ClassInfo, 0, len(classMap))
	for _, c := range classMap {
		r.classes = append(r.classes, c)
	}

	r.properties = make([]PropertyInfo, 0, len(propertyMap))
	for _, p := range propertyMap {
		r.properties = append(r.properties, p)
	}

	r.schemes = make([]SchemeInfo, 0, len(schemeMap))
	for _, s := range schemeMap {
		r.schemes = append(r.schemes, s)
	}
}

func (r *Renderer) runValidation(ctx context.Context) {
	validator := validate.NewValidator(r.triples)
	report, _ := validator.Validate(ctx)
	r.report = report
}

func (r *Renderer) localName(uri string) string {
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		parts := strings.Split(uri, "/")
		last := parts[len(parts)-1]
		if idx := strings.Index(last, "#"); idx != -1 {
			return last[idx+1:]
		}
		return last
	}
	return uri
}

func (r *Renderer) Render(ctx context.Context) error {
	if err := os.MkdirAll(r.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	if err := r.renderIndex(); err != nil {
		return fmt.Errorf("failed to render index: %w", err)
	}

	if err := r.renderClassesIndex(); err != nil {
		return fmt.Errorf("failed to render classes: %w", err)
	}

	if err := r.renderPropertiesIndex(); err != nil {
		return fmt.Errorf("failed to render properties: %w", err)
	}

	if err := r.renderValidation(); err != nil {
		return fmt.Errorf("failed to render validation: %w", err)
	}

	if err := r.renderSPARQL(); err != nil {
		return fmt.Errorf("failed to render sparql: %w", err)
	}

	if err := r.renderViz(); err != nil {
		return fmt.Errorf("failed to render viz: %w", err)
	}

	if err := r.renderVizGraph(); err != nil {
		return fmt.Errorf("failed to render viz graph: %w", err)
	}

	return nil
}

func (r *Renderer) renderIndex() error {
	tmpl, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		return err
	}

	data := struct {
		Title       string
		ClassCount  int
		PropCount   int
		SchemeCount int
		TripleCount int
	}{
		Title:       r.config.Title,
		ClassCount:  len(r.classes),
		PropCount:   len(r.properties),
		SchemeCount: len(r.schemes),
		TripleCount: len(r.triples),
	}

	f, err := os.Create(filepath.Join(r.config.OutputDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func (r *Renderer) renderClassesIndex() error {
	classesDir := filepath.Join(r.config.OutputDir, "classes")
	if err := os.MkdirAll(classesDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("classes").Parse(classesTemplate)
	if err != nil {
		return err
	}

	data := struct {
		Title   string
		Classes []ClassInfo
	}{
		Title:   r.config.Title,
		Classes: r.classes,
	}

	f, err := os.Create(filepath.Join(classesDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func (r *Renderer) renderPropertiesIndex() error {
	propsDir := filepath.Join(r.config.OutputDir, "properties")
	if err := os.MkdirAll(propsDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("properties").Parse(propertiesTemplate)
	if err != nil {
		return err
	}

	data := struct {
		Title      string
		Properties []PropertyInfo
	}{
		Title:      r.config.Title,
		Properties: r.properties,
	}

	f, err := os.Create(filepath.Join(propsDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func (r *Renderer) renderValidation() error {
	validationDir := filepath.Join(r.config.OutputDir, "validation")
	if err := os.MkdirAll(validationDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("validation").Parse(validationTemplate)
	if err != nil {
		return err
	}

	data := struct {
		Title  string
		Report *validate.ValidationReport
	}{
		Title:  r.config.Title,
		Report: r.report,
	}

	f, err := os.Create(filepath.Join(validationDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func (r *Renderer) renderSPARQL() error {
	sparqlDir := filepath.Join(r.config.OutputDir, "sparql")
	if err := os.MkdirAll(sparqlDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("sparql").Parse(sparqlTemplate)
	if err != nil {
		return err
	}

	data := struct {
		Title string
	}{
		Title: r.config.Title,
	}

	f, err := os.Create(filepath.Join(sparqlDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func (r *Renderer) renderViz() error {
	vizDir := filepath.Join(r.config.OutputDir, "viz")
	if err := os.MkdirAll(vizDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("viz").Parse(vizTemplate)
	if err != nil {
		return err
	}

	data := struct {
		Title string
	}{
		Title: r.config.Title,
	}

	f, err := os.Create(filepath.Join(vizDir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

type graphNode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Comment   string `json:"comment,omitempty"`
}

type graphEdge struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Predicate string `json:"predicate"`
	Label     string `json:"label"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

func (r *Renderer) renderVizGraph() error {
	vizDir := filepath.Join(r.config.OutputDir, "viz")
	if err := os.MkdirAll(vizDir, 0755); err != nil {
		return err
	}

	graph := struct {
		Nodes      []graphNode `json:"nodes"`
		Edges      []graphEdge `json:"edges"`
		Namespaces []string    `json:"namespaces"`
		Types      []string    `json:"types"`
		Predicates []string    `json:"predicates"`
	}{
		Nodes:      []graphNode{},
		Edges:      []graphEdge{},
		Namespaces: []string{},
		Types:      []string{},
		Predicates: []string{},
	}

	nodeMap := make(map[string]*graphNode)
	labels := make(map[string]string)

	for _, t := range r.triples {
		if t.Predicate == "http://www.w3.org/2000/01/rdf-schema#label" && t.IsLiteral {
			labels[t.Subject] = t.Object
		}
	}

	nodeTypes := map[string]string{
		"http://www.w3.org/2002/07/owl#Class":              "class",
		"http://www.w3.org/2002/07/owl#ObjectProperty":     "objectProperty",
		"http://www.w3.org/2002/07/owl#DatatypeProperty":   "datatypeProperty",
		"http://www.w3.org/2002/07/owl#AnnotationProperty": "annotationProperty",
		"http://www.w3.org/2000/01/rdf-schema#Class":       "class",
		"http://www.w3.org/2000/01/rdf-schema#Resource":    "resource",
		"http://www.w3.org/2000/01/rdf-schema#Literal":     "literal",
	}

	edgeTypes := map[string]string{
		"http://www.w3.org/1999/02/22-rdf-syntax-ns#type":    "rdf:type",
		"http://www.w3.org/2000/01/rdf-schema#subClassOf":    "rdfs:subClassOf",
		"http://www.w3.org/2000/01/rdf-schema#subPropertyOf": "rdfs:subPropertyOf",
		"http://www.w3.org/2000/01/rdf-schema#domain":        "rdfs:domain",
		"http://www.w3.org/2000/01/rdf-schema#range":         "rdfs:range",
		"http://www.w3.org/2002/07/owl#equivalentClass":      "owl:equivalentClass",
		"http://www.w3.org/2002/07/owl#equivalentProperty":   "owl:equivalentProperty",
		"http://www.w3.org/2002/07/owl#inverseOf":            "owl:inverseOf",
	}

	edgePredicates := map[string]bool{
		"http://www.w3.org/1999/02/22-rdf-syntax-ns#type":    false,
		"http://www.w3.org/2000/01/rdf-schema#subClassOf":    true,
		"http://www.w3.org/2000/01/rdf-schema#subPropertyOf": true,
		"http://www.w3.org/2000/01/rdf-schema#domain":        true,
		"http://www.w3.org/2000/01/rdf-schema#range":         true,
		"http://www.w3.org/2002/07/owl#equivalentClass":      true,
		"http://www.w3.org/2002/07/owl#equivalentProperty":   true,
		"http://www.w3.org/2002/07/owl#inverseOf":            true,
	}

	graphNamespaces := make(map[string]bool)
	graphTypes := make(map[string]bool)
	graphPredicates := make(map[string]bool)

	for _, t := range r.triples {
		if t.Predicate == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
			if ntype, ok := nodeTypes[t.Object]; ok {
				if nodeMap[t.Subject] == nil {
					ns := extractNamespaceStatic(t.Subject)
					node := &graphNode{
						ID:        t.Subject,
						Label:     getLabelStatic(t.Subject, labels),
						Namespace: ns,
						Type:      ntype,
					}
					if !graphNamespaces[ns] {
						graphNamespaces[ns] = true
						graph.Namespaces = append(graph.Namespaces, ns)
					}
					nodeMap[t.Subject] = node
					graph.Nodes = append(graph.Nodes, *node)
				} else {
					nodeMap[t.Subject].Type = ntype
				}
				if !graphTypes[ntype] {
					graphTypes[ntype] = true
					graph.Types = append(graph.Types, ntype)
				}
			}
		}
	}

	for _, t := range r.triples {
		if _, ok := edgePredicates[t.Predicate]; ok && !t.IsLiteral {
			if nodeMap[t.Subject] == nil {
				ns := extractNamespaceStatic(t.Subject)
				node := &graphNode{
					ID:        t.Subject,
					Label:     getLabelStatic(t.Subject, labels),
					Namespace: ns,
					Type:      "resource",
				}
				if !graphNamespaces[ns] {
					graphNamespaces[ns] = true
					graph.Namespaces = append(graph.Namespaces, ns)
				}
				nodeMap[t.Subject] = node
				graph.Nodes = append(graph.Nodes, *node)
			}
			if nodeMap[t.Object] == nil && !strings.HasPrefix(t.Object, "http://www.w3.org/") {
				ns := extractNamespaceStatic(t.Object)
				node := &graphNode{
					ID:        t.Object,
					Label:     getLabelStatic(t.Object, labels),
					Namespace: ns,
					Type:      "resource",
				}
				if !graphNamespaces[ns] {
					graphNamespaces[ns] = true
					graph.Namespaces = append(graph.Namespaces, ns)
				}
				nodeMap[t.Object] = node
				graph.Nodes = append(graph.Nodes, *node)
			}

			edgeType := "other"
			if et, ok := edgeTypes[t.Predicate]; ok {
				edgeType = et
			}
			ns := extractNamespaceStatic(t.Predicate)

			edge := graphEdge{
				Source:    t.Subject,
				Target:    t.Object,
				Predicate: t.Predicate,
				Label:     edgeType,
				Namespace: ns,
				Type:      edgeType,
			}
			graph.Edges = append(graph.Edges, edge)
			if !graphPredicates[edgeType] {
				graphPredicates[edgeType] = true
				graph.Predicates = append(graph.Predicates, edgeType)
			}
		}
	}

	data, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("failed to marshal graph: %w", err)
	}

	f, err := os.Create(filepath.Join(vizDir, "graph.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func extractNamespaceStatic(iri string) string {
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

func getLabelStatic(iri string, labels map[string]string) string {
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

func (r *Renderer) Triples() []types.Triple {
	return r.triples
}

func (r *Renderer) Classes() []ClassInfo {
	return r.classes
}

func (r *Renderer) Properties() []PropertyInfo {
	return r.properties
}

func (r *Renderer) ValidationReport() *validate.ValidationReport {
	return r.report
}

func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Ontology Documentation</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; line-height: 1.6; }
        h1 { color: #333; border-bottom: 2px solid #0066cc; padding-bottom: 10px; }
        .stats { display: flex; gap: 20px; margin: 20px 0; }
        .stat { background: #f5f5f5; padding: 15px 25px; border-radius: 8px; text-align: center; }
        .stat-value { font-size: 2em; font-weight: bold; color: #0066cc; }
        .stat-label { color: #666; }
        nav { background: #f8f8f8; padding: 15px; border-radius: 8px; margin: 20px 0; }
        nav a { margin-right: 20px; color: #0066cc; text-decoration: none; }
        nav a:hover { text-decoration: underline; }
        footer { margin-top: 40px; padding-top: 20px; border-top: 1px solid #ddd; color: #666; }
    </style>
</head>
<body>
    <h1>{{.Title}}</h1>
    <p>Ontology Documentation</p>
    
    <div class="stats">
        <div class="stat">
            <div class="stat-value">{{.ClassCount}}</div>
            <div class="stat-label">Classes</div>
        </div>
        <div class="stat">
            <div class="stat-value">{{.PropCount}}</div>
            <div class="stat-label">Properties</div>
        </div>
        <div class="stat">
            <div class="stat-value">{{.SchemeCount}}</div>
            <div class="stat-label">Schemes</div>
        </div>
        <div class="stat">
            <div class="stat-value">{{.TripleCount}}</div>
            <div class="stat-label">Triples</div>
        </div>
    </div>
    
    <nav>
        <a href="classes/">Classes</a>
        <a href="properties/">Properties</a>
        <a href="validation/">Validation Report</a>
        <a href="sparql/">SPARQL Query</a>
        <a href="viz/">Graph Visualization</a>
    </nav>
    
    <footer>
        Generated by ontology-go site
    </footer>
</body>
</html>`

const classesTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Classes - {{.Title}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; line-height: 1.6; }
        h1 { color: #333; border-bottom: 2px solid #0066cc; padding-bottom: 10px; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #f5f5f5; font-weight: 600; }
        tr:hover { background: #f9f9f9; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
        nav { margin-bottom: 20px; }
        nav a { margin-right: 20px; }
        .empty { color: #666; font-style: italic; }
    </style>
</head>
<body>
    <nav>
        <a href="../">← Home</a>
    </nav>
    <h1>Classes</h1>
    {{if .Classes}}
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Label</th>
                <th>Description</th>
                <th>SubClass Of</th>
            </tr>
        </thead>
        <tbody>
            {{range .Classes}}
            <tr>
                <td><a href="#">{{.Name}}</a></td>
                <td>{{.Label}}</td>
                <td>{{if .Description}}{{.Description}}{{else}}<span class="empty">-</span>{{end}}</td>
                <td>{{if .SubClassOf}}{{.SubClassOf}}{{else}}<span class="empty">-</span>{{end}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{else}}
    <p>No classes found.</p>
    {{end}}
</body>
</html>`

const propertiesTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Properties - {{.Title}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; line-height: 1.6; }
        h1 { color: #333; border-bottom: 2px solid #0066cc; padding-bottom: 10px; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #f5f5f5; font-weight: 600; }
        tr:hover { background: #f9f9f9; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
        nav { margin-bottom: 20px; }
        nav a { margin-right: 20px; }
        .tag { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 0.85em; }
        .tag-object { background: #e3f2fd; color: #1565c0; }
        .tag-datatype { background: #e8f5e9; color: #2e7d32; }
        .empty { color: #666; font-style: italic; }
    </style>
</head>
<body>
    <nav>
        <a href="../">← Home</a>
    </nav>
    <h1>Properties</h1>
    {{if .Properties}}
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Label</th>
                <th>Description</th>
                <th>Type</th>
            </tr>
        </thead>
        <tbody>
            {{range .Properties}}
            <tr>
                <td><a href="#">{{.Name}}</a></td>
                <td>{{.Label}}</td>
                <td>{{if .Description}}{{.Description}}{{else}}<span class="empty">-</span>{{end}}</td>
                <td>{{if .IsDatatype}}<span class="tag tag-datatype">Datatype</span>{{else}}<span class="tag tag-object">Object</span>{{end}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{else}}
    <p>No properties found.</p>
    {{end}}
</body>
</html>`

const validationTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Validation Report - {{.Title}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; line-height: 1.6; }
        h1 { color: #333; border-bottom: 2px solid #0066cc; padding-bottom: 10px; }
        .stats { display: flex; gap: 15px; margin: 20px 0; }
        .stat { background: #f5f5f5; padding: 12px 20px; border-radius: 6px; }
        nav { margin-bottom: 20px; }
        nav a { margin-right: 20px; }
        .issue { padding: 10px; margin: 10px 0; border-radius: 6px; border-left: 4px solid; }
        .issue-error { background: #ffebee; border-color: #c62828; }
        .issue-warning { background: #fff3e0; border-color: #ef6c00; }
        .issue-info { background: #e3f2fd; border-color: #1565c0; }
        .empty { color: #666; }
    </style>
</head>
<body>
    <nav>
        <a href="../">← Home</a>
    </nav>
    <h1>Validation Report</h1>
    
    {{if .Report}}
    <div class="stats">
        <div class="stat">Triples: {{.Report.TotalTriples}}</div>
        <div class="stat">Concepts: {{.Report.TotalConcepts}}</div>
        <div class="stat">Schemes: {{.Report.TotalSchemes}}</div>
    </div>
    
    <h2>Issues</h2>
    {{if .Report.Issues}}
        {{range .Report.Issues}}
        <div class="issue issue-{{.Severity}}">
            <strong>[{{.Severity}}]</strong> {{.Message}}
            {{if .Subject}}<br><small>Subject: {{.Subject}}</small>{{end}}
        </div>
        {{end}}
    {{else}}
        <p class="empty">No issues found.</p>
    {{end}}
    {{else}}
    <p class="empty">No validation report available.</p>
    {{end}}
</body>
</html>`

const sparqlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SPARQL Query - {{.Title}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 1000px; margin: 0 auto; padding: 20px; line-height: 1.6; }
        h1 { color: #333; border-bottom: 2px solid #0066cc; padding-bottom: 10px; }
        nav { margin-bottom: 20px; }
        nav a { margin-right: 20px; }
        textarea { width: 100%; height: 150px; font-family: monospace; padding: 10px; border: 1px solid #ddd; border-radius: 4px; font-size: 14px; }
        button { background: #0066cc; color: white; border: none; padding: 10px 20px; border-radius: 4px; cursor: pointer; font-size: 14px; margin-top: 10px; }
        button:hover { background: #0052a3; }
        button:disabled { background: #ccc; cursor: not-allowed; }
        .results { margin-top: 20px; }
        .error { background: #ffebee; color: #c62828; padding: 15px; border-radius: 4px; border-left: 4px solid #c62828; }
        .success { background: #e8f5e9; color: #2e7d32; padding: 15px; border-radius: 4px; border-left: 4px solid #2e7d32; }
        table { width: 100%; border-collapse: collapse; margin-top: 15px; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #f5f5f5; font-weight: 600; }
        pre { background: #f5f5f5; padding: 15px; border-radius: 4px; overflow-x: auto; }
        .loading { color: #666; font-style: italic; }
    </style>
</head>
<body>
    <nav>
        <a href="../">← Home</a>
    </nav>
    <h1>SPARQL Query Tester</h1>
    <p>Execute SPARQL queries against the loaded ontology.</p>
    
    <textarea id="query" placeholder="Enter SPARQL query...
Example:
SELECT ?s ?p ?o
WHERE {
  ?s ?p ?o
}
LIMIT 10">SELECT ?s ?p ?o
WHERE {
  ?s ?p ?o
}
LIMIT 10</textarea>
    
    <button id="runBtn" onclick="runQuery()">Run Query</button>
    <button onclick="loadExample('concepts')">Load Concepts</button>
    <button onclick="loadExample('schemes')">Load Schemes</button>
    
    <div id="results" class="results"></div>
    
    <script>
        async function runQuery() {
            const query = document.getElementById('query').value;
            const resultsDiv = document.getElementById('results');
            const btn = document.getElementById('runBtn');
            
            btn.disabled = true;
            resultsDiv.innerHTML = '<p class="loading">Executing query...</p>';
            
            try {
                const response = await fetch('/api/sparql?query=' + encodeURIComponent(query));
                const data = await response.json();
                
                if (data.error) {
                    resultsDiv.innerHTML = '<div class="error">' + data.error + '</div>';
                } else {
                    let html = '<div class="success">Query executed successfully</div>';
                    
                    if (data.Bindings && data.Bindings.length > 0) {
                        const vars = Object.keys(data.Bindings[0]);
                        html += '<table><thead><tr>';
                        for (const v of vars) {
                            html += '<th>' + v + '</th>';
                        }
                        html += '</tr></thead><tbody>';
                        for (const row of data.Bindings) {
                            html += '<tr>';
                            for (const v of vars) {
                                html += '<td>' + (row[v] || '') + '</td>';
                            }
                            html += '</tr>';
                        }
                        html += '</tbody></table>';
                    } else {
                        html += '<p>No results found.</p>';
                    }
                    
                    html += '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
                    resultsDiv.innerHTML = html;
                }
            } catch (err) {
                resultsDiv.innerHTML = '<div class="error">Error: ' + err.message + '</div>';
            }
            
            btn.disabled = false;
        }
        
        function loadExample(name) {
            const examples = {
                concepts: 'SELECT ?concept ?label WHERE {\n  ?concept a <http://www.w3.org/2004/02/skos/core#Concept> .\n  ?concept <http://www.w3.org/2000/01/rdf-schema#label> ?label .\n} LIMIT 20',
                schemes: 'SELECT ?scheme ?label WHERE {\n  ?scheme a <http://www.w3.org/2004/02/skos/core#ConceptScheme> .\n  ?scheme <http://www.w3.org/2000/01/rdf-schema#label> ?label .\n}'
            };
            document.getElementById('query').value = examples[name] || '';
        }
    </script>
</body>
</html>`

const vizTemplate = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Graph Visualization - {{.Title}}</title>
  <script src="https://d3js.org/d3.v7.min.js"></script>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 0; overflow: hidden; background: #f8f9fa; }
    #graph { width: 100vw; height: 100vh; }
    #controls { position: fixed; top: 10px; right: 10px; background: #fff; padding: 0; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.15); max-height: 90vh; overflow-y: auto; z-index: 100; width: 40px; transition: width 0.2s ease; overflow: hidden; }
    #controls.expanded { width: 320px; padding: 15px; }
    #controls.collapsed .filter-group, #controls.collapsed #search, #controls.collapsed #legend { display: none; }
    #toggleBtn { position: absolute; top: 10px; left: 10px; width: 20px; height: 20px; cursor: pointer; font-size: 14px; line-height: 20px; color: #666; }
    #search { width: 100%; padding: 8px; margin-bottom: 10px; border: 1px solid #ddd; border-radius: 4px; font-size: 13px; box-sizing: border-box; display: none; }
    #controls.expanded #search { display: block; }
    h3 { margin: 0 0 10px 0; font-size: 14px; color: #333; }
    .filter-group { margin-bottom: 15px; }
    .filter-group label { display: block; font-size: 12px; margin: 3px 0; cursor: pointer; padding: 2px 4px; border-radius: 3px; }
    .filter-group label:hover { background: #f0f0f0; }
    .count { color: #888; font-size: 11px; }
    svg { cursor: grab; }
    svg:active { cursor: grabbing; }
    .node circle { stroke: #fff; stroke-width: 2px; }
    .node text { font-size: 12px; pointer-events: none; fill: #333; font-weight: 500; display: none; paint-order: stroke; stroke: white; stroke-width: 3px; }
    .link { stroke: #999; stroke-opacity: 0.6; stroke-width: 1.5px; }
    .link.faded { stroke-opacity: 0.1; }
    .node.faded { opacity: 0.15; }
    .node text.visible { display: block; }
    .node.highlighted circle { stroke: #000; stroke-width: 3px; }
    .node.neighbor text.visible { display: block; }
    #stats { font-size: 12px; color: #666; margin-bottom: 10px; display: none; }
    #controls.expanded #stats { display: block; }
    #legend { margin-top: 15px; padding-top: 10px; border-top: 1px solid #eee; display: none; }
    #controls.expanded #legend { display: block; }
    .legend-item { display: flex; align-items: center; font-size: 11px; margin: 4px 0; }
    .legend-color { width: 12px; height: 12px; border-radius: 50%; margin-right: 8px; }
    #title { font-size: 16px; font-weight: 600; margin-bottom: 8px; color: #222; display: none; }
    #controls.expanded #title { display: block; }
    .view-toggle { display: none; margin-bottom: 10px; }
    #controls.expanded .view-toggle { display: flex; gap: 10px; }
    .view-toggle label { font-size: 12px; cursor: pointer; }
    #isolateToggle { display: none; margin-top: 10px; padding-top: 10px; border-top: 1px solid #eee; }
    #controls.expanded #isolateToggle { display: block; }
    #titleBar { display: flex; align-items: center; justify-content: space-between; }
  </style>
</head>
<body>
  <div id="controls" class="collapsed">
    <div id="toggleBtn">☰</div>
    <div id="titleBar"><div id="title">Ontology Explorer</div></div>
    <div id="stats"></div>
    <input type="text" id="search" placeholder="Search nodes...">
    <div class="view-toggle">
      <label><input type="radio" name="viewMode" value="all" checked> All</label>
      <label><input type="radio" name="viewMode" value="namespace"> By Namespace</label>
    </div>
    <div class="filter-group">
      <h3>Node Types</h3>
      <div id="nodeTypes"></div>
    </div>
    <div class="filter-group">
      <h3>Relationships</h3>
      <div id="predicates"></div>
    </div>
    <div class="filter-group">
      <h3>Namespaces</h3>
      <div id="namespaces"></div>
    </div>
    <div id="isolateToggle">
      <label><input type="checkbox" id="isolateMode"> Isolate (hide faded)</label>
    </div>
    <div id="legend">
      <h3>Legend</h3>
      <div id="legendItems"></div>
    </div>
  </div>
  <div id="graph"></div>
  <script>
    const width = window.innerWidth;
    const height = window.innerHeight;
    let graphData = { nodes: [], edges: [] };
    let simulation;
    let svg, g, link, node;
    let allNodes = [];
    let allEdges = [];
    let selectedNode = null;
    let currentZoom = 1;
    let highlightMode = 0;
    let highlightedNodes = new Set();
    let highlightedEdges = new Set();

    const colorMap = {
      class: "#22c55e",
      objectProperty: "#3b82f6",
      datatypeProperty: "#8b5cf6",
      annotationProperty: "#f97316",
      resource: "#64748b",
      literal: "#a855f7"
    };

    const edgeColorMap = {
      "rdfs:domain": "#ef4444",
      "rdfs:range": "#f59e0b",
      "rdfs:subClassOf": "#22c55e",
      "rdfs:subPropertyOf": "#3b82f6",
      "owl:inverseOf": "#8b5cf6",
      "owl:equivalentClass": "#ec4899",
      "owl:equivalentProperty": "#ec4899"
    };

    const dashedPredicates = new Set(["rdfs:domain", "rdfs:range"]);

    d3.json("../api/graph").then(data => {
      graphData = data;
      allNodes = [...data.nodes];
      allEdges = [...data.edges];
      setupControls();
      setupLegend();
      renderGraph();
    });

    function setupControls() {
      document.getElementById('toggleBtn').addEventListener('click', () => {
        const c = document.getElementById('controls');
        c.classList.toggle('collapsed');
        c.classList.toggle('expanded');
        document.getElementById('toggleBtn').textContent = c.classList.contains('expanded') ? '✕' : '☰';
      });

      const nodeTypeCounts = {};
      graphData.nodes.forEach(n => {
        const type = n.type || "untyped";
        nodeTypeCounts[type] = (nodeTypeCounts[type] || 0) + 1;
      });

      const predCounts = {};
      graphData.edges.forEach(e => { predCounts[e.label] = (predCounts[e.label] || 0) + 1; });

      const nsCounts = {};
      graphData.nodes.forEach(n => { nsCounts[n.namespace] = (nsCounts[n.namespace] || 0) + 1; });

      document.getElementById('stats').innerHTML =
        graphData.nodes.length + ' nodes, ' + graphData.edges.length + ' edges';

      const nodeDiv = document.getElementById('nodeTypes');
      Object.entries(nodeTypeCounts).sort((a,b) => b[1] - a[1]).forEach(([type, count]) => {
        const label = type === "untyped" ? "(untyped)" : type.replace(/([A-Z])/g, ' $1').trim();
        nodeDiv.innerHTML += '<label><input type="checkbox" checked data-nodetype="' + type + '"> ' + label + ' <span class="count">(' + count + ')</span></label>';
      });

      const predDiv = document.getElementById('predicates');
      Object.entries(predCounts).sort((a,b) => b[1] - a[1]).forEach(([pred, count]) => {
        predDiv.innerHTML += '<label><input type="checkbox" checked data-predicate="' + pred + '"> ' + pred + ' <span class="count">(' + count + ')</span></label>';
      });

      const nsDiv = document.getElementById('namespaces');
      Object.entries(nsCounts).sort((a,b) => b[1] - a[1]).forEach(([ns, count]) => {
        const shortNs = getShortNamespace(ns);
        const title = ns !== shortNs ? 'title="' + ns + '"' : '';
        nsDiv.innerHTML += '<label ' + title + '><input type="checkbox" checked data-namespace="' + ns + '"> ' + shortNs + ' <span class="count">(' + count + ')</span></label>';
      });

      document.querySelectorAll('#nodeTypes input, #predicates input, #namespaces input').forEach(cb => {
        cb.addEventListener('change', updateGraph);
      });

      document.getElementById('search').addEventListener('input', debounce(updateGraph, 300));

      document.querySelectorAll('input[name="viewMode"]').forEach(rb => {
        rb.addEventListener('change', (e) => {
          if (e.target.value === 'namespace') {
            document.querySelectorAll('#namespaces input').forEach(cb => cb.checked = false);
            if (graphData.namespaces.length > 0) {
              document.querySelector('#namespaces input[data-namespace="' + graphData.namespaces[0] + '"]').checked = true;
            }
          } else {
            document.querySelectorAll('#namespaces input').forEach(cb => cb.checked = true);
          }
          updateGraph();
        });
      });

      document.getElementById('isolateMode').addEventListener('change', updateGraph);
    }

    function setupLegend() {
      const legendDiv = document.getElementById('legendItems');
      Object.entries(colorMap).forEach(([type, color]) => {
        if (graphData.types.includes(type)) {
          const label = type.replace(/([A-Z])/g, ' $1').trim();
          legendDiv.innerHTML += '<div class="legend-item"><div class="legend-color" style="background:' + color + '"></div>' + label + '</div>';
        }
      });
    }

    function getShortNamespace(ns) {
      if (!ns || ns === "_") return "blank node";
      const lastHash = ns.lastIndexOf('#');
      const lastSlash = ns.lastIndexOf('/');
      const sep = Math.max(lastHash, lastSlash);
      if (sep > 0 && sep < ns.length - 1) {
        return ns.substring(sep + 1) || ns;
      }
      return ns;
    }

    function debounce(fn, ms) {
      let timer;
      return function(...args) {
        clearTimeout(timer);
        timer = setTimeout(() => fn.apply(this, args), ms);
      };
    }

    function getFilters() {
      const enabledNodeTypes = new Set();
      document.querySelectorAll('#nodeTypes input:checked').forEach(cb => enabledNodeTypes.add(cb.dataset.nodetype));

      const enabledPreds = new Set();
      document.querySelectorAll('#predicates input:checked').forEach(cb => enabledPreds.add(cb.dataset.predicate));

      const enabledNS = new Set();
      document.querySelectorAll('#namespaces input:checked').forEach(cb => enabledNS.add(cb.dataset.namespace));

      const search = document.getElementById('search').value.toLowerCase();

      return { enabledNodeTypes, enabledPreds, enabledNS, search };
    }

    function getNeighbors(nodeId, hops) {
      const neighbors = new Set();
      const visited = new Set([nodeId]);
      let current = [nodeId];

      for (let h = 0; h < hops; h++) {
        const next = [];
        current.forEach(id => {
          allEdges.forEach(e => {
            if (e.source === id && !visited.has(e.target)) {
              visited.add(e.target);
              neighbors.add(e.target);
              next.push(e.target);
            }
            if (e.target === id && !visited.has(e.source)) {
              visited.add(e.source);
              neighbors.add(e.source);
              next.push(e.source);
            }
          });
        });
        current = next;
      }
      return neighbors;
    }

    function shouldShowLabel(d) {
      if (selectedNode && highlightedNodes.has(d.id)) return true;
      if (currentZoom > 1.5) return true;
      return false;
    }

    function renderGraph() {
      document.getElementById('graph').innerHTML = '';
      const { enabledNodeTypes, enabledPreds, enabledNS, search } = getFilters();
      const isolateMode = document.getElementById('isolateMode').checked;

      let nodes, edges;
      const allNodeIds = new Set(allNodes.map(n => n.id));

      if (search) {
        nodes = allNodes.filter(n => n.label.toLowerCase().includes(search));
        const nodeIds = new Set(nodes.map(n => n.id));
        edges = allEdges.filter(e => nodeIds.has(e.source) || nodeIds.has(e.target));
      } else {
        nodes = allNodes.filter(n => {
          const type = n.type || "untyped";
          return enabledNodeTypes.has(type) && enabledNS.has(n.namespace);
        });
        const nodeIds = new Set(nodes.map(n => n.id));

        edges = allEdges.filter(e => {
          if (!enabledPreds.has(e.label)) return false;
          const sourceInFiltered = nodeIds.has(e.source);
          const targetInFiltered = nodeIds.has(e.target);
          const sourceExists = allNodeIds.has(e.source);
          const targetExists = allNodeIds.has(e.target);
          return (sourceInFiltered && targetExists) || (targetInFiltered && sourceExists) || (sourceInFiltered && targetInFiltered);
        });
      }

      if (highlightMode > 0 && selectedNode) {
        const neighbors = getNeighbors(selectedNode, highlightMode);
        highlightedNodes = new Set([selectedNode, ...neighbors]);
        highlightedEdges = new Set();
        edges.forEach(e => {
          if (highlightedNodes.has(e.source) && highlightedNodes.has(e.target)) {
            highlightedEdges.add(e.id || e.source + '-' + e.target);
          }
        });
      } else {
        highlightedNodes = new Set();
        highlightedEdges = new Set();
      }

      if (highlightedNodes.size > 0) {
        if (isolateMode) {
          nodes = nodes.filter(n => highlightedNodes.has(n.id));
          edges = edges.filter(e => highlightedEdges.has(e.id || e.source + '-' + e.target));
        }
      }

      document.getElementById('stats').innerHTML = nodes.length + ' nodes, ' + edges.length + ' edges';

      svg = d3.select("#graph").append("svg")
        .attr("width", width).attr("height", height)
        .call(d3.zoom().on("zoom", (event) => {
          currentZoom = event.transform.k;
          g.attr("transform", event.transform);
          node.selectAll("text").classed("visible", d => shouldShowLabel(d));
        }));

      g = svg.append("g");

      link = g.selectAll(".link").data(edges).enter().append("line")
        .attr("class", "link")
        .attr("stroke", d => edgeColorMap[d.label] || "#999")
        .attr("stroke-dasharray", d => dashedPredicates.has(d.label) ? "5,5" : "none");

      node = g.selectAll(".node").data(nodes).enter().append("g")
        .attr("class", "node")
        .call(d3.drag().on("start", dragstarted).on("drag", dragged).on("end", dragended))
        .on("click", (event, d) => {
          event.stopPropagation();
          selectedNode = d.id;
          highlightMode = 1;
          renderGraph();
        })
        .on("dblclick", (event, d) => {
          event.stopPropagation();
          selectedNode = d.id;
          highlightMode = 2;
          renderGraph();
        });

      node.append("circle").attr("r", 10).attr("fill", d => colorMap[d.type] || "#999");

      node.append("text").attr("dx", 12).attr("dy", ".35em")
        .text(d => d.label)
        .classed("visible", d => shouldShowLabel(d));

      node.append("title").text(d => d.label + '\\n' + d.id + '\\nType: ' + d.type);

      link.append("title").text(d => d.label + '\\n' + d.source + ' → ' + d.target);

      node.classed("highlighted", d => d.id === selectedNode)
          .classed("neighbor", d => d.id !== selectedNode && highlightedNodes.has(d.id));

      if (highlightedNodes.size > 0) {
        const isIsolated = isolateMode;
        node.classed("faded", d => !highlightedNodes.has(d.id) && !isIsolated);
        link.classed("faded", d => !highlightedEdges.has(d.id || d.source + '-' + d.target));
      }

      simulation = d3.forceSimulation(nodes)
        .force("link", d3.forceLink(edges).id(d => d.id).distance(120))
        .force("charge", d3.forceManyBody().strength(-150))
        .force("center", d3.forceCenter(width / 2, height / 2))
        .force("collision", d3.forceCollide().radius(25))
        .on("tick", () => {
          link.attr("x1", d => d.source.x).attr("y1", d => d.source.y)
              .attr("x2", d => d.target.x).attr("y2", d => d.target.y);
          node.attr("transform", d => 'translate(' + d.x + ',' + d.y + ')');
        });

      svg.on("click", () => {
        selectedNode = null;
        highlightMode = 0;
        highlightedNodes = new Set();
        highlightedEdges = new Set();
        node.classed("faded", false).classed("highlighted", false).classed("neighbor", false);
        link.classed("faded", false);
      });
    }

    function updateGraph() {
      if (simulation) simulation.stop();
      renderGraph();
    }

    function dragstarted(event, d) {
      if (!event.active) simulation.alphaTarget(0.3).restart();
      d.fx = d.x; d.fy = d.y;
    }
    function dragged(event, d) { d.fx = event.x; d.fy = event.y; }
    function dragended(event, d) {
      if (!event.active) simulation.alphaTarget(0);
      d.fx = null; d.fy = null;
    }
  </script>
</body>
</html>`

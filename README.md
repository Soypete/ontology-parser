# ontology-parser

A Go library for parsing RDF/OWL/TTL ontology files and generating RDF triples from relational data.

## Overview

ontology-parser provides tools for working with RDF ontologies, including:

- **Parsing**: Load RDF/OWL/TTL ontology files
- **Storage**: In-memory triple store with named graph support
- **SPARQL**: Query triples using SPARQL
- **TBOX/ABOX Mapping**: Map relational database tables to RDF triples

## TBOX and ABOX Mapping

This library supports the two-layer ontology mapping pattern common in knowledge representation:

### TBOX (Terminological Box)

TBOX defines the schema/ontology - the concepts and relationships. Use the library to:

- Parse ontology files (OWL, TTL, RDF/XML) to extract class hierarchies and properties
- Define mappings from database columns to ontology predicates
- Generate RDF triples representing schema-level knowledge

Example TBOX mapping:
```yaml
mappings:
  - table: product_categories
    triples:
      - subject: "https://example.org/category/{id}"
        predicate: "rdf:type"
        object: "schema:ProductCategory"
      - subject: "https://example.org/category/{id}"
        predicate: "rdfs:label"
        object: "{name}"
```

### ABOX (Assertional Box)

ABOX contains the actual assertions/instances - data about individuals. The library:

- Maps database rows to RDF individuals
- Generates instance triples from relational data
- Supports inference through ontology-based reasoning

Example ABOX mapping:
```yaml
mappings:
  - table: products
    graph: "https://example.org/data/products"
    triples:
      - subject: "https://example.org/product/{sku}"
        predicate: "rdf:type"
        object: "schema:Product"
      - subject: "https://example.org/product/{sku}"
        predicate: "schema:name"
        object: "{name}"
        datatype: "xsd:string"
      - subject: "https://example.org/product/{sku}"
        predicate: "schema:price"
        object: "{price}"
        datatype: "xsd:decimal"
```

## Installation

```bash
go get github.com/soypete/ontology-go
```

## Usage

### Parse Ontology Files

```go
package main

import (
    "os"
    "github.com/soypete/ontology-go/rdf"
)

func main() {
    f, _ := os.Open("ontology.ttl")
    defer f.Close()
    
    parser := rdf.NewXMLParser("https://example.org/graph")
    triples, err := parser.Parse(f)
    // Handle error...
}
```

### Store and Query Triples

```go
store := store.NewMemoryStore()

store.Register("graph1", []types.Triple{
    {Subject: "https://example.org/item1", Predicate: "rdf:type", Object: "schema:Product"},
})

results := store.Match("", "rdf:type", "schema:Product")
```

## SPARQL Query Engine

The `sparql` package provides an in-memory SPARQL query engine for querying triples stored in a `store.Store`.

### Quick Start

```go
import (
    "github.com/soypete/ontology-go/sparql/query"
    "github.com/soypete/ontology-go/store"
    "github.com/soypete/ontology-go/types"
)

// Create a store and register triples
s := store.NewMemoryStore()
s.Register("test", []types.Triple{
    {Subject: "http://example.org/alice", Predicate: "http://xmlns.com/foaf/0.1/name", Object: "Alice"},
    {Subject: "http://example.org/bob", Predicate: "http://xmlns.com/foaf/0.1/name", Object: "Bob"},
})

// Create query engine
engine := query.NewEngine(s)

// Execute SPARQL query
result, err := engine.Execute(`
    PREFIX foaf: <http://xmlns.com/foaf/0.1/>
    SELECT ?person ?name WHERE {
        ?person foaf:name ?name .
    }
`)
```

### Supported Features

#### Basic SPARQL
- `SELECT` queries with `WHERE` clause
- Variables: `?var` or `$var` syntax
- PREFIX declarations for namespace abbreviation
- `a` shorthand for `rdf:type`
- `LIMIT` and `OFFSET` for pagination
- `DISTINCT` to remove duplicate results

#### Triple Patterns
- Basic Graph Patterns (BGP) - multiple triple patterns in WHERE clause
- JOINs via shared variables (e.g., `?person` in multiple patterns)
- OPTIONAL patterns for optional matching

#### Filters
- Equality: `FILTER (?x = "value")`
- Inequality: `FILTER (?x != "value")`
- Regex: `FILTER (regex(?x, "pattern"))`

#### Supported Filter Functions

**String Functions:**
| Function | Description | Example |
|----------|-------------|---------|
| `contains(?x, "sub")` | Check if string contains substring | `FILTER(contains(?name, "Alice"))` |
| `startsWith(?x, "pre")` | Check if string starts with prefix | `FILTER(startsWith(?name, "A"))` |
| `endsWith(?x, "suf")` | Check if string ends with suffix | `FILTER(endsWith(?name, "son"))` |
| `strStarts(?x, "pre")` | Alias for startsWith | |
| `strEnds(?x, "suf")` | Alias for endsWith | |
| `lcase(?x)` | Compare lowercase | `FILTER(lcase(?name) = "alice")` |
| `ucase(?x)` | Compare uppercase | `FILTER(ucase(?name) = "ALICE")` |
| `replace(?x, "a", "b")` | Check if replace changes string | |
| `str(?x)` | String coercion | |
| `substr(?x, start, len)` | Substring extraction | |
| `strlen(?x)` | Non-empty string check | |

**Type Functions:**
| Function | Description | Example |
|----------|-------------|---------|
| `isURI(?x)` | Check if value is HTTP/HTTPS/URN URI | `FILTER(isURI(?s))` |
| `isLiteral(?x)` | Check if value is a literal | `FILTER(isLiteral(?o))` |
| `isBlank(?x)` | Check if value is blank node | |
| `lang(?x)` | Language tag extraction | |
| `datatype(?x)` | Datatype extraction | |

**Date/Time Functions:**
| Function | Description |
|----------|-------------|
| `year(?x)` | Extract year from date |
| `month(?x)` | Extract month from date |
| `day(?x)` | Extract day from date |
| `hours(?x)` | Extract hours from time |
| `minutes(?x)` | Extract minutes from time |
| `seconds(?x)` | Extract seconds from time |

**Container Membership (Facade-X compatibility):**
| Function/Property | Description |
|-------------------|-------------|
| `fx:anySlot` | Magic property matching any `rdf:_N` (e.g., rdf:_1, rdf:_2) |
| `fx:cardinal(?x)` | Extract cardinal number from rdf:_N |

#### Aggregates
- `COUNT`, `SUM`, `MIN`, `MAX`, `AVG` with `GROUP BY`
- DISTINCT support in aggregates

### Design

#### Query Optimization
The engine applies query optimization to reduce memory usage for simple queries:
- Fixed values in patterns (e.g., `?s foaf:name "Alice"`) are used to pre-filter triples
- Multi-pattern queries with different predicates fall back to full scan (JOINs)
- Filter is applied after SKOS inference to preserve inference correctness

#### SKOS Inference
The engine supports SKOS (Simple Knowledge Organization System) inference:
- `skos:broader` / `skos:narrower` transitive inference
- `skos:related` symmetric inference
- `skos:exactMatch` / `skos:closeMatch` equivalence inference
- Authority-based matching for cross-dataset queries

### Limitations

The SPARQL engine is designed for in-memory querying and has the following limitations:

1. **Query Types**: Only `SELECT` queries are supported. `CONSTRUCT`, `ASK`, `DESCRIBE` are not implemented.

2. **Complex Patterns**: 
   - No `UNION` patterns
   - No property paths (e.g., `?s foaf:knows* ?o`)
   - No `BIND` for variable assignment
   - No `VALUES` for inline data
   - No subqueries

3. **Filter Expressions**:
   - Limited logical operators (only basic `=`, `!=`, `regex` supported)
   - No `&&`, `||`, `!` in filters
   - No comparison operators (`<`, `>`, `<=`, `>=`)
   - No parentheses in filter expressions
   - String functions work in filter context but don't transform results

4. **Data Types**:
   - No explicit XSD datatype handling
   - Language tags not fully supported
   - Blank nodes treated as regular URIs

5. **Performance**:
   - Full in-memory scan for complex JOINs
   - No query planning or cost-based optimization
   - Large datasets may cause memory pressure

### Comparison with SPARQL Anything

This engine is inspired by [SPARQL Anything](https://sparql-anything.readthedocs.io/) but differs in focus:

| Feature | SPARQL Anything | ontology-go |
|---------|-----------------|-------------|
| Primary Use | Query non-RDF files with SPARQL | Query in-memory RDF triples |
| Data Source | Files, HTTP, commands | Store interface |
| Facade-X | Full implementation | Partial (container membership) |
| Inference | None | SKOS inference |
| Optimization | On-disk option | In-memory filtering |

## Modules

- `rdf` - RDF/OWL/TTL parsing
- `store` - Triple storage with named graphs
- `sparql` - SPARQL query support
- `ttl` - Turtle format parsing
- `fetch` - Remote ontology fetching
- `types` - Core RDF type definitions

## License

MIT
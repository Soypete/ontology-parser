# OWL Reasoning Package

The `reasoning` package provides OWL reasoning capabilities for RDF ontologies. It supports loading TBox (schema) and ABox (instance) data, performing inference to derive implicit relationships, and supports up to 2-hop node traversal inference.

## Features

- **TBox Loading**: Load ontology schema files (classes, properties)
- **ABox Loading**: Load instance data
- **Class Hierarchy Inference**: Infer `rdfs:subClassOf` relationships
- **Property Hierarchy Inference**: Infer `rdfs:subPropertyOf` relationships
- **2-hop Node Traversal**: Find related entities within configurable hops

## Installation

```bash
go get github.com/soypete/ontology-go/reasoning
```

## Quick Start

```go
package main

import (
    "context"
    "log/slog"

    "github.com/soypete/ontology-go/reasoning"
    "github.com/soypete/ontology-go/store"
)

func main() {
    // Create a triple store
    s := store.NewMemoryStore()

    // Create reasoner with options
    r := reasoning.New(s,
        reasoning.WithLogger(slog.Default()),
        reasoning.WithMaxHops(2),
    )

    // Load TBox (schema)
    err := r.LoadTBox(context.Background(), "schema.ttl")
    if err != nil {
        log.Fatal(err)
    }

    // Load ABox (instances)
    err = r.LoadABox(context.Background(), "data.ttl")
    if err != nil {
        log.Fatal(err)
    }

    // Perform reasoning
    err = r.Reason(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    // Get inferred triples
    inferred := r.Inferred()
    log.Info("inferred triples", "count", len(inferred))
}
```

## Loading Data

### From Files

```go
// Load TBox from file
err := r.LoadTBox(ctx, "path/to/schema.ttl")

// Load ABox from file
err := r.LoadABox(ctx, "path/to/instances.ttl")
```

### From Code

```go
import "github.com/soypete/ontology-go/types"

// Load TBox from triples
triples := []types.Triple{
    {Subject: "ex:Animal", Predicate: types.RDFType, Object: types.RDFSClass},
    {Subject: "ex:Dog", Predicate: types.RDFSSubClassOf, Object: "ex:Animal"},
}
err := r.LoadTBoxData(ctx, triples)

// Load ABox from triples
triples = []types.Triple{
    {Subject: "ex:fido", Predicate: types.RDFType, Object: "ex:Dog"},
}
err = r.LoadABoxData(ctx, triples)
```

## Inference

### Class Hierarchy

The reasoner infers transitive `rdfs:subClassOf` relationships:

```
TBox: ex:SportsCar -> ex:Car -> ex:Vehicle
Inferred: ex:SportsCar -> ex:Vehicle
```

### Property Hierarchy

The reasoner infers transitive `rdfs:subPropertyOf` relationships:

```
TBox: ex:hasGrandparent -> ex:hasParent -> ex:hasAncestor
Inferred: ex:hasGrandparent -> ex:hasAncestor
```

## Querying Inferred Data

### Get All Inferred Triples

```go
inferred := r.Inferred()
```

### Get Inferred by Subject

```go
triples := r.GetInferredBySubject("ex:Dog")
```

### Find Related Entities (2-hop traversal)

```go
// Find all triples related to ex:alice within 2 hops
related := r.FindRelated(ctx, "ex:alice", 2)
```

## Options

```go
// Custom logger
reasoning.WithLogger(myLogger)

// Custom max hops (default is 2)
reasoning.WithMaxHops(3)
```

## Package Constants

The package uses named graphs:
- `tbox` - TBox data
- `abox` - ABox data  
- `inferred` - Inferred triples

## See Also

- [Examples](./examples/basic.go)
- [Store Package](../store/README.md)
- [TTL Parser](../ttl/README.md)
- [Types Package](../types/README.md)
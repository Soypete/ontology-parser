package skosast

import (
	"strings"
	"testing"

	"github.com/soypete/ontology-go/ttlast"
)

func TestBuildFromDocument(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/transitive_broader.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	if len(hierarchy.Concepts) == 0 {
		t.Error("Expected concepts in hierarchy")
	}

	onlineCourse := hierarchy.GetConcept("http://example.org/onlineCourse")
	if onlineCourse == nil {
		t.Error("Expected to find onlineCourse concept")
		return
	}

	if len(onlineCourse.Broader) != 1 {
		t.Errorf("Expected 1 broader relation, got %d", len(onlineCourse.Broader))
	}

	if onlineCourse.Broader[0] != "http://example.org/course" {
		t.Errorf("Expected broader to be http://example.org/course, got %s", onlineCourse.Broader[0])
	}
}

func TestGetAllBroaderTransitive(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/transitive_broader.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	allBroader := hierarchy.GetAllBroaderTransitive("http://example.org/selfPaced")

	foundCourse := false
	for _, b := range allBroader {
		if b == "http://example.org/course" {
			foundCourse = true
			break
		}
	}

	if !foundCourse {
		t.Error("Expected selfPaced to have transitive broader relation to course")
	}
}

func TestHasCycle(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/cycle_detection.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	hasCycle, _ := hierarchy.HasCycle()
	if !hasCycle {
		t.Error("Expected cycle detection to return true")
	}
}

func TestNoCycle(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/transitive_broader.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	hasCycle, _ := hierarchy.HasCycle()
	if hasCycle {
		t.Error("Expected no cycle in transitive_broader.ttl")
	}
}

func TestFindInconsistencies(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/inconsistency_broader_narrower.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	inconsistencies := hierarchy.FindInconsistencies()
	if len(inconsistencies) == 0 {
		t.Error("Expected to find inconsistency")
	}

	found := false
	for _, msg := range inconsistencies {
		if strings.Contains(msg, "both broader and narrower") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find broader/narrower inconsistency")
	}
}

func TestSymmetricRelated(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/symmetric_related.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	math := hierarchy.GetConcept("http://example.org/math")
	if math == nil {
		t.Fatal("Expected to find math concept")
	}

	found := false
	for _, r := range math.Related {
		if r == "http://example.org/science" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected math to be related to science")
	}
}

func TestPreOrder(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/transitive_broader.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	nodes := PreOrder(hierarchy, hierarchy.Concepts["http://example.org/selfPaced"])
	if len(nodes) == 0 {
		t.Error("Expected nodes in pre-order traversal")
	}
}

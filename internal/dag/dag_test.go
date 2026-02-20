// SPDX-License-Identifier: MPL-2.0

package dag

import (
	"errors"
	"slices"
	"testing"
)

func TestTopologicalSort_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := New()
	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order != nil {
		t.Errorf("expected nil, got %v", order)
	}
}

func TestTopologicalSort_SingleNode(t *testing.T) {
	t.Parallel()
	g := New()
	g.AddNode("A")
	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(order, []string{"A"}) {
		t.Errorf("expected [A], got %v", order)
	}
}

func TestTopologicalSort_LinearChain(t *testing.T) {
	t.Parallel()
	g := New()
	// A -> B -> C (A must run first, then B, then C)
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"A", "B", "C"}
	if !slices.Equal(order, expected) {
		t.Errorf("expected %v, got %v", expected, order)
	}
}

func TestTopologicalSort_Diamond(t *testing.T) {
	t.Parallel()
	g := New()
	// A -> B, A -> C, B -> D, C -> D
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("B", "D")
	g.AddEdge("C", "D")

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A must be first, D must be last
	if order[0] != "A" {
		t.Errorf("expected A first, got %v", order)
	}
	if order[len(order)-1] != "D" {
		t.Errorf("expected D last, got %v", order)
	}
	if len(order) != 4 {
		t.Errorf("expected 4 nodes, got %d: %v", len(order), order)
	}
}

func TestTopologicalSort_SimpleCycle(t *testing.T) {
	t.Parallel()
	g := New()
	g.AddEdge("A", "B")
	g.AddEdge("B", "A")

	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("expected *CycleError, got %T: %v", err, err)
	}
	if len(cycleErr.Cycle) < 2 {
		t.Errorf("expected at least 2 nodes in cycle, got %v", cycleErr.Cycle)
	}
}

func TestTopologicalSort_SelfLoop(t *testing.T) {
	t.Parallel()
	g := New()
	g.AddEdge("A", "A")

	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("expected *CycleError, got %T: %v", err, err)
	}
}

func TestTopologicalSort_ComplexCycle(t *testing.T) {
	t.Parallel()
	g := New()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A")

	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("expected *CycleError, got %T: %v", err, err)
	}
	if len(cycleErr.Cycle) < 3 {
		t.Errorf("expected at least 3 nodes in cycle, got %v", cycleErr.Cycle)
	}
}

func TestTopologicalSort_DisconnectedComponents(t *testing.T) {
	t.Parallel()
	g := New()
	g.AddEdge("A", "B")
	g.AddNode("C")
	g.AddNode("D")

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Errorf("expected 4 nodes, got %d: %v", len(order), order)
	}
	// A must come before B
	aIdx := slices.Index(order, "A")
	bIdx := slices.Index(order, "B")
	if aIdx >= bIdx {
		t.Errorf("A (idx %d) must come before B (idx %d) in %v", aIdx, bIdx, order)
	}
}

func TestTopologicalSort_DuplicateEdges(t *testing.T) {
	t.Parallel()
	g := New()
	g.AddEdge("A", "B")
	g.AddEdge("A", "B") // duplicate

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still work — duplicates just increase in-degree but Kahn's handles it.
	if !slices.Equal(order, []string{"A", "B"}) {
		t.Errorf("expected [A, B], got %v", order)
	}
}

func TestCycleError_Message(t *testing.T) {
	t.Parallel()
	err := &CycleError{Cycle: []string{"A", "B", "C"}}
	expected := "dependency cycle detected: A -> B -> C"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

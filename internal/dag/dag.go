// SPDX-License-Identifier: MPL-2.0

// Package dag provides directed acyclic graph operations for topological sorting
// and cycle detection. It is used by the command execution pipeline to order
// depends_on.cmds entries that have execute: true.
package dag

import (
	"fmt"
	"strings"
)

type (
	// CycleError indicates that the graph contains a cycle, preventing topological ordering.
	CycleError struct {
		// Cycle contains the nodes that form the cycle (not necessarily all of them,
		// but enough to identify the problem).
		Cycle []string
	}

	// Graph is a directed graph for topological sorting.
	// Nodes are identified by string keys. Edges represent "must run before" relationships:
	// an edge from A to B means A must complete before B starts.
	Graph struct {
		// adjacency maps each node to its outgoing neighbors (nodes that depend on it).
		adjacency map[string][]string
		// nodes tracks all nodes in insertion order for deterministic output.
		nodes []string
		// nodeSet provides O(1) lookup for node existence.
		nodeSet map[string]bool
	}
)

func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected: %s", strings.Join(e.Cycle, " -> "))
}

// New creates an empty Graph.
func New() *Graph {
	return &Graph{
		adjacency: make(map[string][]string),
		nodeSet:   make(map[string]bool),
	}
}

// AddNode adds a node to the graph. If the node already exists, this is a no-op.
func (g *Graph) AddNode(name string) {
	if g.nodeSet[name] {
		return
	}
	g.nodeSet[name] = true
	g.nodes = append(g.nodes, name)
}

// AddEdge adds a directed edge from -> to, meaning "from" must run before "to".
// Both nodes are implicitly added if they don't exist.
func (g *Graph) AddEdge(from, to string) {
	g.AddNode(from)
	g.AddNode(to)
	g.adjacency[from] = append(g.adjacency[from], to)
}

// TopologicalSort returns a valid execution order using Kahn's algorithm.
// Returns CycleError if the graph contains a cycle.
// The returned order is deterministic: nodes at the same topological level
// appear in the order they were first added to the graph.
func (g *Graph) TopologicalSort() ([]string, error) {
	if len(g.nodes) == 0 {
		return nil, nil
	}

	// Compute in-degrees.
	inDegree := make(map[string]int, len(g.nodes))
	for _, node := range g.nodes {
		inDegree[node] = 0
	}
	for _, neighbors := range g.adjacency {
		for _, neighbor := range neighbors {
			inDegree[neighbor]++
		}
	}

	// Seed the queue with nodes that have no incoming edges, in insertion order.
	queue := make([]string, 0)
	for _, node := range g.nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, neighbor := range g.adjacency[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(g.nodes) {
		// Remaining nodes with non-zero in-degree form the cycle.
		var cycleNodes []string
		for _, node := range g.nodes {
			if inDegree[node] > 0 {
				cycleNodes = append(cycleNodes, node)
			}
		}
		return nil, &CycleError{Cycle: cycleNodes}
	}

	return result, nil
}

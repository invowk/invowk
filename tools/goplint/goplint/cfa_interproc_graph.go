// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

type interprocNodeKind string

const (
	interprocNodeKindCFG    interprocNodeKind = "cfg-node"
	interprocNodeKindCall   interprocNodeKind = "call-site"
	interprocNodeKindReturn interprocNodeKind = "return-site"
)

type interprocNodeID struct {
	FuncKey    string
	BlockIndex int32
	NodeIndex  int
	Kind       interprocNodeKind
}

func (id interprocNodeID) Key() string {
	return fmt.Sprintf("%s|%s|%d|%d", id.FuncKey, id.Kind, id.BlockIndex, id.NodeIndex)
}

type interprocEdgeKind string

const (
	interprocEdgeIntra        interprocEdgeKind = "intra"
	interprocEdgeCall         interprocEdgeKind = "call"
	interprocEdgeReturn       interprocEdgeKind = "return"
	interprocEdgeCallToReturn interprocEdgeKind = "call-to-return"
)

type interprocEdge struct {
	From   interprocNodeID
	To     interprocNodeID
	Kind   interprocEdgeKind
	Reason pathOutcomeReason
}

type interprocSupergraph struct {
	Nodes map[string]interprocNodeID
	Edges []interprocEdge
}

func newInterprocSupergraph() interprocSupergraph {
	return interprocSupergraph{Nodes: make(map[string]interprocNodeID)}
}

func (g *interprocSupergraph) addNode(id interprocNodeID) {
	if g == nil {
		return
	}
	if g.Nodes == nil {
		g.Nodes = make(map[string]interprocNodeID)
	}
	g.Nodes[id.Key()] = id
}

func (g *interprocSupergraph) addEdge(edge interprocEdge) {
	if g == nil {
		return
	}
	g.addNode(edge.From)
	g.addNode(edge.To)
	g.Edges = append(g.Edges, edge)
}

func buildInterprocSupergraphForFunc(pass *analysis.Pass, fnDecl *ast.FuncDecl, backend string) interprocSupergraph {
	graph := newInterprocSupergraph()
	if fnDecl == nil || fnDecl.Body == nil {
		return graph
	}

	cfg := buildFuncCFGForBackend(pass, fnDecl.Body, backend)
	if cfg == nil {
		return graph
	}

	funcKey := fnDecl.Name.Name
	if pass != nil && pass.Pkg != nil {
		funcKey = pass.Pkg.Path() + "." + fnDecl.Name.Name
	}

	blockEntryNode := map[int32]interprocNodeID{}
	for _, block := range cfg.Blocks {
		if block == nil || len(block.Nodes) == 0 {
			continue
		}
		first := interprocNodeID{
			FuncKey:    funcKey,
			BlockIndex: block.Index,
			NodeIndex:  0,
			Kind:       interprocNodeKindCFG,
		}
		blockEntryNode[block.Index] = first
	}

	for _, block := range cfg.Blocks {
		if block == nil || len(block.Nodes) == 0 {
			continue
		}
		for idx := range block.Nodes {
			nodeID := interprocNodeID{
				FuncKey:    funcKey,
				BlockIndex: block.Index,
				NodeIndex:  idx,
				Kind:       interprocNodeKindCFG,
			}
			graph.addNode(nodeID)
			if idx > 0 {
				prev := interprocNodeID{
					FuncKey:    funcKey,
					BlockIndex: block.Index,
					NodeIndex:  idx - 1,
					Kind:       interprocNodeKindCFG,
				}
				graph.addEdge(interprocEdge{From: prev, To: nodeID, Kind: interprocEdgeIntra})
			}

			if !nodeContainsCallExpr(block.Nodes[idx]) {
				continue
			}
			callNode := interprocNodeID{
				FuncKey:    funcKey,
				BlockIndex: block.Index,
				NodeIndex:  idx,
				Kind:       interprocNodeKindCall,
			}
			retNode := interprocNodeID{
				FuncKey:    funcKey,
				BlockIndex: block.Index,
				NodeIndex:  idx,
				Kind:       interprocNodeKindReturn,
			}
			graph.addEdge(interprocEdge{From: nodeID, To: callNode, Kind: interprocEdgeCall})
			graph.addEdge(interprocEdge{
				From:   callNode,
				To:     retNode,
				Kind:   interprocEdgeCallToReturn,
				Reason: pathOutcomeReasonUnresolvedTarget,
			})
			if idx+1 < len(block.Nodes) {
				next := interprocNodeID{
					FuncKey:    funcKey,
					BlockIndex: block.Index,
					NodeIndex:  idx + 1,
					Kind:       interprocNodeKindCFG,
				}
				graph.addEdge(interprocEdge{From: retNode, To: next, Kind: interprocEdgeReturn})
			}
		}

		if len(block.Succs) == 0 {
			continue
		}
		lastNode := interprocNodeID{
			FuncKey:    funcKey,
			BlockIndex: block.Index,
			NodeIndex:  len(block.Nodes) - 1,
			Kind:       interprocNodeKindCFG,
		}
		for _, succ := range block.Succs {
			if succ == nil {
				continue
			}
			entry, ok := blockEntryNode[succ.Index]
			if !ok {
				continue
			}
			graph.addEdge(interprocEdge{From: lastNode, To: entry, Kind: interprocEdgeIntra})
		}
	}

	return graph
}

func nodeContainsCallExpr(node ast.Node) bool {
	containsCall := false
	ast.Inspect(node, func(n ast.Node) bool {
		if containsCall {
			return false
		}
		_, ok := n.(*ast.CallExpr)
		if ok {
			containsCall = true
			return false
		}
		return true
	})
	return containsCall
}

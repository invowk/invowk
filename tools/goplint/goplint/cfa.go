// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/ast"

	gocfg "golang.org/x/tools/go/cfg"
)

// collectDeferredClosureLits scans a function or closure body for deferred
// closures: defer func() { ... }(). Returns the set of *ast.FuncLit nodes
// that are deferred. These closures are guaranteed to execute before the
// enclosing function returns (Go spec), so Validate() calls inside them
// validate the outer function's path — unlike goroutine closures which
// execute concurrently with no ordering guarantee.
//
// The scan is shallow: it finds FuncLit nodes that are directly invoked by
// a DeferStmt. Nested closures inside deferred closures are not collected
// here — they are handled by their own collectDeferredClosureLits call
// when inspectClosureCastsCFA processes them.
func collectDeferredClosureLits(body *ast.BlockStmt) map[*ast.FuncLit]bool {
	if body == nil {
		return nil
	}
	var result map[*ast.FuncLit]bool
	ast.Inspect(body, func(n ast.Node) bool {
		deferStmt, ok := n.(*ast.DeferStmt)
		if !ok {
			return true
		}
		// defer expr() — the deferred expression is always a CallExpr.
		// Check if the call's function is a FuncLit: defer func() { ... }()
		if funcLit, ok := deferStmt.Call.Fun.(*ast.FuncLit); ok {
			if result == nil {
				result = make(map[*ast.FuncLit]bool)
			}
			result[funcLit] = true
		}
		return true
	})
	return result
}

// buildFuncCFG constructs a control-flow graph for a function body using
// conservative mayReturn (all calls may return). Returns nil if body is nil.
//
// The conservative mayReturn stub ensures no feasible paths are pruned —
// correct for validation reachability analysis where we want to detect
// ALL paths where Validate() might be missing.
func buildFuncCFG(body *ast.BlockStmt) *gocfg.CFG {
	if body == nil {
		return nil
	}
	// Conservative: every call may return. Never prunes feasible paths.
	return gocfg.New(body, func(*ast.CallExpr) bool { return true })
}

// findDefiningBlock locates the CFG block containing the given AST node.
// Returns the block and the node's index within Block.Nodes.
// Returns (nil, -1) if the node is not found (e.g., in unreachable code
// that was eliminated from the CFG).
func findDefiningBlock(g *gocfg.CFG, target ast.Node) (*gocfg.Block, int) {
	targetPos := target.Pos()
	targetEnd := target.End()

	for _, block := range g.Blocks {
		for i, node := range block.Nodes {
			if node.Pos() == targetPos && node.End() == targetEnd {
				return block, i
			}
		}
	}
	return nil, -1
}

// nodeSliceContainsValidateCall checks whether any node in the given
// slice contains a varName.Validate() selector call expression.
// Deferred closures in deferredLits are descended into (their Validate
// calls count as outer-path validation); other closures are skipped.
func nodeSliceContainsValidateCall(nodes []ast.Node, varName string, deferredLits map[*ast.FuncLit]bool) bool {
	for _, node := range nodes {
		if containsValidateCall(node, varName, deferredLits) {
			return true
		}
	}
	return false
}

// containsValidateCall checks whether a single AST node or any of its
// descendants contains a varName.Validate() call. Closures (FuncLit) are
// NOT descended into by default — they are analyzed independently with
// their own CFGs, and a Validate() call inside a goroutine closure does
// not guarantee execution before the outer function returns.
//
// Exception: deferred closures (FuncLit nodes in deferredLits) ARE
// descended into. Go guarantees that deferred functions execute before
// the enclosing function returns, so a Validate() call inside a deferred
// closure does validate the outer function's path. This distinguishes
// defer func() { x.Validate() }() (safe) from go func() { x.Validate() }()
// (unsafe).
func containsValidateCall(node ast.Node, varName string, deferredLits map[*ast.FuncLit]bool) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		// Closures: descend into deferred closures (guaranteed
		// to execute before return) but skip goroutine and
		// immediate closures (no execution ordering guarantee).
		if lit, ok := n.(*ast.FuncLit); ok {
			return deferredLits[lit]
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Validate" {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == varName {
			found = true
		}
		return !found
	})
	return found
}

// blockContainsValidateCall checks all nodes in a CFG block for a
// varName.Validate() call. Deferred closures in deferredLits are
// descended into.
func blockContainsValidateCall(block *gocfg.Block, varName string, deferredLits map[*ast.FuncLit]bool) bool {
	return nodeSliceContainsValidateCall(block.Nodes, varName, deferredLits)
}

// hasPathToReturnWithoutValidate performs a depth-first search from the
// defining block (starting after defIdx) through CFG successors. Returns
// true if any path from the cast definition to a return block never passes
// through a Validate() call on varName.
//
// Deferred closures in deferredLits are recognized as containing Validate
// calls when applicable (their execution before return is guaranteed).
//
// Algorithm:
//  1. Check remainder of defBlock.Nodes[defIdx+1:] for Validate call.
//     If found, all paths through this block are validated → return false.
//  2. If defBlock has zero successors (return block) and no Validate in
//     remainder → return true (unvalidated path to return).
//  3. DFS over successors: for each unvisited live block, if it contains
//     Validate → prune (validated). If it's a return block (zero succs) →
//     return true. Otherwise recurse into its successors.
func hasPathToReturnWithoutValidate(
	g *gocfg.CFG,
	defBlock *gocfg.Block,
	defIdx int,
	varName string,
	deferredLits map[*ast.FuncLit]bool,
) bool {
	// Check the remainder of the defining block after the cast.
	remainder := defBlock.Nodes[defIdx+1:]
	if nodeSliceContainsValidateCall(remainder, varName, deferredLits) {
		return false // validated in same block after cast
	}

	// If no successors, this is a return block — unvalidated path exists.
	if len(defBlock.Succs) == 0 {
		return true
	}

	// DFS from successors.
	visited := make(map[int32]bool)
	visited[defBlock.Index] = true

	return dfsUnvalidatedPath(defBlock.Succs, varName, visited, deferredLits)
}

// dfsUnvalidatedPath recursively checks whether any path through the given
// successor blocks reaches a return block without encountering a Validate()
// call on varName. Deferred closures in deferredLits are descended into.
func dfsUnvalidatedPath(succs []*gocfg.Block, varName string, visited map[int32]bool, deferredLits map[*ast.FuncLit]bool) bool {
	for _, succ := range succs {
		if visited[succ.Index] {
			continue
		}
		visited[succ.Index] = true

		// Skip dead blocks — unreachable code can't constitute a
		// real execution path.
		if !succ.Live {
			continue
		}

		// If this block contains Validate(), this path is safe.
		if blockContainsValidateCall(succ, varName, deferredLits) {
			continue
		}

		// If this is a return block (no successors), we have an
		// unvalidated path.
		if len(succ.Succs) == 0 {
			return true
		}

		// Recurse into successors.
		if dfsUnvalidatedPath(succ.Succs, varName, visited, deferredLits) {
			return true
		}
	}
	return false
}

// isVarUse reports whether the given AST node contains a "use" of varName
// that is not a display-only or validation call. A use means the variable's
// value is consumed by a non-trivial operation before it is validated.
//
// What counts as a use:
//   - Passing varName as a function argument: useFunc(x)
//   - Method call on varName where the method is not Validate, String,
//     Error, or GoString: x.Setup()
//
// What does NOT count as a use:
//   - x.Validate() — the validation call itself
//   - x.String(), x.Error(), x.GoString() — display-only methods
//
// Closures are NOT descended into (same reasoning as containsValidateCall).
func isVarUse(node ast.Node, varName string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for method call on varName: x.Method(...)
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == varName {
				switch sel.Sel.Name {
				case "Validate", "String", "Error", "GoString":
					return true // display-only or validation — not a use
				default:
					found = true
					return false
				}
			}
		}

		// Check for varName appearing as a function argument.
		for _, arg := range call.Args {
			if ident, ok := arg.(*ast.Ident); ok && ident.Name == varName {
				found = true
				return false
			}
		}

		return true
	})
	return found
}

// hasUseBeforeValidateInBlock checks whether, in the nodes of a block
// starting at startIdx, a "use" of varName appears before a Validate()
// call. Returns true if the variable is used (as an argument or non-display
// method receiver) before Validate() is encountered. Deferred closures
// in deferredLits are recognized when checking for Validate() calls.
//
// Algorithm:
//  1. Scan nodes[startIdx:] in order.
//  2. If a Validate() call on varName is found first → return false (safe).
//  3. If a "use" of varName is found first → return true (UBV detected).
//  4. If neither is found → return false (no use in this block).
func hasUseBeforeValidateInBlock(nodes []ast.Node, startIdx int, varName string, deferredLits map[*ast.FuncLit]bool) bool {
	for i := startIdx; i < len(nodes); i++ {
		node := nodes[i]
		if containsValidateCall(node, varName, deferredLits) {
			return false // Validate() seen first — safe
		}
		if isVarUse(node, varName) {
			return true // use before Validate() — flagged
		}
	}
	return false
}

// hasUseBeforeValidateCrossBlock performs a DFS from the defining block
// through CFG successors to detect uses of varName that occur before
// any Validate() call on that path. Unlike hasUseBeforeValidateInBlock
// which only checks within the defining block, this function checks
// across block boundaries.
//
// The function is only called when hasPathToReturnWithoutValidate returns
// false (all paths DO validate) — the question is whether any path
// uses the variable before reaching the Validate() call.
//
// Algorithm:
//  1. Start from defBlock.Succs (the cast's defining block has already
//     been checked by hasUseBeforeValidateInBlock).
//  2. For each live, unvisited successor block:
//     a. Scan nodes in order: if a use is found before Validate → flag.
//     b. If Validate is found first → prune this path (validated).
//     c. If neither is found → continue DFS to successors.
func hasUseBeforeValidateCrossBlock(
	defBlock *gocfg.Block,
	defIdx int,
	varName string,
	deferredLits map[*ast.FuncLit]bool,
) bool {
	// First check remainder of defBlock for use (same-block already
	// handled) — skip directly to successor blocks.
	// But we need to check if defBlock remainder has validate, which
	// would prune all successor paths.
	remainder := defBlock.Nodes[defIdx+1:]
	if nodeSliceContainsValidateCall(remainder, varName, deferredLits) {
		return false // validated in same block — successors are safe
	}

	if len(defBlock.Succs) == 0 {
		return false // return block — no successors to check
	}

	visited := make(map[int32]bool)
	visited[defBlock.Index] = true

	return dfsUseBeforeValidate(defBlock.Succs, varName, visited, deferredLits)
}

// dfsUseBeforeValidate recursively checks whether any path through
// successor blocks contains a "use" of varName before a "Validate" call.
// Blocks containing Validate() prune their path (downstream is safe).
// Blocks with no use and no Validate continue the DFS.
func dfsUseBeforeValidate(
	succs []*gocfg.Block,
	varName string,
	visited map[int32]bool,
	deferredLits map[*ast.FuncLit]bool,
) bool {
	for _, succ := range succs {
		if visited[succ.Index] {
			continue
		}
		visited[succ.Index] = true

		if !succ.Live {
			continue
		}

		// Scan this block's nodes in order: use vs validate.
		foundUse := false
		foundValidate := false
		for _, node := range succ.Nodes {
			if containsValidateCall(node, varName, deferredLits) {
				foundValidate = true
				break // Validate found first in this block — path is pruned
			}
			if isVarUse(node, varName) {
				foundUse = true
				break // use found before Validate in this block
			}
		}

		if foundUse {
			return true // cross-block UBV detected
		}
		if foundValidate {
			continue // this path is validated — skip successors
		}

		// Neither use nor validate in this block — continue DFS.
		if dfsUseBeforeValidate(succ.Succs, varName, visited, deferredLits) {
			return true
		}
	}
	return false
}

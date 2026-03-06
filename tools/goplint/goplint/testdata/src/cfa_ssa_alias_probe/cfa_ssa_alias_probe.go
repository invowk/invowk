// Package cfa_ssa_alias_probe provides simple fixtures for SSA alias
// unit tests. Used by probe analyzers that inspect SSA output directly.
package cfa_ssa_alias_probe

type ProbeTarget string

func (p ProbeTarget) Validate() error { return nil }

// CopyAlias: y should alias x (both point to same SSA value).
func CopyAlias(raw string) ProbeTarget {
	x := ProbeTarget(raw)
	y := x
	_ = y.Validate()
	return x
}

// ReassignedAlias: y := x; y = other should exclude y from alias set.
func ReassignedAlias(raw1, raw2 string) ProbeTarget {
	x := ProbeTarget(raw1)
	y := x
	y = ProbeTarget(raw2)
	_ = y.Validate()
	return x
}

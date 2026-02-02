// SPDX-License-Identifier: MPL-2.0

package docsaudit

import "time"

// Documentation audit constants.
const (
	// Documentation kinds.
	DocKindReadme  DocumentationKind = "readme"
	DocKindWebsite DocumentationKind = "website"
	DocKindGuide   DocumentationKind = "guide"
	DocKindSample  DocumentationKind = "sample"

	// Surface types.
	SurfaceTypeCommand     SurfaceType = "command"
	SurfaceTypeFlag        SurfaceType = "flag"
	SurfaceTypeConfigField SurfaceType = "config_field"
	SurfaceTypeModule      SurfaceType = "module"
	SurfaceTypeBehavior    SurfaceType = "behavior"

	// Example statuses.
	ExampleStatusValid   ExampleStatus = "valid"
	ExampleStatusInvalid ExampleStatus = "invalid"

	// Mismatch types.
	MismatchTypeMissing      MismatchType = "missing"
	MismatchTypeOutdated     MismatchType = "outdated"
	MismatchTypeIncorrect    MismatchType = "incorrect"
	MismatchTypeInconsistent MismatchType = "inconsistent"

	// Severity levels.
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type (
	// DocumentationKind identifies the kind of documentation source.
	DocumentationKind string

	// DocumentationSource describes a documentation source to audit.
	DocumentationSource struct {
		ID         string            `json:"id"`
		Kind       DocumentationKind `json:"kind"`
		Location   string            `json:"location"`
		ScopeNotes string            `json:"scope_notes"`
	}

	// SurfaceType identifies the user-facing surface type.
	SurfaceType string

	// DocReference describes a specific documentation reference.
	DocReference struct {
		SourceID string `json:"source_id"`
		FilePath string `json:"file_path"`
		Line     int    `json:"line"`
		Snippet  string `json:"snippet"`
	}

	// UserFacingSurface represents a documented user-facing surface.
	UserFacingSurface struct {
		ID                string         `json:"id"`
		Type              SurfaceType    `json:"type"`
		Name              string         `json:"name"`
		SourceLocation    string         `json:"source_location"`
		DocumentationRefs []DocReference `json:"documentation_refs"`
	}

	// ExampleStatus identifies the validation status of an example.
	ExampleStatus string

	// Example describes a documented example and its validation status.
	Example struct {
		ID               string        `json:"id"`
		SourceLocation   string        `json:"source_location"`
		RelatedSurfaceID string        `json:"related_surface_id"`
		Status           ExampleStatus `json:"status"`
		InvalidReason    string        `json:"invalid_reason"`
		OutsideCanonical bool          `json:"outside_canonical"`
	}

	// MismatchType identifies the mismatch category.
	MismatchType string

	// Severity identifies the severity of a finding.
	Severity string

	// Finding represents a single audit finding.
	Finding struct {
		ID               string       `json:"id"`
		MismatchType     MismatchType `json:"mismatch_type"`
		Severity         Severity     `json:"severity"`
		SourceLocation   string       `json:"source_location"`
		ExpectedBehavior string       `json:"expected_behavior"`
		Recommendation   string       `json:"recommendation"`
		RelatedSurfaceID string       `json:"related_surface_id"`
		Summary          string       `json:"summary"`
	}

	// AuditScope defines the scope of a documentation audit.
	AuditScope struct {
		DocSources            []DocumentationSource `json:"doc_sources"`
		Exclusions            []string              `json:"exclusions"`
		Assumptions           []string              `json:"assumptions"`
		CanonicalExamplesPath string                `json:"canonical_examples_path"`
	}

	// Metrics captures aggregate audit metrics.
	Metrics struct {
		TotalSurfaces        int                  `json:"total_surfaces"`
		CoveragePercentage   float64              `json:"coverage_percentage"`
		CountsByMismatchType map[MismatchType]int `json:"counts_by_mismatch_type"`
		CountsBySeverity     map[Severity]int     `json:"counts_by_severity"`
	}

	// AuditReport is the complete documentation audit report.
	AuditReport struct {
		GeneratedAt time.Time             `json:"generated_at"`
		Scope       AuditScope            `json:"scope"`
		Metrics     Metrics               `json:"metrics"`
		Sources     []DocumentationSource `json:"sources"`
		Surfaces    []UserFacingSurface   `json:"surfaces"`
		Findings    []Finding             `json:"findings"`
		Examples    []Example             `json:"examples"`
	}
)

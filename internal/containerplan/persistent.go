// SPDX-License-Identifier: MPL-2.0

// Package containerplan owns pure container execution policies shared by the
// command service and runtime adapter.
package containerplan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/containerargs"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	// PersistentModeEphemeral means a command uses a fresh one-shot container.
	PersistentModeEphemeral PersistentMode = "ephemeral"
	// PersistentModePersistent means a command targets a named persistent container.
	PersistentModePersistent PersistentMode = "persistent"

	// PersistentNameSourceCLI means the persistent container name came from a CLI override.
	PersistentNameSourceCLI PersistentNameSource = "cli"
	// PersistentNameSourceConfig means the persistent container name came from runtime config.
	PersistentNameSourceConfig PersistentNameSource = "config"
	// PersistentNameSourceDerived means Invowk derived the persistent container name.
	PersistentNameSourceDerived PersistentNameSource = "derived"

	persistentContainerNamePrefix = "invowk-"
	persistentContainerHashLen    = 12
)

type (
	//goplint:constant-only
	//
	// PersistentMode classifies whether container execution targets a persistent container.
	PersistentMode string

	//goplint:constant-only
	//
	// PersistentNameSource classifies where a persistent container name came from.
	PersistentNameSource string

	// CommandNamespace is command identity text used for deterministic container names.
	CommandNamespace string

	// PersistentRequestOption configures a persistent planning request.
	PersistentRequestOption func(*PersistentRequest)

	// PersistentPlanOption configures a persistent plan.
	PersistentPlanOption func(*PersistentPlan)

	// PersistentRequest contains the facts needed to plan persistent container targeting.
	PersistentRequest struct {
		commandFullName       *CommandNamespace
		commandName           *CommandNamespace
		invowkfilePath        *invowkfile.FilesystemPath
		containerNameOverride invowkfile.ContainerName
		config                *invowkfile.RuntimePersistentConfig
	}

	// PersistentPlan describes the persistent container target for a command.
	PersistentPlan struct {
		mode            PersistentMode
		name            invowkfile.ContainerName
		nameSource      PersistentNameSource
		createIfMissing bool
	}
)

// NewPersistentRequest creates a validated persistent container planning request.
func NewPersistentRequest(opts ...PersistentRequestOption) (PersistentRequest, error) {
	req := PersistentRequest{}
	for _, opt := range opts {
		if opt != nil {
			opt(&req)
		}
	}
	if err := req.Validate(); err != nil {
		return PersistentRequest{}, err
	}
	return req, nil
}

// WithCommandFullName sets the discovered fully-qualified command namespace.
func WithCommandFullName(commandFullName *CommandNamespace) PersistentRequestOption {
	return func(req *PersistentRequest) {
		req.commandFullName = commandFullName
	}
}

// WithCommandName sets the command's local namespace fallback.
func WithCommandName(commandName *CommandNamespace) PersistentRequestOption {
	return func(req *PersistentRequest) {
		req.commandName = commandName
	}
}

// WithInvowkfilePath sets the invowkfile path used for deterministic names.
func WithInvowkfilePath(invowkfilePath *invowkfile.FilesystemPath) PersistentRequestOption {
	return func(req *PersistentRequest) {
		req.invowkfilePath = invowkfilePath
	}
}

// WithContainerNameOverride sets the CLI persistent container name override.
func WithContainerNameOverride(containerNameOverride invowkfile.ContainerName) PersistentRequestOption {
	return func(req *PersistentRequest) {
		req.containerNameOverride = containerNameOverride
	}
}

// WithConfig sets the runtime persistent container config.
func WithConfig(cfg *invowkfile.RuntimePersistentConfig) PersistentRequestOption {
	return func(req *PersistentRequest) {
		req.config = cfg
	}
}

// Validate returns nil when the planning request contains valid typed fields.
func (r PersistentRequest) Validate() error {
	var errs []error
	if r.commandFullName != nil {
		if err := r.commandFullName.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.commandName != nil {
		if err := r.commandName.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.invowkfilePath != nil {
		if err := r.invowkfilePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.containerNameOverride != "" {
		if err := r.containerNameOverride.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.config != nil {
		if err := r.config.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// NewPersistentPlan creates a validated persistent container plan.
func NewPersistentPlan(opts ...PersistentPlanOption) (PersistentPlan, error) {
	plan := PersistentPlan{}
	for _, opt := range opts {
		if opt != nil {
			opt(&plan)
		}
	}
	if err := plan.Validate(); err != nil {
		return PersistentPlan{}, err
	}
	return plan, nil
}

// WithMode sets whether the plan targets a persistent or ephemeral container.
func WithMode(mode PersistentMode) PersistentPlanOption {
	return func(plan *PersistentPlan) {
		plan.mode = mode
	}
}

// WithName sets the resolved persistent container name.
func WithName(name invowkfile.ContainerName) PersistentPlanOption {
	return func(plan *PersistentPlan) {
		plan.name = name
	}
}

// WithNameSource sets where the persistent container name came from.
func WithNameSource(source PersistentNameSource) PersistentPlanOption {
	return func(plan *PersistentPlan) {
		plan.nameSource = source
	}
}

// WithCreateIfMissing sets whether missing managed persistent containers may be created.
func WithCreateIfMissing(createIfMissing bool) PersistentPlanOption {
	return func(plan *PersistentPlan) {
		plan.createIfMissing = createIfMissing
	}
}

// EphemeralPlan returns the no-persistent-container plan.
func EphemeralPlan() PersistentPlan {
	return PersistentPlan{mode: PersistentModeEphemeral}
}

// ResolvePersistentTarget returns the pure persistent-container plan for a command.
func ResolvePersistentTarget(req PersistentRequest) PersistentPlan {
	if req.containerNameOverride == "" && req.config == nil {
		return PersistentPlan{mode: PersistentModeEphemeral}
	}

	plan := PersistentPlan{
		mode:            PersistentModePersistent,
		createIfMissing: req.config != nil && req.config.CreateIfMissing,
	}
	switch {
	case req.containerNameOverride != "":
		plan.name = req.containerNameOverride
		plan.nameSource = PersistentNameSourceCLI
	case req.config != nil && req.config.Name != "":
		plan.name = req.config.Name
		plan.nameSource = PersistentNameSourceConfig
	default:
		plan.name = DerivePersistentName(req)
		plan.nameSource = PersistentNameSourceDerived
	}
	return plan
}

// Requested reports whether the plan targets a persistent container.
func (p PersistentPlan) Requested() bool {
	return p.mode == PersistentModePersistent
}

// Validate returns nil when the persistent plan contains valid typed fields.
func (p PersistentPlan) Validate() error {
	var errs []error
	if err := p.mode.Validate(); err != nil {
		errs = append(errs, err)
	}
	if p.name != "" {
		if err := p.name.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.nameSource != "" {
		if err := p.nameSource.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Mode returns whether the plan targets a persistent or ephemeral container.
func (p PersistentPlan) Mode() PersistentMode { return p.mode }

// Name returns the resolved persistent container name.
func (p PersistentPlan) Name() invowkfile.ContainerName { return p.name }

// NameSource returns where the persistent container name came from.
func (p PersistentPlan) NameSource() PersistentNameSource { return p.nameSource }

// CreateIfMissing reports whether missing managed persistent containers may be created.
func (p PersistentPlan) CreateIfMissing() bool { return p.createIfMissing }

// Validate returns nil when mode is one of the known persistent modes.
func (m PersistentMode) Validate() error {
	switch m {
	case PersistentModeEphemeral, PersistentModePersistent:
		return nil
	default:
		return fmt.Errorf("invalid persistent container mode %q", m)
	}
}

// String returns the render label for the persistent mode.
func (m PersistentMode) String() string { return string(m) }

// Validate returns nil when source is one of the known persistent name sources.
func (s PersistentNameSource) Validate() error {
	switch s {
	case PersistentNameSourceCLI, PersistentNameSourceConfig, PersistentNameSourceDerived:
		return nil
	default:
		return fmt.Errorf("invalid persistent container name source %q", s)
	}
}

// String returns the render label for the persistent name source.
func (s PersistentNameSource) String() string { return string(s) }

// Validate returns nil when the command namespace is non-empty.
func (n CommandNamespace) Validate() error {
	if strings.TrimSpace(string(n)) == "" {
		return errors.New("command namespace must not be empty")
	}
	return nil
}

// String returns the command namespace text.
func (n CommandNamespace) String() string { return string(n) }

// DerivePersistentName derives a deterministic managed persistent container name.
func DerivePersistentName(req PersistentRequest) invowkfile.ContainerName {
	namespace := CommandNamespace("")
	if req.commandFullName != nil {
		namespace = *req.commandFullName
	}
	if namespace == "" && req.commandName != nil {
		namespace = *req.commandName
	}
	source := ""
	if req.invowkfilePath != nil {
		source = string(*req.invowkfilePath)
	}

	sum := sha256.Sum256([]byte(string(namespace) + "\x00" + source))
	hash := hex.EncodeToString(sum[:])[:persistentContainerHashLen]
	slug := containerNameSlug(string(namespace))

	maxSlugLen := max(containerargs.MaxContainerNameLength-len(persistentContainerNamePrefix)-len(hash)-1, 1)
	if len(slug) > maxSlugLen {
		slug = strings.Trim(slug[:maxSlugLen], "-._")
	}
	if slug == "" {
		slug = "cmd"
	}

	//goplint:ignore -- validated immediately below with deterministic fallback.
	name := invowkfile.ContainerName(persistentContainerNamePrefix + slug + "-" + hash)
	if err := name.Validate(); err != nil {
		//goplint:ignore -- deterministic hash fallback uses the same portable grammar.
		return invowkfile.ContainerName(persistentContainerNamePrefix + hash)
	}
	return name
}

//goplint:ignore -- slug builder operates on command namespace text for container-name derivation.
func containerNameSlug(value string) string {
	var b strings.Builder
	lastSep := false
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSep = false
		case r == '.', r == '_', r == '-':
			if b.Len() > 0 {
				b.WriteRune(r)
				lastSep = r == '-'
			}
		case !lastSep && b.Len() > 0:
			b.WriteByte('-')
			lastSep = true
		}
	}
	slug := strings.Trim(b.String(), "-._")
	if slug == "" {
		return "cmd"
	}
	return slug
}

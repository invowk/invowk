// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"io"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// ProgramStatusApp identifies Invowk in OSC 7501 Program Status reports.
	ProgramStatusApp = "invowk"

	programStatusMaxTitleBytes   = 192
	programStatusMaxMessageBytes = 2048
)

type programStatusMeta struct {
	title      types.DescriptionText
	password   bool
	selectable bool
}

// NormalizeProgramStatus applies Invowk's standard app identifier and
// sanitizes/bounds Title and Message per OSC 7501 limits before emission.
func NormalizeProgramStatus(status tea.ProgramStatus) *tea.ProgramStatus {
	if status.State == ansi.ProgramStateClear {
		return &tea.ProgramStatus{
			State: ansi.ProgramStateClear,
			ID:    status.ID,
		}
	}
	status.App = ProgramStatusApp
	if status.State != ansi.ProgramStateBlocked {
		status.Kind = ""
	}
	//goplint:ignore -- ANSI OSC 7501 Title and Message are free-text protocol labels normalized below.
	status.Title = sanitizeProgramStatusText(types.DescriptionText(status.Title), true).String()
	//goplint:ignore -- ANSI OSC 7501 Title and Message are free-text protocol labels normalized below.
	status.Message = sanitizeProgramStatusText(types.DescriptionText(status.Message), false).String()
	return &status
}

// Validate validates the program status metadata fields.
func (m programStatusMeta) Validate() error {
	return m.title.Validate()
}

func statusForTUIComponent(component ComponentType, meta programStatusMeta) *tea.ProgramStatus {
	return statusForTUIComponentWithTitle(component, meta.title, "", meta)
}

func statusForTUIComponentWithTitle(component ComponentType, title, message types.DescriptionText, meta programStatusMeta) *tea.ProgramStatus {
	var state ansi.ProgramState
	var kind ansi.ProgramStatusKind

	switch component {
	case ComponentTypeConfirm:
		state = ansi.ProgramStateBlocked
		kind = ansi.ProgramStatusKindPermission
	case ComponentTypeInput:
		state = ansi.ProgramStateBlocked
		if meta.password {
			kind = ansi.ProgramStatusKindAuth
		} else {
			kind = ansi.ProgramStatusKindQuestion
		}
	case ComponentTypeWrite, ComponentTypeTextArea, ComponentTypeChoose, ComponentTypeFilter, ComponentTypeFile:
		state = ansi.ProgramStateBlocked
		kind = ansi.ProgramStatusKindQuestion
	case ComponentTypeTable:
		if meta.selectable {
			state = ansi.ProgramStateBlocked
			kind = ansi.ProgramStatusKindQuestion
		} else {
			state = ansi.ProgramStateIdle
		}
	case ComponentTypePager:
		state = ansi.ProgramStateIdle
	case ComponentTypeSpin:
		state = ansi.ProgramStateWorking
	default:
		return nil
	}

	return NormalizeProgramStatus(tea.ProgramStatus{
		State:   state,
		Kind:    kind,
		Title:   title.String(),
		Message: message.String(),
	})
}

func programStatusMetaFromOptions(component ComponentType, options any) programStatusMeta {
	switch component {
	case ComponentTypeConfirm:
		if opts, ok := options.(ConfirmOptions); ok {
			return programStatusMeta{title: types.DescriptionText(opts.Title)} //goplint:ignore -- display label
		}
	case ComponentTypeInput:
		if opts, ok := options.(InputOptions); ok {
			return programStatusMeta{
				title:    types.DescriptionText(opts.Title), //goplint:ignore -- display label
				password: opts.Password,
			}
		}
	case ComponentTypeWrite, ComponentTypeTextArea:
		if opts, ok := options.(WriteOptions); ok {
			return programStatusMeta{title: types.DescriptionText(opts.Title)} //goplint:ignore -- display label
		}
	case ComponentTypeChoose:
		if opts, ok := options.(ChooseStringOptions); ok {
			return programStatusMeta{title: types.DescriptionText(opts.Title)} //goplint:ignore -- display label
		}
	case ComponentTypeFilter:
		if opts, ok := options.(FilterOptions); ok {
			return programStatusMeta{title: types.DescriptionText(opts.Title)} //goplint:ignore -- display label
		}
	case ComponentTypeFile:
		if opts, ok := options.(FileOptions); ok {
			return programStatusMeta{title: types.DescriptionText(opts.Title)} //goplint:ignore -- display label
		}
	case ComponentTypeTable:
		if opts, ok := options.(TableOptions); ok {
			return programStatusMeta{
				title:      types.DescriptionText(opts.Title), //goplint:ignore -- display label
				selectable: opts.Selectable,
			}
		}
	case ComponentTypePager:
		if opts, ok := options.(PagerOptions); ok {
			return programStatusMeta{title: types.DescriptionText(opts.Title)} //goplint:ignore -- display label
		}
	case ComponentTypeSpin:
		if opts, ok := options.(SpinCommandOptions); ok {
			return programStatusMeta{title: types.DescriptionText(opts.Title)} //goplint:ignore -- display label
		}
	}
	return programStatusMeta{}
}

func sanitizeProgramStatusText(raw types.DescriptionText, forTitle bool) types.DescriptionText {
	limit := programStatusMaxMessageBytes
	if forTitle {
		limit = programStatusMaxTitleBytes
	}
	if limit <= 0 {
		return ""
	}
	s := strings.ToValidUTF8(string(raw), "\uFFFD")
	var sb strings.Builder
	sb.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			if !prevSpace {
				sb.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		sb.WriteRune(r)
	}
	clean := strings.TrimSpace(sb.String())
	if len(clean) > limit {
		clean = clean[:limit]
		for clean != "" && !utf8.ValidString(clean) {
			clean = clean[:len(clean)-1]
		}
		clean = strings.TrimSpace(clean)
	}
	return types.DescriptionText(clean) //goplint:ignore -- sanitized UTF-8 display text bounded by OSC 7501 limit
}

func writeClearProgramStatus(w io.Writer) {
	if w == nil {
		return
	}
	if _, err := io.WriteString(w, ansi.ClearProgramStatus); err != nil {
		return
	}
}

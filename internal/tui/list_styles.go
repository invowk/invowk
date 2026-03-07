// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
)

// applyDelegateStyles sets the standard delegate item styles (NormalTitle,
// NormalDesc, SelectedTitle, SelectedDesc, DimmedTitle, DimmedDesc) for modal
// or non-modal mode. This unifies the identical styling used by choose and
// filter list delegates.
func applyDelegateStyles(delegate *list.DefaultDelegate, forModal bool) {
	if forModal {
		base := modalBaseStyle()
		delegate.Styles.NormalTitle = base.Foreground(modalColorForeground)
		delegate.Styles.NormalDesc = base.Foreground(modalColorMuted)
		delegate.Styles.SelectedTitle = base.
			Foreground(modalColorPrimary).
			Bold(true).
			Padding(0, 0, 0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(modalColorPrimary)
		delegate.Styles.SelectedDesc = base.
			Foreground(modalColorPrimarySoft).
			Padding(0, 0, 0, 1)
		delegate.Styles.DimmedTitle = base.Foreground(modalColorMuted)
		delegate.Styles.DimmedDesc = base.Foreground(modalColorMuted)
	} else {
		delegate.Styles.NormalTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		delegate.Styles.NormalDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		delegate.Styles.SelectedTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true).
			Padding(0, 0, 0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("212"))
		delegate.Styles.SelectedDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Padding(0, 0, 0, 1)
		delegate.Styles.DimmedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		delegate.Styles.DimmedDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}
}

// applyListStyles sets the standard list chrome styles (Title, TitleBar,
// PaginationStyle, HelpStyle) for modal or non-modal mode. In modal mode,
// NoItems is also set with an explicit background. Callers that need additional
// styles (e.g., filter-specific) apply them after this call.
func applyListStyles(l *list.Model, forModal bool) {
	if forModal {
		base := modalBaseStyle()
		l.Styles.Title = base.Bold(true).Foreground(modalColorPrimary)
		l.Styles.TitleBar = base.Padding(0, 0, 1, 0)
		l.Styles.PaginationStyle = base.Foreground(modalColorMuted)
		l.Styles.HelpStyle = base.Foreground(modalColorMuted)
		l.Styles.NoItems = base.Foreground(modalColorMuted)
	} else {
		l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)
		l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		l.Styles.HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}
}

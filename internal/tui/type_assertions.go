// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func expectModel[T tea.Model](model tea.Model, component ComponentType) (T, error) {
	typed, ok := model.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("%s returned unexpected model type %T", component, model)
	}
	return typed, nil
}

func expectResult[T any](result any, component ComponentType) (T, error) {
	typed, ok := result.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("%s returned unexpected result type %T", component, result)
	}
	return typed, nil
}

// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"encoding/json"
	"fmt"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiwire"
)

func componentResponseToProtocol(component tui.ComponentType, response tui.ComponentResponse) tuiwire.Response {
	switch {
	case response.Cancelled:
		return tuiwire.Response{Cancelled: true}
	case response.Err != nil:
		return tuiwire.Response{Error: response.Err.Error()}
	default:
		resultJSON, err := json.Marshal(componentResultToProtocol(component, response.Result))
		if err != nil {
			return tuiwire.Response{Error: fmt.Sprintf("failed to marshal result: %v", err)}
		}
		return tuiwire.Response{Result: resultJSON}
	}
}

func componentResultToProtocol(component tui.ComponentType, result any) any {
	switch component {
	case tui.ComponentTypeInput:
		if s, ok := result.(string); ok {
			return tuiwire.InputResult{Value: s}
		}
		return tuiwire.InputResult{}
	case tui.ComponentTypeTextArea:
		if s, ok := result.(string); ok {
			return tuiwire.TextAreaResult{Value: s}
		}
		return tuiwire.TextAreaResult{}
	case tui.ComponentTypeWrite:
		return tuiwire.WriteResult{}
	case tui.ComponentTypeConfirm:
		if b, ok := result.(bool); ok {
			return tuiwire.ConfirmResult{Confirmed: b}
		}
		return tuiwire.ConfirmResult{}
	case tui.ComponentTypeChoose:
		if selected, ok := result.([]string); ok {
			return tuiwire.ChooseResult{Selected: selected}
		}
		return tuiwire.ChooseResult{Selected: []string{}}
	case tui.ComponentTypeFilter:
		if selected, ok := result.([]string); ok {
			return tuiwire.FilterResult{Selected: selected}
		}
		return tuiwire.FilterResult{Selected: []string{}}
	case tui.ComponentTypeFile:
		if path, ok := result.(string); ok {
			return tuiwire.FileResult{Path: path}
		}
		return tuiwire.FileResult{}
	case tui.ComponentTypeTable:
		if tableResult, ok := result.(tui.TableSelectionResult); ok {
			return tuiwire.TableResult{
				SelectedRow:   tableResult.SelectedRow,
				SelectedIndex: tableResult.SelectedIndex,
			}
		}
		return tuiwire.TableResult{SelectedIndex: -1}
	case tui.ComponentTypePager:
		return tuiwire.PagerResult{}
	case tui.ComponentTypeSpin:
		if spinResult, ok := result.(tuiwire.SpinResult); ok {
			return spinResult
		}
		return tuiwire.SpinResult{}
	default:
		return result
	}
}

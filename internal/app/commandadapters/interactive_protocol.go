// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"encoding/json"
	"fmt"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiwire"
)

func componentRequestFromProtocol(component tui.ComponentType, options json.RawMessage) (any, error) {
	switch component {
	case tui.ComponentTypeInput:
		var req tuiwire.InputRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		width, err := terminalDimensionFromProtocol(req.Width)
		if err != nil {
			return nil, err
		}
		return tui.InputOptions{
			Title:       req.Title,
			Description: req.Description,
			Placeholder: req.Placeholder,
			Value:       req.Value,
			CharLimit:   req.CharLimit,
			Width:       width,
			Password:    req.Password,
			Prompt:      req.Prompt,
		}, nil
	case tui.ComponentTypeConfirm:
		var req tuiwire.ConfirmRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		return tui.ConfirmOptions{
			Title:       req.Title,
			Description: req.Description,
			Affirmative: req.Affirmative,
			Negative:    req.Negative,
			Default:     req.Default,
		}, nil
	case tui.ComponentTypeChoose:
		var opts tui.ChooseStringOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, err
		}
		return opts, nil
	case tui.ComponentTypeFilter:
		var req tuiwire.FilterRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		height, err := terminalDimensionFromProtocol(req.Height)
		if err != nil {
			return nil, err
		}
		width, err := terminalDimensionFromProtocol(req.Width)
		if err != nil {
			return nil, err
		}
		return tui.FilterOptions{
			Title:       req.Title,
			Placeholder: req.Placeholder,
			Options:     req.Options,
			Limit:       req.Limit,
			NoLimit:     req.NoLimit,
			Height:      height,
			Width:       width,
			Reverse:     req.Reverse,
			Fuzzy:       req.Fuzzy,
			Sort:        req.Sort,
			Strict:      req.Strict,
		}, nil
	case tui.ComponentTypeFile:
		var req tuiwire.FileRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		height, err := terminalDimensionFromProtocol(req.Height)
		if err != nil {
			return nil, err
		}
		return tui.FileOptions{
			Title:             req.Title,
			Description:       req.Description,
			CurrentDirectory:  req.Path,
			AllowedExtensions: req.AllowedExts,
			ShowHidden:        req.ShowHidden,
			Height:            height,
			FileAllowed:       req.ShowFiles,
			DirAllowed:        req.ShowDirs,
		}, nil
	case tui.ComponentTypeWrite:
		var req tuiwire.WriteRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		width, err := terminalDimensionFromProtocol(req.Width)
		if err != nil {
			return nil, err
		}
		return tui.WriteOptions{Value: req.Text, Width: width}, nil
	case tui.ComponentTypeTextArea:
		var req tuiwire.TextAreaRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		width, err := terminalDimensionFromProtocol(req.Width)
		if err != nil {
			return nil, err
		}
		height, err := terminalDimensionFromProtocol(req.Height)
		if err != nil {
			return nil, err
		}
		return tui.WriteOptions{
			Title:           req.Title,
			Description:     req.Description,
			Placeholder:     req.Placeholder,
			Value:           req.Value,
			CharLimit:       req.CharLimit,
			Width:           width,
			Height:          height,
			ShowLineNumbers: req.ShowLineNumbers,
		}, nil
	case tui.ComponentTypePager:
		var req tuiwire.PagerRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		return tui.PagerOptions{
			Title:           req.Title,
			Content:         req.Content,
			ShowLineNumbers: req.ShowLineNum,
			SoftWrap:        req.SoftWrap,
		}, nil
	case tui.ComponentTypeTable:
		return tableOptionsFromProtocol(options)
	case tui.ComponentTypeSpin:
		var req tuiwire.SpinRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		spinType := tui.SpinnerLine
		if req.Spinner != "" {
			parsed, err := tui.ParseSpinnerType(req.Spinner)
			if err != nil {
				return nil, err
			}
			spinType = parsed
		}
		return tui.SpinCommandOptions{Title: req.Title, Type: spinType}, nil
	default:
		return nil, fmt.Errorf("unknown component type: %s", component)
	}
}

func tableOptionsFromProtocol(options json.RawMessage) (tui.TableOptions, error) {
	var req tuiwire.TableRequest
	if err := json.Unmarshal(options, &req); err != nil {
		return tui.TableOptions{}, err
	}
	columns := make([]tui.TableColumn, len(req.Columns))
	for i := range req.Columns {
		columns[i] = tui.TableColumn{Title: req.Columns[i]}
		if i < len(req.Widths) {
			width, err := terminalDimensionFromProtocol(req.Widths[i])
			if err != nil {
				return tui.TableOptions{}, err
			}
			columns[i].Width = width
		}
	}
	height, err := terminalDimensionFromProtocol(req.Height)
	if err != nil {
		return tui.TableOptions{}, err
	}
	return tui.TableOptions{
		Columns:    columns,
		Rows:       req.Rows,
		Height:     height,
		Selectable: !req.Print,
		Separator:  req.Separator,
		Border:     req.Border != "none",
	}, nil
}

//goplint:ignore -- protocol DTO dimensions are primitive JSON fields validated before use.
func terminalDimensionFromProtocol(value int) (tui.TerminalDimension, error) {
	dimension := tui.TerminalDimension(value)
	if err := dimension.Validate(); err != nil {
		return 0, err
	}
	return dimension, nil
}

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
		if spinResult, ok := result.(tui.SpinResult); ok {
			return spinResult
		}
		return tui.SpinResult{}
	default:
		return result
	}
}

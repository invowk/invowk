// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiwire"
	"github.com/invowk/invowk/pkg/types"
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
		var req tuiwire.ChooseRequest
		if err := json.Unmarshal(options, &req); err != nil {
			return nil, err
		}
		if req.Description != "" || req.Selected != "" || req.Ordered || req.Cursor != "" {
			return nil, errors.New("choose request contains unsupported fields: description, selected, ordered, cursor")
		}
		height, err := terminalDimensionFromProtocol(req.Height)
		if err != nil {
			return nil, err
		}
		return tui.ChooseStringOptions{
			Title:   req.Title,
			Options: req.Options,
			Limit:   req.Limit,
			NoLimit: req.NoLimit,
			Height:  height,
		}, nil
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
		style, err := styleFromWriteRequest(req)
		if err != nil {
			return nil, err
		}
		text := types.DescriptionText(req.Text) //goplint:ignore -- delegated write text is display content, not a domain identifier.
		return tui.StyledTextOptions{
			Text:  text,
			Style: style,
			Width: width,
		}, nil
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

func validateProtocolStyleBoxes(style tui.Style) error {
	if !validBoxValues(style.Padding) {
		return errors.New("padding must contain 1, 2, or 4 values")
	}
	if !validBoxValues(style.Margin) {
		return errors.New("margin must contain 1, 2, or 4 values")
	}
	return nil
}

//goplint:ignore -- protocol box dimensions are raw JSON integer lists validated before rendering.
func validBoxValues(values []int) bool {
	switch len(values) {
	case 0, 1, 2, 4:
		return true
	default:
		return false
	}
}

func styleFromWriteRequest(req tuiwire.WriteRequest) (tui.Style, error) {
	foreground, err := colorSpecFromProtocol(req.Foreground)
	if err != nil {
		return tui.Style{}, err
	}
	background, err := colorSpecFromProtocol(req.Background)
	if err != nil {
		return tui.Style{}, err
	}
	borderForeground, err := colorSpecFromProtocol(req.BorderForeground)
	if err != nil {
		return tui.Style{}, err
	}
	border, err := borderStyleFromProtocol(req.Border)
	if err != nil {
		return tui.Style{}, err
	}
	align, err := textAlignFromProtocol(req.Align)
	if err != nil {
		return tui.Style{}, err
	}
	style := tui.Style{
		Foreground:       foreground,
		Background:       background,
		Bold:             req.Bold,
		Italic:           req.Italic,
		Underline:        req.Underline,
		Strikethrough:    req.Strikethrough,
		Faint:            req.Faint,
		Blink:            req.Blink,
		Border:           border,
		BorderForeground: borderForeground,
		Align:            align,
		Padding:          req.Padding,
		Margin:           req.Margin,
	}
	if err := validateProtocolStyleBoxes(style); err != nil {
		return tui.Style{}, err
	}
	return style, nil
}

//goplint:ignore -- protocol color values are raw JSON strings converted and validated before return.
func colorSpecFromProtocol(value string) (tui.ColorSpec, error) {
	spec := tui.ColorSpec(value) //goplint:ignore -- validated before return
	if err := spec.Validate(); err != nil {
		return "", err
	}
	return spec, nil
}

//goplint:ignore -- protocol border style values are raw JSON strings converted and validated before return.
func borderStyleFromProtocol(value string) (tui.BorderStyle, error) {
	border := tui.BorderStyle(value) //goplint:ignore -- validated before return
	if err := border.Validate(); err != nil {
		return "", err
	}
	return border, nil
}

//goplint:ignore -- protocol text alignment values are raw JSON strings converted and validated before return.
func textAlignFromProtocol(value string) (tui.TextAlign, error) {
	align := tui.TextAlign(value) //goplint:ignore -- validated before return
	if err := align.Validate(); err != nil {
		return "", err
	}
	return align, nil
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

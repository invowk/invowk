// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"sync"

	"golang.org/x/tools/go/analysis"
)

type findingJSONLRecord struct {
	Category string `json:"category"`
	ID       string `json:"id"`
	Message  string `json:"message"`
	Posn     string `json:"posn,omitempty"`
}

type findingSink struct {
	path string
	file *os.File
	refs int
	mu   sync.Mutex
}

var findingSinkRegistry struct {
	mu     sync.Mutex
	byPath map[string]*findingSink
	byPass map[*analysis.Pass]*findingSink
}

func registerFindingSinkForPass(pass *analysis.Pass, path string) error {
	if pass == nil || path == "" {
		return nil
	}

	findingSinkRegistry.mu.Lock()
	defer findingSinkRegistry.mu.Unlock()

	if findingSinkRegistry.byPath == nil {
		findingSinkRegistry.byPath = make(map[string]*findingSink)
	}
	if findingSinkRegistry.byPass == nil {
		findingSinkRegistry.byPass = make(map[*analysis.Pass]*findingSink)
	}

	sink := findingSinkRegistry.byPath[path]
	if sink == nil {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("opening findings sink %q: %w", path, err)
		}
		sink = &findingSink{
			path: path,
			file: file,
		}
		findingSinkRegistry.byPath[path] = sink
	}

	sink.refs++
	findingSinkRegistry.byPass[pass] = sink
	return nil
}

func unregisterFindingSinkForPass(pass *analysis.Pass) {
	if pass == nil {
		return
	}

	findingSinkRegistry.mu.Lock()
	defer findingSinkRegistry.mu.Unlock()

	if findingSinkRegistry.byPass == nil {
		return
	}
	sink := findingSinkRegistry.byPass[pass]
	delete(findingSinkRegistry.byPass, pass)
	if sink == nil {
		return
	}

	sink.refs--
	if sink.refs > 0 {
		return
	}
	if findingSinkRegistry.byPath != nil {
		delete(findingSinkRegistry.byPath, sink.path)
	}
	_ = sink.file.Close()
}

func writeFindingToSink(pass *analysis.Pass, pos token.Pos, category, findingID, message string) {
	if pass == nil {
		return
	}

	findingSinkRegistry.mu.Lock()
	sink := findingSinkRegistry.byPass[pass]
	findingSinkRegistry.mu.Unlock()
	if sink == nil {
		return
	}

	record := findingJSONLRecord{
		Category: category,
		ID:       findingID,
		Message:  message,
	}
	if pass.Fset != nil && pos.IsValid() {
		record.Posn = pass.Fset.Position(pos).String()
	}

	line, err := json.Marshal(record)
	if err != nil {
		return
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	_, _ = sink.file.Write(line)
	_, _ = sink.file.Write([]byte("\n"))
}

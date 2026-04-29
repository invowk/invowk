// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/tuiserver"
)

func TestReadTableConfig(t *testing.T) {
	t.Parallel()

	cmd := newTUITableCommand()
	if err := cmd.Flags().Set("file", "data.csv"); err != nil {
		t.Fatalf("Set(file): %v", err)
	}
	if err := cmd.Flags().Set("separator", "|"); err != nil {
		t.Fatalf("Set(separator): %v", err)
	}
	if err := cmd.Flags().Set("columns", "name,age"); err != nil {
		t.Fatalf("Set(columns): %v", err)
	}
	if err := cmd.Flags().Set("widths", "20,10"); err != nil {
		t.Fatalf("Set(widths): %v", err)
	}
	if err := cmd.Flags().Set("height", "7"); err != nil {
		t.Fatalf("Set(height): %v", err)
	}
	if err := cmd.Flags().Set("selectable", "true"); err != nil {
		t.Fatalf("Set(selectable): %v", err)
	}

	cfg, err := readTableConfig(cmd)
	if err != nil {
		t.Fatalf("readTableConfig() error = %v", err)
	}

	if cfg.file != tableFile("data.csv") {
		t.Fatalf("cfg.file = %q, want %q", cfg.file, "data.csv")
	}
	if cfg.separator != tableSeparator("|") {
		t.Fatalf("cfg.separator = %q, want %q", cfg.separator, "|")
	}
	if len(cfg.columns) != 2 || cfg.columns[0] != "name" || cfg.columns[1] != "age" {
		t.Fatalf("cfg.columns = %v", cfg.columns)
	}
	if len(cfg.widths) != 2 || cfg.widths[0] != 20 || cfg.widths[1] != 10 {
		t.Fatalf("cfg.widths = %v", cfg.widths)
	}
	if cfg.height != tableHeight(7) {
		t.Fatalf("cfg.height = %d, want 7", cfg.height)
	}
	if !cfg.selectable {
		t.Fatal("cfg.selectable = false, want true")
	}
}

func TestLoadTableRowsFromFileAndSplitHeaders(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "data.csv")
	if err := os.WriteFile(csvPath, []byte("name,age\nAlice,30\nBob,25\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	rows, err := loadTableRows(tableConfig{
		file:      tableFile(csvPath),
		separator: tableSeparator(","),
	})
	if err != nil {
		t.Fatalf("loadTableRows(): %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}

	headers, data := splitHeadersAndData(rows, nil)
	if strings.Join(headers, ",") != "name,age" {
		t.Fatalf("headers = %v", headers)
	}
	if len(data) != 2 || strings.Join(data[1], ",") != "Bob,25" {
		t.Fatalf("data = %v", data)
	}

	overrideHeaders, overrideData := splitHeadersAndData(rows, tableColumns{"first", "second"})
	if strings.Join(overrideHeaders, ",") != "first,second" {
		t.Fatalf("overrideHeaders = %v", overrideHeaders)
	}
	if len(overrideData) != 3 {
		t.Fatalf("len(overrideData) = %d, want 3", len(overrideData))
	}
}

func TestLoadTableRowsFromStdin(t *testing.T) {
	// No t.Parallel(): withPipeStdin replaces os.Stdin (process-wide).
	restore := withPipeStdin(t, "name|age\nAlice|30\nBob|25\n")
	defer restore()

	rows, err := loadTableRowsFromStdin(tableConfig{separator: tableSeparator("|")})
	if err != nil {
		t.Fatalf("loadTableRowsFromStdin(): %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	if strings.Join(rows[2], "|") != "Bob|25" {
		t.Fatalf("rows[2] = %v", rows[2])
	}
}

func TestRenderTUITableDirectWithNoRows(t *testing.T) {
	t.Parallel()

	idx, row, err := renderTUITableDirect(
		tableConfig{separator: tableSeparator(",")},
		[]string{"name", "age"},
		nil,
	)
	if err != nil {
		t.Fatalf("renderTUITableDirect(): %v", err)
	}
	if idx != -1 {
		t.Fatalf("idx = %d, want -1", idx)
	}
	if row != nil {
		t.Fatalf("row = %v, want nil", row)
	}
}

func TestRenderTUITableWithClient(t *testing.T) {
	t.Parallel()

	server, requests, asyncErrs := startTableTestServer(t, tuiserver.TableResult{
		SelectedIndex: 1,
		SelectedRow:   []string{"Bob", "25"},
	})

	client := tuiserver.NewClient(string(server.URL()), server.Token())
	idx, row, err := renderTUITableWithClient(
		t.Context(),
		tableConfig{
			separator:  tableSeparator(","),
			widths:     tableWidths{12, 8},
			height:     tableHeight(6),
			selectable: true,
		},
		[]string{"name", "age"},
		[][]string{{"Alice", "30"}, {"Bob", "25"}},
		client,
	)
	if err != nil {
		t.Fatalf("renderTUITableWithClient(): %v", err)
	}
	if idx != 1 {
		t.Fatalf("idx = %d, want 1", idx)
	}
	if strings.Join(row, ",") != "Bob,25" {
		t.Fatalf("row = %v", row)
	}

	req := <-requests
	if req.Border != "normal" {
		t.Fatalf("req.Border = %q, want normal", req.Border)
	}
	if req.Print {
		t.Fatal("req.Print = true, want false")
	}
	if req.Height != 6 {
		t.Fatalf("req.Height = %d, want 6", req.Height)
	}
	if len(req.Widths) != 2 || req.Widths[0] != 12 || req.Widths[1] != 8 {
		t.Fatalf("req.Widths = %v", req.Widths)
	}
	assertNoAsyncError(t, asyncErrs)
}

func TestRunTuiTableSelectablePrintsSelectedRow(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "data.csv")
	if err := os.WriteFile(csvPath, []byte("name,age\nAlice,30\nBob,25\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	server, requests, asyncErrs := startTableTestServer(t, tuiserver.TableResult{
		SelectedIndex: 1,
		SelectedRow:   []string{"Bob", "25"},
	})
	t.Setenv(tuiserver.EnvTUIAddr, string(server.URL()))
	t.Setenv(tuiserver.EnvTUIToken, server.Token().String())

	cmd := newTUITableCommand()
	if err := cmd.Flags().Set("file", csvPath); err != nil {
		t.Fatalf("Set(file): %v", err)
	}
	if err := cmd.Flags().Set("selectable", "true"); err != nil {
		t.Fatalf("Set(selectable): %v", err)
	}

	output := captureStdout(t, func() {
		if err := runTuiTable(cmd, nil); err != nil {
			t.Fatalf("runTuiTable(): %v", err)
		}
	})

	if output != "Bob,25\n" {
		t.Fatalf("stdout = %q, want %q", output, "Bob,25\n")
	}

	req := <-requests
	if req.Border != "normal" {
		t.Fatalf("req.Border = %q, want normal", req.Border)
	}
	if req.Print {
		t.Fatal("req.Print = true, want false")
	}
	assertNoAsyncError(t, asyncErrs)
}

func TestRunTuiTableNoData(t *testing.T) {
	// No t.Parallel(): withPipeStdin replaces os.Stdin (process-wide).
	restore := withPipeStdin(t, "")
	defer restore()

	err := runTuiTable(newTUITableCommand(), nil)
	if !errors.Is(err, errNoDataToDisplay) {
		t.Fatalf("runTuiTable() error = %v, want errNoDataToDisplay", err)
	}
}

func withPipeStdin(t *testing.T, input string) func() {
	t.Helper()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("WriteString(): %v", err)
	}
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("Close(writer): %v", closeErr)
	}
	os.Stdin = r

	return func() {
		_ = r.Close()
		os.Stdin = oldStdin
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	os.Stdout = w

	fn()

	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("Close(writer): %v", closeErr)
	}
	os.Stdout = oldStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll(): %v", err)
	}
	_ = r.Close()
	return string(out)
}

func startTableTestServer(t *testing.T, result tuiserver.TableResult) (server *tuiserver.Server, requests <-chan tuiserver.TableRequest, asyncErrs <-chan error) {
	t.Helper()

	server, err := tuiserver.New()
	if err != nil {
		t.Fatalf("tuiserver.New(): %v", err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatalf("server.Start(): %v", err)
	}
	t.Cleanup(func() {
		if stopErr := server.Stop(); stopErr != nil {
			t.Fatalf("server.Stop(): %v", stopErr)
		}
	})

	requestCh := make(chan tuiserver.TableRequest, 1)
	errCh := make(chan error, 1)
	go func() {
		req := <-server.RequestChannel()
		var tableReq tuiserver.TableRequest
		if err := json.Unmarshal(req.Options, &tableReq); err != nil {
			errCh <- err
			return
		}
		requestCh <- tableReq

		payload, err := json.Marshal(result)
		if err != nil {
			errCh <- err
			return
		}
		req.ResponseCh <- tuiserver.Response{Result: payload}
	}()

	return server, requestCh, errCh
}

func assertNoAsyncError(t *testing.T, asyncErrs <-chan error) {
	t.Helper()

	select {
	case err := <-asyncErrs:
		t.Fatalf("async server error: %v", err)
	default:
	}
}

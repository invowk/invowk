// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/types"
)

// startTestServer creates a running TUI server and client for testing.
// The server is stopped via t.Cleanup.
func startTestServer(t *testing.T) (*Server, *Client) {
	t.Helper()

	server, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { testutil.MustStop(t, server) })

	client := NewClient(string(server.URL()), server.Token())
	return server, client
}

// respondWith spawns a goroutine that reads one TUIRequest from the server's
// request channel and sends the given response. It always signals completion
// on the returned channel: nil on success, non-nil if the channel closed.
func respondWith(t *testing.T, server *Server, resp Response) <-chan error {
	t.Helper()

	errCh := make(chan error, 1)
	go func() {
		req, ok := <-server.RequestChannel()
		if !ok {
			errCh <- errors.New("request channel closed unexpectedly")
			return
		}
		req.ResponseCh <- resp
		errCh <- nil
	}()
	return errCh
}

// mustMarshalResult marshals v into json.RawMessage for use in Response.Result.
func mustMarshalResult(t *testing.T, v any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

// assertNoAsyncError waits for the respondWith goroutine to complete and
// verifies it did not encounter an error.
func assertNoAsyncError(t *testing.T, errCh <-chan error) {
	t.Helper()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("async server error: %v", err)
		}
	case <-timer.C:
		t.Fatal("timed out waiting for respondWith goroutine to complete")
	}
}

// assertCancelled verifies that a client method returns ErrUserCancelled when
// the server responds with Cancelled: true. This is the shared cancellation
// contract test — all interactive TUI components must respect this contract.
func assertCancelled(t *testing.T, server *Server, callClient func() error) {
	t.Helper()

	errCh := respondWith(t, server, Response{Cancelled: true})

	err := callClient()
	if !errors.Is(err, types.ErrUserCancelled) {
		t.Fatalf("error = %v, want ErrUserCancelled", err)
	}
	assertNoAsyncError(t, errCh)
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_Input(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("success", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, InputResult{Value: "hello world"}),
		})

		val, err := client.Input(InputRequest{Title: "test"})
		if err != nil {
			t.Fatalf("Input() error = %v", err)
		}
		if val != "hello world" {
			t.Fatalf("Input() = %q, want %q", val, "hello world")
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.Input(InputRequest{Title: "test"})
			return err
		})
	})
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_Confirm(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("confirmed", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, ConfirmResult{Confirmed: true}),
		})

		confirmed, err := client.Confirm(ConfirmRequest{Title: "proceed?"})
		if err != nil {
			t.Fatalf("Confirm() error = %v", err)
		}
		if !confirmed {
			t.Fatal("Confirm() = false, want true")
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("denied", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, ConfirmResult{Confirmed: false}),
		})

		confirmed, err := client.Confirm(ConfirmRequest{Title: "proceed?"})
		if err != nil {
			t.Fatalf("Confirm() error = %v", err)
		}
		if confirmed {
			t.Fatal("Confirm() = true, want false")
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.Confirm(ConfirmRequest{Title: "proceed?"})
			return err
		})
	})
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_Choose(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("success", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, ChooseResult{Selected: "option-a"}),
		})

		result, err := client.Choose(ChooseRequest{Options: []string{"option-a", "option-b"}})
		if err != nil {
			t.Fatalf("Choose() error = %v", err)
		}
		if result != "option-a" {
			t.Fatalf("Choose() = %v, want %q", result, "option-a")
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.Choose(ChooseRequest{Options: []string{"a"}})
			return err
		})
	})
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_ChooseSingle(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("string result", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, ChooseResult{Selected: "pick"}),
		})

		val, err := client.ChooseSingle(ChooseRequest{Options: []string{"pick", "other"}})
		if err != nil {
			t.Fatalf("ChooseSingle() error = %v", err)
		}
		if val != "pick" {
			t.Fatalf("ChooseSingle() = %q, want %q", val, "pick")
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("array result", func(t *testing.T) {
		// JSON unmarshals a single-element array as []any, which ChooseSingle coerces.
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, ChooseResult{Selected: []string{"pick"}}),
		})

		val, err := client.ChooseSingle(ChooseRequest{Options: []string{"pick", "other"}})
		if err != nil {
			t.Fatalf("ChooseSingle() error = %v", err)
		}
		if val != "pick" {
			t.Fatalf("ChooseSingle() = %q, want %q", val, "pick")
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.ChooseSingle(ChooseRequest{Options: []string{"a"}})
			return err
		})
	})
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_ChooseMultiple(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("array result", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, ChooseResult{Selected: []string{"a", "b"}}),
		})

		vals, err := client.ChooseMultiple(ChooseRequest{Options: []string{"a", "b", "c"}, NoLimit: true})
		if err != nil {
			t.Fatalf("ChooseMultiple() error = %v", err)
		}
		if !slices.Equal(vals, []string{"a", "b"}) {
			t.Fatalf("ChooseMultiple() = %v, want [a b]", vals)
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("string fallback", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, ChooseResult{Selected: "solo"}),
		})

		vals, err := client.ChooseMultiple(ChooseRequest{Options: []string{"solo"}})
		if err != nil {
			t.Fatalf("ChooseMultiple() error = %v", err)
		}
		if !slices.Equal(vals, []string{"solo"}) {
			t.Fatalf("ChooseMultiple() = %v, want [solo]", vals)
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.ChooseMultiple(ChooseRequest{Options: []string{"a"}})
			return err
		})
	})
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_Filter(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("success", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, FilterResult{Selected: []string{"x", "y"}}),
		})

		vals, err := client.Filter(FilterRequest{Options: []string{"x", "y", "z"}})
		if err != nil {
			t.Fatalf("Filter() error = %v", err)
		}
		if !slices.Equal(vals, []string{"x", "y"}) {
			t.Fatalf("Filter() = %v, want [x y]", vals)
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.Filter(FilterRequest{Options: []string{"a"}})
			return err
		})
	})
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_File(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("success", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "file.txt")
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, FileResult{Path: filePath}),
		})

		path, err := client.File(FileRequest{Title: "pick file"})
		if err != nil {
			t.Fatalf("File() error = %v", err)
		}
		if path != filePath {
			t.Fatalf("File() = %q, want %q", path, filePath)
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.File(FileRequest{Title: "pick file"})
			return err
		})
	})
}

func TestClient_Write(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	errCh := respondWith(t, server, Response{
		Result: mustMarshalResult(t, WriteResult{}),
	})

	err := client.Write(WriteRequest{Text: "hello"})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	assertNoAsyncError(t, errCh)
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_TextArea(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("success", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, TextAreaResult{Value: "multi\nline"}),
		})

		val, err := client.TextArea(TextAreaRequest{Title: "editor"})
		if err != nil {
			t.Fatalf("TextArea() error = %v", err)
		}
		if val != "multi\nline" {
			t.Fatalf("TextArea() = %q, want %q", val, "multi\nline")
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.TextArea(TextAreaRequest{Title: "editor"})
			return err
		})
	})
}

func TestClient_Spin(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	errCh := respondWith(t, server, Response{
		Result: mustMarshalResult(t, SpinResult{Stdout: "output", ExitCode: 0}),
	})

	result, err := client.Spin(SpinRequest{Title: "loading", Command: []string{"echo", "hi"}})
	if err != nil {
		t.Fatalf("Spin() error = %v", err)
	}
	if result.Stdout != "output" {
		t.Fatalf("Spin().Stdout = %q, want %q", result.Stdout, "output")
	}
	if result.ExitCode != 0 {
		t.Fatalf("Spin().ExitCode = %d, want 0", result.ExitCode)
	}
	assertNoAsyncError(t, errCh)
}

func TestClient_Pager(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	errCh := respondWith(t, server, Response{
		Result: mustMarshalResult(t, PagerResult{}),
	})

	err := client.Pager(PagerRequest{Content: "long text"})
	if err != nil {
		t.Fatalf("Pager() error = %v", err)
	}
	assertNoAsyncError(t, errCh)
}

//nolint:tparallel // Subtests share server's RequestChannel and must run sequentially.
func TestClient_Table(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	t.Run("success", func(t *testing.T) {
		errCh := respondWith(t, server, Response{
			Result: mustMarshalResult(t, TableResult{
				SelectedRow:   []string{"Alice", "30"},
				SelectedIndex: 0,
			}),
		})

		result, err := client.Table(TableRequest{
			Columns: []string{"name", "age"},
			Rows:    [][]string{{"Alice", "30"}, {"Bob", "25"}},
		})
		if err != nil {
			t.Fatalf("Table() error = %v", err)
		}
		if result.SelectedIndex != 0 {
			t.Fatalf("Table().SelectedIndex = %d, want 0", result.SelectedIndex)
		}
		if !slices.Equal(result.SelectedRow, []string{"Alice", "30"}) {
			t.Fatalf("Table().SelectedRow = %v, want [Alice 30]", result.SelectedRow)
		}
		assertNoAsyncError(t, errCh)
	})

	t.Run("cancelled", func(t *testing.T) {
		assertCancelled(t, server, func() error {
			_, err := client.Table(TableRequest{Columns: []string{"a"}, Rows: [][]string{{"1"}}})
			return err
		})
	})
}

func TestClient_ServerError(t *testing.T) {
	t.Parallel()

	server, client := startTestServer(t)

	errCh := respondWith(t, server, Response{Error: "something broke"})

	_, err := client.Input(InputRequest{Title: "test"})
	if err == nil {
		t.Fatal("expected error from server error response")
	}
	if !strings.Contains(err.Error(), "TUI error: something broke") {
		t.Fatalf("error = %q, want containing %q", err.Error(), "TUI error: something broke")
	}
	assertNoAsyncError(t, errCh)
}

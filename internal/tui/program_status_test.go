// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/invowk/invowk/pkg/types"
)

type quitWithoutDoneModel struct {
	done      bool
	interrupt bool
}

func (m quitWithoutDoneModel) Init() tea.Cmd {
	if m.interrupt {
		return tea.Interrupt
	}
	return tea.Quit
}
func (m quitWithoutDoneModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m quitWithoutDoneModel) View() tea.View                      { return tea.NewView("running") }
func (m quitWithoutDoneModel) IsDone() bool                        { return m.done }
func (m quitWithoutDoneModel) Result() (any, error)                { return "", nil }
func (m quitWithoutDoneModel) Cancelled() bool                     { return false }
func (m quitWithoutDoneModel) SetSize(TerminalDimension, TerminalDimension) {
}

func TestProgramStatusMappingsAndSanitization(t *testing.T) {
	t.Parallel()

	t.Run("component mappings", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name      string
			component ComponentType
			meta      programStatusMeta
			wantState ansi.ProgramState
			wantKind  ansi.ProgramStatusKind
		}{
			{
				name:      "confirm",
				component: ComponentTypeConfirm,
				meta:      programStatusMeta{title: "Proceed?"},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindPermission,
			},
			{
				name:      "password input",
				component: ComponentTypeInput,
				meta:      programStatusMeta{title: "Token", password: true},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindAuth,
			},
			{
				name:      "plain input",
				component: ComponentTypeInput,
				meta:      programStatusMeta{title: "Name"},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindQuestion,
			},
			{
				name:      "write",
				component: ComponentTypeWrite,
				meta:      programStatusMeta{title: "Notes"},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindQuestion,
			},
			{
				name:      "textarea",
				component: ComponentTypeTextArea,
				meta:      programStatusMeta{title: "Body"},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindQuestion,
			},
			{
				name:      "choose",
				component: ComponentTypeChoose,
				meta:      programStatusMeta{title: "Pick"},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindQuestion,
			},
			{
				name:      "filter",
				component: ComponentTypeFilter,
				meta:      programStatusMeta{title: "Search"},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindQuestion,
			},
			{
				name:      "file",
				component: ComponentTypeFile,
				meta:      programStatusMeta{title: "File"},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindQuestion,
			},
			{
				name:      "selectable table",
				component: ComponentTypeTable,
				meta:      programStatusMeta{title: "Rows", selectable: true},
				wantState: ansi.ProgramStateBlocked,
				wantKind:  ansi.ProgramStatusKindQuestion,
			},
			{
				name:      "non-selectable table",
				component: ComponentTypeTable,
				meta:      programStatusMeta{title: "Rows", selectable: false},
				wantState: ansi.ProgramStateIdle,
			},
			{
				name:      "pager",
				component: ComponentTypePager,
				meta:      programStatusMeta{title: "Logs"},
				wantState: ansi.ProgramStateIdle,
			},
			{
				name:      "spin",
				component: ComponentTypeSpin,
				meta:      programStatusMeta{title: "Building"},
				wantState: ansi.ProgramStateWorking,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := statusForTUIComponent(tc.component, tc.meta)
				if got == nil {
					t.Fatalf("statusForTUIComponent(%s) = nil", tc.component)
				}
				if got.App != ProgramStatusApp {
					t.Fatalf("App = %q, want %q", got.App, ProgramStatusApp)
				}
				if got.State != tc.wantState {
					t.Fatalf("State = %q, want %q", got.State, tc.wantState)
				}
				if got.Kind != tc.wantKind {
					t.Fatalf("Kind = %q, want %q", got.Kind, tc.wantKind)
				}
			})
		}

		if got := statusForTUIComponent(ComponentType("unknown"), programStatusMeta{}); got != nil {
			t.Fatalf("unknown component status = %+v, want nil", got)
		}
	})

	t.Run("sanitization and utf8 bounds", func(t *testing.T) {
		t.Parallel()

		raw := types.DescriptionText(" \nhello\x00\t\x1bworld\x7f!\u0080\u0085\u009fend\xff ")
		got := sanitizeProgramStatusText(raw, true).String()
		want := "hello world ! end\uFFFD"
		if got != want {
			t.Fatalf("sanitizeProgramStatusText() = %q, want %q", got, want)
		}

		if empty := sanitizeProgramStatusText(" \t\r\n ", true); empty != "" {
			t.Fatalf("whitespace-only sanitizeProgramStatusText() = %q, want empty", empty)
		}

		// 4-byte UTF-8 boundary + trailing space trim after truncation.
		fourByte := types.DescriptionText(strings.Repeat("a", 189) + " 😀😀")
		bounded4 := sanitizeProgramStatusText(fourByte, true).String()
		if len(bounded4) > 192 || !utf8.ValidString(bounded4) || strings.HasSuffix(bounded4, " ") {
			t.Fatalf("4-byte bounded = %q (len=%d, valid=%v)", bounded4, len(bounded4), utf8.ValidString(bounded4))
		}

		st := NormalizeProgramStatus(tea.ProgramStatus{
			State:   ansi.ProgramStateWorking,
			Kind:    ansi.ProgramStatusKindPermission,
			Title:   strings.Repeat("é", 120),
			Message: strings.Repeat("a", 2200),
		})
		if st.App != ProgramStatusApp || st.Kind != "" {
			t.Fatalf("unexpected NormalizeProgramStatus App/Kind: %+v", st)
		}
		if len(st.Title) > 192 || len(st.Message) > 2048 || !utf8.ValidString(st.Title) {
			t.Fatalf("unexpected bounds: title=%d msg=%d", len(st.Title), len(st.Message))
		}

		clr := NormalizeProgramStatus(tea.ProgramStatus{
			State: ansi.ProgramStateClear,
			ID:    "sub",
			App:   "ignored",
			Title: "ignored",
		})
		if clr.State != ansi.ProgramStateClear || clr.ID != "sub" || clr.App != "" || clr.Title != "" {
			t.Fatalf("clear NormalizeProgramStatus = %+v, want clean clear", clr)
		}
	})
}

func TestInteractiveModel_ProgramStatusLifecycle(t *testing.T) {
	t.Parallel()

	m := newInteractiveModel(InteractiveOptions{Title: "Running", CommandName: "deploy"}, nil)

	// Before ready: working
	v := m.View()
	if v.ProgramStatus == nil || v.ProgramStatus.State != ansi.ProgramStateWorking || v.ProgramStatus.Title != "deploy" {
		t.Fatalf("pre-ready View().ProgramStatus = %+v, want working deploy", v.ProgramStatus)
	}

	// Force quit while !m.ready clears status
	preQuitM := newInteractiveModel(InteractiveOptions{CommandName: "deploy"}, nil)
	_, cmd := preQuitM.Update(tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected quit cmd on Ctrl+\\ before ready")
	}
	if got := preQuitM.View().ProgramStatus; got != nil {
		t.Fatalf("pre-ready quit View().ProgramStatus = %+v, want nil", got)
	}

	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// ComponentTypeWrite is non-blocking styled print inside stateExecuting (stays working).
	writeRespCh := make(chan ComponentResponse, 1)
	_, _ = m.Update(TUIComponentMsg{
		Component:  ComponentTypeWrite,
		Options:    StyledTextOptions{Text: "hello"},
		ResponseCh: writeRespCh,
	})
	if st := m.View().ProgramStatus; st == nil || st.State != ansi.ProgramStateWorking {
		t.Fatalf("write styled output status = %+v, want working", st)
	}

	overlayCases := []struct {
		name      string
		comp      ComponentType
		opts      any
		wantState ansi.ProgramState
		wantKind  ansi.ProgramStatusKind
		wantMsg   string
	}{
		{name: "confirm", comp: ComponentTypeConfirm, opts: ConfirmOptions{Title: "Approve?"}, wantState: ansi.ProgramStateBlocked, wantKind: ansi.ProgramStatusKindPermission, wantMsg: "Approve?"},
		{name: "password input", comp: ComponentTypeInput, opts: InputOptions{Title: "API Secret", Password: true}, wantState: ansi.ProgramStateBlocked, wantKind: ansi.ProgramStatusKindAuth, wantMsg: "API Secret"},
		{name: "selectable table", comp: ComponentTypeTable, opts: TableOptions{Title: "Pick Row", Columns: []TableColumn{{Title: "Col"}}, Rows: [][]string{{"1"}}, Selectable: true}, wantState: ansi.ProgramStateBlocked, wantKind: ansi.ProgramStatusKindQuestion, wantMsg: "Pick Row"},
		{name: "non-selectable table", comp: ComponentTypeTable, opts: TableOptions{Title: "View Rows", Columns: []TableColumn{{Title: "Col"}}, Rows: [][]string{{"1"}}, Selectable: false}, wantState: ansi.ProgramStateIdle, wantMsg: "View Rows"},
		{name: "pager", comp: ComponentTypePager, opts: PagerOptions{Title: "Logs", Content: "line1"}, wantState: ansi.ProgramStateIdle, wantMsg: "Logs"},
		{name: "spin", comp: ComponentTypeSpin, opts: SpinCommandOptions{Title: "Working"}, wantState: ansi.ProgramStateWorking, wantMsg: "Working"},
	}

	for _, oc := range overlayCases {
		respCh := make(chan ComponentResponse, 1)
		_, _ = m.Update(TUIComponentMsg{
			Component:  oc.comp,
			Options:    oc.opts,
			ResponseCh: respCh,
		})
		st := m.View().ProgramStatus
		if st == nil || st.State != oc.wantState || st.Kind != oc.wantKind || st.Title != "deploy" || st.Message != oc.wantMsg {
			t.Fatalf("%s overlay status = %+v, want state=%q kind=%q title=deploy msg=%q", oc.name, st, oc.wantState, oc.wantKind, oc.wantMsg)
		}
		_, _ = m.Update(tuiComponentDoneMsg{result: "ok"})
		select {
		case <-respCh:
		case <-time.After(time.Second):
			t.Fatalf("%s: timed out waiting for ResponseCh", oc.name)
		}
		back := m.View().ProgramStatus
		if back == nil || back.State != ansi.ProgramStateWorking || back.Message != "" {
			t.Fatalf("%s post-overlay status = %+v, want working without overlay message", oc.name, back)
		}
	}

	// Ctrl+\ during stateTUI force-quits and clears status cleanly.
	forceRespCh := make(chan ComponentResponse, 1)
	_, _ = m.Update(TUIComponentMsg{
		Component:  ComponentTypeConfirm,
		Options:    ConfirmOptions{Title: "Hang"},
		ResponseCh: forceRespCh,
	})
	_, fqCmd := m.Update(tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl})
	if fqCmd == nil || !m.quitting || m.View().ProgramStatus != nil {
		t.Fatalf("Ctrl+\\ in stateTUI: cmd=%v quitting=%v status=%+v", fqCmd != nil, m.quitting, m.View().ProgramStatus)
	}
	select {
	case resp := <-forceRespCh:
		if !errors.Is(resp.Err, ErrCancelled) {
			t.Fatalf("force-quit ResponseCh.Err = %v, want ErrCancelled", resp.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for force-quit ResponseCh")
	}

	// Completion done vs error + dismiss clear on fresh model
	doneM := newInteractiveModel(InteractiveOptions{CommandName: "deploy"}, nil)
	_, _ = doneM.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, _ = doneM.Update(doneMsg{result: InteractiveResult{ExitCode: 0}})
	if st := doneM.View().ProgramStatus; st == nil || st.State != ansi.ProgramStateDone || st.Title != "deploy" {
		t.Fatalf("completed exit 0 status = %+v, want done", st)
	}
	_, _ = doneM.Update(doneMsg{result: InteractiveResult{ExitCode: 2}})
	if st := doneM.View().ProgramStatus; st == nil || st.State != ansi.ProgramStateError {
		t.Fatalf("completed exit 2 status = %+v, want error", st)
	}
	_, _ = doneM.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if st := doneM.View().ProgramStatus; st != nil {
		t.Fatalf("dismissed completed status = %+v, want nil", st)
	}
}

func TestRunInteractiveSession_ForcedExitsEmitClearProgramStatus(t *testing.T) {
	t.Parallel()

	t.Run("ErrProgramKilled emits clear", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var clearBuf bytes.Buffer
		var outBuf bytes.Buffer
		readyCh := make(chan struct{}, 1)

		_, err := RunInteractiveSession(
			ctx,
			InteractiveOptions{
				CommandName:       "watch-cmd",
				StatusClearWriter: &clearBuf,
				OnProgramReady: func(_ *tea.Program) {
					select {
					case readyCh <- struct{}{}:
					default:
					}
				},
				ProgramOptions: []tea.ProgramOption{
					tea.WithInput(nil),
					tea.WithOutput(&outBuf),
					tea.WithWindowSize(80, 24),
				},
			},
			&fakeInteractiveTerminal{},
			func(waitCtx context.Context) InteractiveResult {
				select {
				case <-readyCh:
				case <-waitCtx.Done():
				}
				cancel()
				<-waitCtx.Done()
				return InteractiveResult{}
			},
		)
		if !errors.Is(err, tea.ErrProgramKilled) {
			t.Fatalf("RunInteractiveSession() err = %v, want ErrProgramKilled", err)
		}
		if clearBuf.String() != ansi.ClearProgramStatus {
			t.Fatalf("StatusClearWriter = %q, want %q", clearBuf.String(), ansi.ClearProgramStatus)
		}
	})

	t.Run("external tea.Quit without quitting flag emits clear", func(t *testing.T) {
		t.Parallel()

		var clearBuf bytes.Buffer
		var outBuf bytes.Buffer
		progCh := make(chan *tea.Program, 1)

		_, err := RunInteractiveSession(
			t.Context(),
			InteractiveOptions{
				CommandName:       "sigterm-cmd",
				StatusClearWriter: &clearBuf,
				OnProgramReady:    func(p *tea.Program) { progCh <- p },
				ProgramOptions: []tea.ProgramOption{
					tea.WithInput(nil),
					tea.WithOutput(&outBuf),
					tea.WithWindowSize(80, 24),
				},
			},
			&fakeInteractiveTerminal{},
			func(waitCtx context.Context) InteractiveResult {
				select {
				case p := <-progCh:
					p.Quit()
				case <-waitCtx.Done():
				}
				return InteractiveResult{}
			},
		)
		if err != nil {
			t.Fatalf("RunInteractiveSession() err = %v, want nil on external Quit", err)
		}
		if clearBuf.String() != ansi.ClearProgramStatus {
			t.Fatalf("StatusClearWriter on external Quit = %q, want %q", clearBuf.String(), ansi.ClearProgramStatus)
		}
	})

	t.Run("ErrInterrupted emits clear", func(t *testing.T) {
		t.Parallel()

		var clearBuf bytes.Buffer
		var outBuf bytes.Buffer
		progCh := make(chan *tea.Program, 1)

		_, err := RunInteractiveSession(
			t.Context(),
			InteractiveOptions{
				CommandName:       "int-cmd",
				StatusClearWriter: &clearBuf,
				OnProgramReady:    func(p *tea.Program) { progCh <- p },
				ProgramOptions: []tea.ProgramOption{
					tea.WithInput(nil),
					tea.WithOutput(&outBuf),
					tea.WithWindowSize(80, 24),
				},
			},
			&fakeInteractiveTerminal{},
			func(waitCtx context.Context) InteractiveResult {
				select {
				case p := <-progCh:
					p.Send(tea.InterruptMsg{})
				case <-waitCtx.Done():
				}
				return InteractiveResult{}
			},
		)
		if !errors.Is(err, tea.ErrInterrupted) {
			t.Fatalf("RunInteractiveSession() err = %v, want ErrInterrupted", err)
		}
		if clearBuf.String() != ansi.ClearProgramStatus {
			t.Fatalf("StatusClearWriter on ErrInterrupted = %q, want %q", clearBuf.String(), ansi.ClearProgramStatus)
		}
	})
}

func TestStandaloneVsModalComponentProgramStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		standalone EmbeddableComponent
		modal      EmbeddableComponent
		cancelKey  tea.KeyPressMsg
		wantState  ansi.ProgramState
		wantKind   ansi.ProgramStatusKind
	}{
		{
			name:       "confirm",
			standalone: NewConfirmModel(ConfirmOptions{Title: "Deploy?"}),
			modal:      NewConfirmModelForModal(ConfirmOptions{Title: "Deploy?"}),
			cancelKey:  tea.KeyPressMsg{Code: tea.KeyEsc},
			wantState:  ansi.ProgramStateBlocked,
			wantKind:   ansi.ProgramStatusKindPermission,
		},
		{
			name:       "input password",
			standalone: NewInputModel(InputOptions{Title: "Token", Password: true}),
			modal:      NewInputModelForModal(InputOptions{Title: "Token", Password: true}),
			cancelKey:  tea.KeyPressMsg{Code: tea.KeyEsc},
			wantState:  ansi.ProgramStateBlocked,
			wantKind:   ansi.ProgramStatusKindAuth,
		},
		{
			name:       "write",
			standalone: NewWriteModel(WriteOptions{Title: "Message"}),
			modal:      NewWriteModelForModal(WriteOptions{Title: "Message"}),
			cancelKey:  tea.KeyPressMsg{Code: tea.KeyEsc},
			wantState:  ansi.ProgramStateBlocked,
			wantKind:   ansi.ProgramStatusKindQuestion,
		},
		{
			name:       "choose",
			standalone: NewChooseModel(ChooseStringOptions{Title: "Choice", Options: []string{"a", "b"}}),
			modal:      NewChooseModelForModal(ChooseStringOptions{Title: "Choice", Options: []string{"a", "b"}}),
			cancelKey:  tea.KeyPressMsg{Code: tea.KeyEsc},
			wantState:  ansi.ProgramStateBlocked,
			wantKind:   ansi.ProgramStatusKindQuestion,
		},
		{
			name:       "filter",
			standalone: NewFilterModel(FilterOptions{Title: "Filter", Options: []string{"a", "b"}}),
			modal:      NewFilterModelForModal(FilterOptions{Title: "Filter", Options: []string{"a", "b"}}),
			cancelKey:  tea.KeyPressMsg{Code: tea.KeyEsc},
			wantState:  ansi.ProgramStateBlocked,
			wantKind:   ansi.ProgramStatusKindQuestion,
		},
		{
			name:       "file",
			standalone: NewFileModel(FileOptions{Title: "File"}),
			modal:      NewFileModelForModal(FileOptions{Title: "File"}),
			cancelKey:  tea.KeyPressMsg{Code: tea.KeyEsc},
			wantState:  ansi.ProgramStateBlocked,
			wantKind:   ansi.ProgramStatusKindQuestion,
		},
		{
			name:       "table selectable",
			standalone: NewTableModel(TableOptions{Title: "Rows", Columns: []TableColumn{{Title: "Col"}}, Rows: [][]string{{"1"}}, Selectable: true}),
			modal:      NewTableModelForModal(TableOptions{Title: "Rows", Columns: []TableColumn{{Title: "Col"}}, Rows: [][]string{{"1"}}, Selectable: true}),
			cancelKey:  tea.KeyPressMsg{Code: tea.KeyEsc},
			wantState:  ansi.ProgramStateBlocked,
			wantKind:   ansi.ProgramStatusKindQuestion,
		},
		{
			name:       "table non-selectable",
			standalone: NewTableModel(TableOptions{Title: "Rows", Columns: []TableColumn{{Title: "Col"}}, Rows: [][]string{{"1"}}, Selectable: false}),
			modal:      NewTableModelForModal(TableOptions{Title: "Rows", Columns: []TableColumn{{Title: "Col"}}, Rows: [][]string{{"1"}}, Selectable: false}),
			cancelKey:  tea.KeyPressMsg{Code: tea.KeyEsc},
			wantState:  ansi.ProgramStateIdle,
		},
		{
			name:       "pager",
			standalone: NewPagerModel(PagerOptions{Title: "Logs", Content: "line1"}),
			modal:      NewPagerModelForModal(PagerOptions{Title: "Logs", Content: "line1"}),
			cancelKey:  tea.KeyPressMsg{Code: 'q', Text: "q"},
			wantState:  ansi.ProgramStateIdle,
		},
		{
			name:       "spin",
			standalone: NewSpinModel(t.Context(), SpinCommandOptions{Title: "Working", Run: func() tea.Msg { return nil }}),
			modal:      NewSpinModelForModal(t.Context(), SpinCommandOptions{Title: "Working", Run: func() tea.Msg { return nil }}),
			cancelKey:  tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl},
			wantState:  ansi.ProgramStateWorking,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			st := tc.standalone.View().ProgramStatus
			if st == nil || st.State != tc.wantState || st.Kind != tc.wantKind {
				t.Fatalf("standalone status = %+v, want state=%q kind=%q", st, tc.wantState, tc.wantKind)
			}
			if modalSt := tc.modal.View().ProgramStatus; modalSt != nil {
				t.Fatalf("modal status = %+v, want nil", modalSt)
			}
			updated, _ := tc.standalone.Update(tc.cancelKey)
			if ec, ok := updated.(EmbeddableComponent); !ok || !ec.IsDone() || ec.View().ProgramStatus != nil {
				t.Fatalf("done status = %+v (isDone=%v), want nil and done=true", updated.View().ProgramStatus, ok && ec.IsDone())
			}
		})
	}

	// Verify list quit keybinding 'q' is disabled on choose and filter so IsDone stays accurate.
	ch := NewChooseModel(ChooseStringOptions{Title: "Choice", Options: []string{"a"}})
	_, cmd := ch.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if ch.IsDone() || cmd != nil {
		t.Fatalf("choose 'q' unexpectedly completed/quit: done=%v cmd=%v", ch.IsDone(), cmd != nil)
	}
	flt := NewFilterModel(FilterOptions{Title: "Filter", Options: []string{"a"}})
	_, fltCmd := flt.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if flt.IsDone() || fltCmd != nil {
		t.Fatalf("filter 'q' unexpectedly completed/quit: done=%v cmd=%v", flt.IsDone(), fltCmd != nil)
	}

	// Verify spinActionModel clears status when done or interrupted.
	doneCh := make(chan struct{})
	sa := spinActionModel{title: "Spinning", doneCh: doneCh}
	if sa.View().ProgramStatus == nil || sa.View().ProgramStatus.State != ansi.ProgramStateWorking {
		t.Fatalf("active spinActionModel status = %+v, want working", sa.View().ProgramStatus)
	}
	next, _ := sa.Update(spinnerDoneMsg{})
	saDone, ok := next.(spinActionModel)
	if !ok || !saDone.done || saDone.View().ProgramStatus != nil {
		t.Fatalf("done spinActionModel = (%#v, %v), want done=true and ProgramStatus=nil", next, ok)
	}
}

func TestRunComponentAndSpinnerForcedClearProgramStatus(t *testing.T) {
	t.Parallel()

	t.Run("runComponentProgramWith clears when model exits without IsDone", func(t *testing.T) {
		t.Parallel()

		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		inr, inw, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe(in) error = %v", err)
		}
		t.Cleanup(func() {
			_ = pr.Close()
			_ = pw.Close()
			_ = inr.Close()
			_ = inw.Close()
		})

		_, runErr := runComponentProgramWith(
			func(ComponentType) (componentTerminal, error) {
				return componentTerminal{in: inr, out: pw}, nil
			},
			ComponentTypeConfirm,
			quitWithoutDoneModel{done: false},
		)
		if runErr != nil {
			t.Fatalf("runComponentProgramWith() err = %v", runErr)
		}
		if closeErr := pw.Close(); closeErr != nil {
			t.Fatalf("pw.Close() error = %v", closeErr)
		}
		outBytes, readErr := io.ReadAll(pr)
		if readErr != nil {
			t.Fatalf("io.ReadAll() error = %v", readErr)
		}
		if !strings.Contains(string(outBytes), ansi.ClearProgramStatus) {
			t.Fatalf("expected ClearProgramStatus when !IsDone(), got %q", string(outBytes))
		}
	})

	t.Run("runComponentProgramWith clears on ErrInterrupted", func(t *testing.T) {
		t.Parallel()

		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		inr, inw, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe(in) error = %v", err)
		}
		t.Cleanup(func() {
			_ = pr.Close()
			_ = pw.Close()
			_ = inr.Close()
			_ = inw.Close()
		})

		_, runErr := runComponentProgramWith(
			func(ComponentType) (componentTerminal, error) {
				return componentTerminal{in: inr, out: pw}, nil
			},
			ComponentTypeConfirm,
			quitWithoutDoneModel{done: true, interrupt: true},
		)
		if !errors.Is(runErr, tea.ErrInterrupted) {
			t.Fatalf("runComponentProgramWith() err = %v, want ErrInterrupted", runErr)
		}
		if closeErr := pw.Close(); closeErr != nil {
			t.Fatalf("pw.Close() error = %v", closeErr)
		}
		outBytes, readErr := io.ReadAll(pr)
		if readErr != nil {
			t.Fatalf("io.ReadAll() error = %v", readErr)
		}
		if !strings.Contains(string(outBytes), ansi.ClearProgramStatus) {
			t.Fatalf("expected ClearProgramStatus on ErrInterrupted, got %q", string(outBytes))
		}
	})

	t.Run("runSpinnerProgram clears when interrupted", func(t *testing.T) {
		t.Parallel()

		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		inr, inw, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe(in) error = %v", err)
		}
		t.Cleanup(func() {
			_ = pr.Close()
			_ = pw.Close()
			_ = inr.Close()
			_ = inw.Close()
		})

		doneCh := make(chan struct{})
		go func() {
			_, _ = inw.Write([]byte{0x03})
		}()
		runErr := runSpinnerProgram(componentTerminal{in: inr, out: pw}, SpinOptions{Title: "Spin"}, doneCh)
		if !errors.Is(runErr, tea.ErrInterrupted) {
			t.Fatalf("runSpinnerProgram() err = %v, want ErrInterrupted", runErr)
		}
		if closeErr := pw.Close(); closeErr != nil {
			t.Fatalf("pw.Close() error = %v", closeErr)
		}
		outBytes, readErr := io.ReadAll(pr)
		if readErr != nil {
			t.Fatalf("io.ReadAll() error = %v", readErr)
		}
		if !strings.Contains(string(outBytes), ansi.ClearProgramStatus) {
			t.Fatalf("expected ClearProgramStatus on ErrInterrupted, got %q", string(outBytes))
		}
	})

	t.Run("needsSpinnerStatusClear clears on external tea.QuitMsg without done", func(t *testing.T) {
		t.Parallel()

		m := spinActionModel{title: "Spin"}
		nextModel, quitCmd := m.Update(tea.QuitMsg{})
		if quitCmd != nil {
			t.Fatalf("spinActionModel.Update(tea.QuitMsg{}) cmd = %v, want nil", quitCmd)
		}
		if !needsSpinnerStatusClear(nextModel, nil) {
			t.Fatal("needsSpinnerStatusClear(spinActionModel{done: false}, nil) = false, want true")
		}
		if needsSpinnerStatusClear(spinActionModel{done: true}, nil) {
			t.Fatal("needsSpinnerStatusClear(spinActionModel{done: true}, nil) = true, want false")
		}
		if !needsSpinnerStatusClear(spinActionModel{done: true}, errors.New("boom")) {
			t.Fatal("needsSpinnerStatusClear(done, err) = false, want true")
		}
	})
}
